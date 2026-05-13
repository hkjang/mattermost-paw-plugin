package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

type qwenPawSessionCreateRequest struct {
	ParentID string `json:"parentID,omitempty"`
	Title    string `json:"title,omitempty"`
}

type qwenPawSessionCreateResponse struct {
	ID string `json:"id"`
}

type qwenPawMessageRequest struct {
	Input     []qwenPawMessage `json:"input"`
	SessionID string           `json:"session_id,omitempty"`
	UserID    string           `json:"user_id,omitempty"`
	Channel   string           `json:"channel,omitempty"`
}

type qwenPawMessage struct {
	Role    string           `json:"role"`
	Content []qwenPawContent `json:"content"`
}

type qwenPawContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type qwenPawResponseEvent struct {
	SequenceNumber int                    `json:"sequence_number,omitempty"`
	Object         string                 `json:"object,omitempty"`
	Status         string                 `json:"status,omitempty"`
	Output         []qwenPawOutputMessage `json:"output,omitempty"`
	Error          any                    `json:"error,omitempty"`
	SessionID      string                 `json:"session_id,omitempty"`
	Usage          map[string]any         `json:"usage,omitempty"`
}

type qwenPawOutputMessage struct {
	Role    string           `json:"role"`
	Content []qwenPawContent `json:"content"`
}

type qwenPawCallError struct {
	Code       string
	Message    string
	StatusCode int
}

func (e *qwenPawCallError) Error() string {
	if e == nil {
		return ""
	}
	return e.Message
}

type qwenPawSSEEvent struct {
	Event string
	Data  []byte
}

type qwenPawStreamState struct {
	MessageID       string
	Text            string
	Thinking        string
	Tools           []string
	PartTypes       map[string]string
	ToolPartIndexes map[string]int
	ToolLabels      map[string]string
	ToolInputs      map[string]string
	ToolOutputs     map[string]string
	Done            bool
}

func (p *Plugin) createQwenPawSession(ctx context.Context, cfg *runtimeConfiguration, baseURL, title string) (string, error) {
	_ = ctx
	_ = cfg
	_ = baseURL
	title = strings.ToLower(strings.TrimSpace(title))
	title = strings.ReplaceAll(title, " ", "-")
	title = collapseHyphens(title)
	title = strings.Trim(title, "-")
	if title == "" {
		title = "mattermost"
	}
	return fmt.Sprintf("%s-%d", truncatePlain(title, 40), time.Now().UnixNano()), nil
}

func (p *Plugin) streamQwenPawMessage(ctx context.Context, cfg *runtimeConfiguration, baseURL, sessionID, userID, channelID, rootID, prompt string) (string, error) {
	updater, err := p.createPawStreamingPost(channelID, rootID)
	if err != nil {
		return "", err
	}

	idleTimeout := cfg.RequestTimeout
	if idleTimeout < 2*time.Minute {
		idleTimeout = 2 * time.Minute
	}
	streamCtx, cancel := context.WithTimeout(ctx, idleTimeout)
	defer cancel()

	request, err := p.newQwenPawChatRequest(streamCtx, cfg, baseURL, sessionID, userID, prompt)
	if err != nil {
		_ = updater.fail(userFacingQwenPawError(err))
		return "", err
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		err = classifyQwenPawRequestError(err)
		_ = updater.fail(userFacingQwenPawError(err))
		return "", err
	}
	defer response.Body.Close()
	if response.StatusCode >= http.StatusBadRequest {
		err = classifyQwenPawHTTPError(response.StatusCode)
		_ = updater.fail(userFacingQwenPawError(err))
		return "", err
	}

	state := qwenPawStreamState{}
	lastRendered := ""
	reader := bufio.NewReader(response.Body)
	for {
		event, err := readSSEEvent(reader)
		if err != nil {
			if errors.Is(err, io.EOF) {
				final := finalQwenPawMessageOrEmpty(state)
				return final, updater.completeState(state.Text, state.Thinking, state.Tools)
			}
			err = classifyQwenPawRequestError(err)
			if final := finalQwenPawMessage(state); final != "" {
				return final, updater.completeState(state.Text, state.Thinking, state.Tools)
			}
			_ = updater.fail(userFacingQwenPawError(err))
			return "", err
		}
		if event.Event == "" && len(event.Data) == 0 {
			continue
		}
		if !applyQwenPawSSEEvent(&state, sessionID, event) {
			continue
		}
		rendered := renderStreamingMessage(state.Text, state.Thinking, state.Tools, !state.Done)
		if rendered != "" && rendered != lastRendered {
			if err := updater.updateState(state.Text, state.Thinking, state.Tools, !state.Done); err != nil {
				return "", err
			}
			lastRendered = rendered
		}
		if state.Done {
			final := finalQwenPawMessageOrEmpty(state)
			return final, updater.completeState(state.Text, state.Thinking, state.Tools)
		}
	}
}

func (p *Plugin) newQwenPawChatRequest(ctx context.Context, cfg *runtimeConfiguration, baseURL, sessionID, userID, prompt string) (*http.Request, error) {
	endpoint, err := qwenPawURL(baseURL, "api", "console", "chat")
	if err != nil {
		return nil, err
	}
	body, err := json.Marshal(qwenPawMessageRequest{
		Input: []qwenPawMessage{{
			Role:    "user",
			Content: []qwenPawContent{{Type: "text", Text: prompt}},
		}},
		SessionID: sessionID,
		UserID:    userID,
		Channel:   cfg.QwenPawChannel,
	})
	if err != nil {
		return nil, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "text/event-stream")
	request.Header.Set("X-Agent-Id", cfg.QwenPawAgentID)
	if cfg.QwenPawAuthToken != "" {
		request.Header.Set("Authorization", "Bearer "+cfg.QwenPawAuthToken)
	}
	return request, nil
}

func (p *Plugin) abortQwenPawSession(ctx context.Context, cfg *runtimeConfiguration, baseURL, sessionID string) error {
	_ = ctx
	_ = cfg
	_ = baseURL
	_ = sessionID
	return nil
}

func (p *Plugin) deleteQwenPawSession(ctx context.Context, cfg *runtimeConfiguration, baseURL, sessionID string) error {
	_ = ctx
	_ = cfg
	_ = baseURL
	_ = sessionID
	return nil
}

func readSSEEvent(reader *bufio.Reader) (qwenPawSSEEvent, error) {
	var event qwenPawSSEEvent
	dataLines := []string{}
	for {
		line, err := reader.ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return event, err
		}
		line = strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r")
		if line == "" {
			event.Data = []byte(strings.Join(dataLines, "\n"))
			return event, nil
		}
		if strings.HasPrefix(line, "event:") {
			event.Event = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		} else if strings.HasPrefix(line, "data:") {
			dataLines = append(dataLines, strings.TrimSpace(strings.TrimPrefix(line, "data:")))
		}
		if errors.Is(err, io.EOF) {
			event.Data = []byte(strings.Join(dataLines, "\n"))
			return event, nil
		}
	}
}

func applyQwenPawSSEEvent(state *qwenPawStreamState, sessionID string, event qwenPawSSEEvent) bool {
	eventName := strings.TrimSpace(event.Event)
	var payload any
	if len(event.Data) > 0 {
		_ = json.Unmarshal(event.Data, &payload)
	}
	if applyQwenPawResponsePayload(state, payload) {
		return true
	}

	switch eventName {
	case "message_start":
		if data, ok := payload.(map[string]any); ok {
			state.MessageID = firstTextField(data, "id", "messageID")
		}
		return true
	case "message_delta":
		if data, ok := payload.(map[string]any); ok {
			if delta := firstRawTextField(data, "delta", "text", "content"); delta != "" {
				state.Text += delta
				return true
			}
		}
	case "message_end":
		if data, ok := payload.(map[string]any); ok {
			if content := firstTextField(data, "content", "text"); content != "" {
				state.Text = content
			}
		}
		state.Done = true
		return true
	}

	envelope := normalizeQwenPawEventPayload(payload)
	if envelope == nil {
		if eventName == "message.part.updated" || eventName == "message.part.delta" || eventName == "message.updated" || eventName == "session.idle" || eventName == "session.status_changed" || eventName == "session.error" {
			envelope = map[string]any{
				"type":       eventName,
				"properties": payload,
			}
		} else {
			return false
		}
	}
	eventType := stringValue(envelope["type"])
	props, _ := envelope["properties"].(map[string]any)
	switch eventType {
	case "message.part.updated":
		part, _ := props["part"].(map[string]any)
		if part == nil || stringValue(part["sessionID"]) != sessionID {
			return false
		}
		rememberQwenPawPartType(state, part)
		delta := rawStringValue(props["delta"])
		partID := firstTextField(part, "id", "partID")
		partType := stringValue(part["type"])
		switch partType {
		case "text":
			if delta != "" {
				state.Text += delta
			} else if text := rawStringValue(part["text"]); text != "" {
				state.Text = text
			}
			return true
		case "reasoning":
			if delta != "" {
				state.Thinking += delta
			} else if text := rawStringValue(part["text"]); text != "" {
				state.Thinking = text
			}
			return true
		case "tool":
			return rememberQwenPawToolPart(state, partID, part)
		case "file":
			return rememberQwenPawFilePart(state, partID, part)
		}
	case "message.part.delta":
		if props == nil || stringValue(props["sessionID"]) != sessionID {
			return false
		}
		field := stringValue(props["field"])
		delta := rawStringValue(props["delta"])
		if delta == "" {
			return false
		}
		partID := stringValue(props["partID"])
		partType := qwenPawPartType(state, partID)
		if partType == "tool" || partType == "file" {
			if isRenderableToolDeltaField(field) {
				return appendQwenPawToolOutput(state, partID, delta)
			}
			return false
		}
		if field != "" && field != "text" && field != "content" {
			return false
		}
		if partType == "reasoning" {
			state.Thinking += delta
			return true
		}
		state.Text += delta
		return true
	case "message.updated":
		info, _ := props["info"].(map[string]any)
		if info == nil || stringValue(info["sessionID"]) != sessionID {
			return false
		}
		if text := extractQwenPawTextFromEventProperties(props); text != "" {
			state.Text = text
			return true
		}
		return false
	case "session.idle":
		if props != nil && stringValue(props["sessionID"]) == sessionID {
			state.Done = true
			return true
		}
	case "session.status_changed":
		if props != nil && stringValue(props["sessionID"]) == sessionID {
			if strings.EqualFold(stringValue(props["status"]), "idle") {
				state.Done = true
				return true
			}
		}
	case "session.error":
		if props == nil || stringValue(props["sessionID"]) == sessionID {
			errMsg := stringValue(props["error"])
			if errMsg == "" {
				errMsg = stringValue(props["message"])
			}
			if errMsg == "" {
				if errData, ok := props["data"].(map[string]any); ok {
					errMsg = stringValue(errData["message"])
					if errMsg == "" {
						errMsg = stringValue(errData["error"])
					}
				}
			}
			if errMsg != "" {
				state.Text = fmt.Sprintf("QwenPaw server error. %s", errMsg)
			} else {
				state.Text = "QwenPaw server error."
			}
			state.Text += "\n\n" + sessionResetHint
			state.Done = true
			return true
		}
	}
	return false
}

func applyQwenPawResponsePayload(state *qwenPawStreamState, payload any) bool {
	typed, ok := payload.(map[string]any)
	if !ok {
		return false
	}
	status := strings.ToLower(stringValue(typed["status"]))
	if errorText := extractQwenPawErrorText(typed["error"]); errorText != "" {
		state.Text = "QwenPaw error: " + errorText
		state.Done = true
		return true
	}
	if outputText := extractQwenPawOutputText(typed["output"]); outputText != "" {
		state.Text = outputText
	}
	switch status {
	case "completed", "failed", "cancelled", "canceled":
		state.Done = true
	case "created", "in_progress":
	}
	return status != "" || state.Text != ""
}

func extractQwenPawOutputText(value any) string {
	parts := []string{}
	items, ok := value.([]any)
	if !ok {
		return ""
	}
	for _, item := range items {
		message, ok := item.(map[string]any)
		if !ok || strings.ToLower(stringValue(message["role"])) != "assistant" {
			continue
		}
		contentItems, _ := message["content"].([]any)
		for _, contentItem := range contentItems {
			content, ok := contentItem.(map[string]any)
			if !ok || strings.ToLower(stringValue(content["type"])) != "text" {
				continue
			}
			if text := rawStringValue(content["text"]); strings.TrimSpace(text) != "" {
				parts = append(parts, text)
			}
		}
	}
	return strings.TrimSpace(strings.Join(parts, "\n\n"))
}

func extractQwenPawErrorText(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(typed)
	case map[string]any:
		if message := firstTextField(typed, "message", "detail", "error", "code"); message != "" {
			return message
		}
		body, _ := json.Marshal(typed)
		return strings.TrimSpace(string(body))
	default:
		return strings.TrimSpace(fmt.Sprint(typed))
	}
}

func rememberQwenPawPartType(state *qwenPawStreamState, part map[string]any) {
	partID := firstTextField(part, "id", "partID")
	partType := stringValue(part["type"])
	if partID == "" || partType == "" {
		return
	}
	if state.PartTypes == nil {
		state.PartTypes = map[string]string{}
	}
	state.PartTypes[partID] = partType
}

func qwenPawPartType(state *qwenPawStreamState, partID string) string {
	if state.PartTypes == nil || partID == "" {
		return ""
	}
	return state.PartTypes[partID]
}

func extractQwenPawTextFromEventProperties(props map[string]any) string {
	for _, key := range []string{"parts", "message", "data"} {
		if value, ok := props[key]; ok {
			if text := extractQwenPawAnswerText(value); text != "" {
				return text
			}
		}
	}
	return ""
}

func normalizeQwenPawEventPayload(payload any) map[string]any {
	typed, ok := payload.(map[string]any)
	if !ok {
		return nil
	}
	if payloadValue, ok := typed["payload"].(map[string]any); ok {
		return payloadValue
	}
	if typed["type"] != nil {
		return typed
	}
	return nil
}

var ansiEscapePattern = regexp.MustCompile(`\x1b\[[0-?]*[ -/]*[@-~]`)

func rememberQwenPawToolPart(state *qwenPawStreamState, partID string, part map[string]any) bool {
	label := renderToolLabel(part)
	input := extractToolInput(part)
	output := extractToolOutput(part)
	if partID != "" {
		if state.ToolLabels == nil {
			state.ToolLabels = map[string]string{}
		}
		if label != "" {
			state.ToolLabels[partID] = label
		}
		if input != "" {
			if state.ToolInputs == nil {
				state.ToolInputs = map[string]string{}
			}
			state.ToolInputs[partID] = input
		} else if state.ToolInputs != nil {
			input = state.ToolInputs[partID]
		}
		if output != "" {
			if state.ToolOutputs == nil {
				state.ToolOutputs = map[string]string{}
			}
			state.ToolOutputs[partID] = output
		} else if state.ToolOutputs != nil {
			output = state.ToolOutputs[partID]
		}
		label = state.ToolLabels[partID]
	}
	return rememberQwenPawRenderedToolBlock(state, partID, renderStoredToolBlock(label, input, output))
}

func rememberQwenPawFilePart(state *qwenPawStreamState, partID string, part map[string]any) bool {
	label := renderFileLabel(part)
	content := extractFileContent(part)
	if partID != "" {
		if state.ToolLabels == nil {
			state.ToolLabels = map[string]string{}
		}
		state.ToolLabels[partID] = label
		if content != "" {
			if state.ToolOutputs == nil {
				state.ToolOutputs = map[string]string{}
			}
			state.ToolOutputs[partID] = content
		} else if state.ToolOutputs != nil {
			content = state.ToolOutputs[partID]
		}
	}
	return rememberQwenPawRenderedToolBlock(state, partID, renderStoredToolBlock(label, "", content))
}

func appendQwenPawToolOutput(state *qwenPawStreamState, partID, delta string) bool {
	delta = cleanTerminalText(delta)
	if partID == "" || delta == "" {
		return false
	}
	if state.ToolOutputs == nil {
		state.ToolOutputs = map[string]string{}
	}
	state.ToolOutputs[partID] += delta
	label := "tool"
	if state.ToolLabels != nil && state.ToolLabels[partID] != "" {
		label = state.ToolLabels[partID]
	}
	input := ""
	if state.ToolInputs != nil {
		input = state.ToolInputs[partID]
	}
	return rememberQwenPawRenderedToolBlock(state, partID, renderStoredToolBlock(label, input, state.ToolOutputs[partID]))
}

func rememberQwenPawRenderedToolBlock(state *qwenPawStreamState, partID, block string) bool {
	block = strings.TrimSpace(block)
	if block == "" {
		return false
	}
	if partID != "" {
		if state.ToolPartIndexes == nil {
			state.ToolPartIndexes = map[string]int{}
		}
		if index, ok := state.ToolPartIndexes[partID]; ok && index >= 0 && index < len(state.Tools) {
			if state.Tools[index] == block {
				return false
			}
			state.Tools[index] = block
			return true
		}
		state.ToolPartIndexes[partID] = len(state.Tools)
		state.Tools = append(state.Tools, block)
		return true
	}
	if !containsString(state.Tools, block) {
		state.Tools = append(state.Tools, block)
		return true
	}
	return false
}

func isRenderableToolDeltaField(field string) bool {
	switch strings.ToLower(strings.TrimSpace(field)) {
	case "", "text", "content", "output", "stdout", "stderr", "result", "data", "source", "code", "message", "error":
		return true
	default:
		return false
	}
}

func renderToolPart(part map[string]any) string {
	return renderStoredToolBlock(renderToolLabel(part), extractToolInput(part), extractToolOutput(part))
}

func renderToolLabel(part map[string]any) string {
	tool := firstTextField(part, "tool", "name", "title")
	state, _ := part["state"].(map[string]any)
	if state == nil {
		if tool == "" {
			return "tool"
		}
		return tool
	}
	title := firstTextField(state, "title")
	status := firstTextField(state, "status")
	label := strings.TrimSpace(strings.Join([]string{tool, title}, " "))
	if label == "" {
		label = "tool"
	}
	if status != "" {
		label += " (" + status + ")"
	}
	return label
}

func renderStoredToolBlock(label, input, output string) string {
	if strings.HasPrefix(strings.TrimSpace(label), "File") {
		return renderFileBlock(label, output)
	}
	return renderToolBlock(label, input, output)
}

func renderToolBlock(label, input, output string) string {
	label = strings.TrimSpace(label)
	if label == "" {
		label = "tool"
	}
	blocks := []string{fmt.Sprintf("> Tool call: `%s`", inlineCode(label))}
	if input = sanitizeTerminalText(input); input != "" {
		blocks = append(blocks, "Input:\n\n"+fencedCodeBlock(codeLanguageForTool(label, input), input))
	}
	if output = sanitizeTerminalText(output); output != "" {
		blocks = append(blocks, "Output:\n\n"+fencedCodeBlock(codeLanguageForTool(label, output), output))
	}
	return strings.TrimSpace(strings.Join(blocks, "\n\n"))
}

func renderFilePart(part map[string]any) string {
	return renderFileBlock(renderFileLabel(part), extractFileContent(part))
}

func renderFileLabel(part map[string]any) string {
	filename := firstTextField(part, "filename", "path", "url", "name", "title")
	if filename == "" {
		return "File"
	}
	return "File: " + filename
}

func renderFileBlock(label, content string) string {
	label = strings.TrimSpace(label)
	if label == "" {
		label = "File"
	}
	blocks := []string{fmt.Sprintf("> %s", inlineCode(label))}
	if content = sanitizeTerminalText(content); content != "" {
		blocks = append(blocks, "Source:\n\n"+fencedCodeBlock(codeLanguageForFilename(label, content), content))
	}
	return strings.TrimSpace(strings.Join(blocks, "\n\n"))
}

func extractToolInput(part map[string]any) string {
	state, _ := part["state"].(map[string]any)
	for _, source := range []map[string]any{state, part} {
		if source == nil {
			continue
		}
		if command := firstStructuredTextField(source, "command", "script"); command != "" {
			return command
		}
		if input := firstStructuredTextField(source, "input", "args", "arguments", "parameters"); input != "" {
			return input
		}
	}
	return ""
}

func extractToolOutput(part map[string]any) string {
	state, _ := part["state"].(map[string]any)
	for _, source := range []map[string]any{state, part} {
		if source == nil {
			continue
		}
		outputs := []string{}
		if stdout := firstStructuredTextField(source, "stdout"); stdout != "" {
			outputs = append(outputs, "stdout:\n"+stdout)
		}
		if stderr := firstStructuredTextField(source, "stderr"); stderr != "" {
			outputs = append(outputs, "stderr:\n"+stderr)
		}
		if len(outputs) > 0 {
			return strings.Join(outputs, "\n\n")
		}
		if output := firstStructuredTextField(source, "output", "result", "results", "response", "error", "message", "content", "data"); output != "" {
			return output
		}
	}
	return ""
}

func extractFileContent(part map[string]any) string {
	return firstStructuredTextField(part, "content", "text", "source", "code", "data")
}

func firstStructuredTextField(values map[string]any, keys ...string) string {
	for _, key := range keys {
		if text := structuredTextValue(values[key]); text != "" {
			return text
		}
	}
	return ""
}

func structuredTextValue(value any) string {
	switch typed := value.(type) {
	case string:
		return sanitizeTerminalText(typed)
	case nil:
		return ""
	case map[string]any, []any:
		body, err := json.MarshalIndent(typed, "", "  ")
		if err != nil {
			return ""
		}
		return sanitizeTerminalText(string(body))
	default:
		return sanitizeTerminalText(fmt.Sprint(typed))
	}
}

func sanitizeTerminalText(value string) string {
	return strings.TrimSpace(cleanTerminalText(value))
}

func cleanTerminalText(value string) string {
	value = strings.ReplaceAll(value, "\r\n", "\n")
	value = strings.ReplaceAll(value, "\r", "\n")
	return ansiEscapePattern.ReplaceAllString(value, "")
}

func fencedCodeBlock(language, text string) string {
	text = sanitizeTerminalText(text)
	if text == "" {
		return ""
	}
	fence := markdownFence(text)
	language = strings.TrimSpace(language)
	if language != "" {
		return fmt.Sprintf("%s%s\n%s\n%s", fence, language, text, fence)
	}
	return fmt.Sprintf("%s\n%s\n%s", fence, text, fence)
}

func markdownFence(text string) string {
	maxRun := 2
	run := 0
	for _, r := range text {
		if r == '`' {
			run++
			if run > maxRun {
				maxRun = run
			}
			continue
		}
		run = 0
	}
	return strings.Repeat("`", maxRun+1)
}

func codeLanguageForTool(label, content string) string {
	lowerLabel := strings.ToLower(label)
	switch {
	case strings.Contains(lowerLabel, "powershell") || strings.Contains(lowerLabel, "pwsh"):
		return "powershell"
	case strings.Contains(lowerLabel, "bash") || strings.Contains(lowerLabel, "shell") || strings.Contains(lowerLabel, "terminal") || strings.Contains(lowerLabel, "command"):
		return "console"
	case json.Valid([]byte(strings.TrimSpace(content))):
		return "json"
	default:
		return "text"
	}
}

func codeLanguageForFilename(label, content string) string {
	lower := strings.ToLower(label)
	switch {
	case strings.HasSuffix(lower, ".go"):
		return "go"
	case strings.HasSuffix(lower, ".ts"):
		return "typescript"
	case strings.HasSuffix(lower, ".tsx"):
		return "tsx"
	case strings.HasSuffix(lower, ".js"):
		return "javascript"
	case strings.HasSuffix(lower, ".jsx"):
		return "jsx"
	case strings.HasSuffix(lower, ".json") || json.Valid([]byte(strings.TrimSpace(content))):
		return "json"
	case strings.HasSuffix(lower, ".yaml") || strings.HasSuffix(lower, ".yml"):
		return "yaml"
	case strings.HasSuffix(lower, ".md"):
		return "markdown"
	case strings.HasSuffix(lower, ".py"):
		return "python"
	case strings.HasSuffix(lower, ".sh"):
		return "bash"
	case strings.HasSuffix(lower, ".ps1"):
		return "powershell"
	case strings.HasSuffix(lower, ".sql"):
		return "sql"
	case strings.HasSuffix(lower, ".html"):
		return "html"
	case strings.HasSuffix(lower, ".css"):
		return "css"
	case strings.HasSuffix(lower, ".scss"):
		return "scss"
	default:
		return "text"
	}
}

func inlineCode(value string) string {
	return strings.ReplaceAll(strings.TrimSpace(value), "`", "'")
}

func renderStreamingMessage(text, thinking string, tools []string, streaming bool) string {
	visibleText, completedThinking, activeThinking := splitThinkBlocks(text)
	if activeThinking != "" {
		thinking = strings.TrimSpace(thinking + "\n" + activeThinking)
	}
	text = strings.TrimSpace(visibleText)
	blocks := make([]string, 0, len(completedThinking)+1)
	for _, item := range completedThinking {
		item = strings.TrimSpace(removeThinkTags(item))
		if item != "" {
			blocks = append(blocks, fmt.Sprintf("<div class=\"paw-thinking paw-thinking-complete\">%s</div>", item))
		}
	}
	thinking = strings.TrimSpace(removeThinkTags(thinking))
	if thinking != "" {
		className := "paw-thinking"
		if !streaming {
			className += " paw-thinking-complete"
		}
		blocks = append(blocks, fmt.Sprintf("<div class=\"%s\">%s</div>", className, thinking))
	}
	for _, tool := range tools {
		if strings.TrimSpace(tool) != "" {
			blocks = append(blocks, tool)
		}
	}
	blocks = append(blocks, text)
	return strings.TrimSpace(strings.Join(blocks, "\n\n"))
}

func finalQwenPawMessage(state qwenPawStreamState) string {
	return strings.TrimSpace(renderStreamingMessage(state.Text, state.Thinking, state.Tools, false))
}

func finalQwenPawMessageOrEmpty(state qwenPawStreamState) string {
	final := finalQwenPawMessage(state)
	if final == "" {
		return "The response was empty."
	}
	return final
}

func splitThinkBlocks(text string) (string, []string, string) {
	remaining := text
	visible := strings.Builder{}
	completed := []string{}
	active := ""
	for {
		lower := strings.ToLower(remaining)
		start := strings.Index(lower, "<think>")
		endBeforeStart := strings.Index(lower, "</think>")
		if start < 0 && endBeforeStart < 0 {
			visible.WriteString(removeThinkTags(remaining))
			break
		}
		if endBeforeStart >= 0 && (start < 0 || endBeforeStart < start) {
			if before := strings.TrimSpace(remaining[:endBeforeStart]); before != "" {
				completed = append(completed, before)
			}
			remaining = remaining[endBeforeStart+len("</think>"):]
			continue
		}

		visible.WriteString(remaining[:start])
		afterStart := remaining[start+len("<think>"):]
		end := strings.Index(strings.ToLower(afterStart), "</think>")
		if end < 0 {
			active = strings.TrimSpace(afterStart)
			break
		}
		if block := strings.TrimSpace(afterStart[:end]); block != "" {
			completed = append(completed, block)
		}
		remaining = afterStart[end+len("</think>"):]
	}
	return strings.TrimSpace(visible.String()), completed, active
}

func removeThinkBlocks(text string) string {
	visible, _, _ := splitThinkBlocks(text)
	return visible
}

func removeThinkTags(text string) string {
	replacer := strings.NewReplacer("<think>", "", "</think>", "", "<THINK>", "", "</THINK>", "")
	return strings.TrimSpace(replacer.Replace(text))
}

func (p *Plugin) doQwenPawJSON(ctx context.Context, method, endpoint string, body []byte, timeoutDuration time.Duration) ([]byte, int, error) {
	requestCtx, cancel := context.WithTimeout(ctx, timeoutDuration)
	defer cancel()
	request, err := http.NewRequestWithContext(requestCtx, method, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, 0, err
	}
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	request.Header.Set("Accept", "application/json")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return nil, 0, classifyQwenPawRequestError(err)
	}
	defer response.Body.Close()
	responseBody, readErr := io.ReadAll(io.LimitReader(response.Body, 1024*1024))
	if readErr != nil {
		return nil, response.StatusCode, &qwenPawCallError{Code: "parse_failed", Message: "Could not parse the QwenPaw response.", StatusCode: response.StatusCode}
	}
	return responseBody, response.StatusCode, nil
}

func renderQwenPawResponse(body []byte) (string, error) {
	var payload any
	if err := json.Unmarshal(body, &payload); err != nil {
		return strings.TrimSpace(string(body)), nil
	}
	if message := extractQwenPawErrorMessage(payload); message != "" {
		return "Error: " + message, nil
	}
	text := extractQwenPawText(payload)
	if text != "" {
		return text, nil
	}
	pretty, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(pretty)), nil
}

func extractQwenPawErrorMessage(value any) string {
	typed, ok := value.(map[string]any)
	if !ok {
		return ""
	}
	info, ok := typed["info"].(map[string]any)
	if !ok {
		return ""
	}
	errValue, ok := info["error"].(map[string]any)
	if !ok {
		return ""
	}
	if data, ok := errValue["data"].(map[string]any); ok {
		if message := stringValue(data["message"]); message != "" {
			return message
		}
	}
	return stringValue(errValue["name"])
}

func extractQwenPawText(value any) string {
	parts := make([]string, 0)
	if typed, ok := value.(map[string]any); ok {
		if responseParts, ok := typed["parts"]; ok {
			collectQwenPawParts(responseParts, &parts)
			return strings.TrimSpace(strings.Join(parts, "\n\n"))
		}
	}
	collectQwenPawParts(value, &parts)
	return strings.TrimSpace(strings.Join(parts, "\n\n"))
}

func extractQwenPawAnswerText(value any) string {
	parts := make([]string, 0)
	collectQwenPawAnswerText(value, &parts)
	return strings.TrimSpace(strings.Join(parts, "\n\n"))
}

func collectQwenPawAnswerText(value any, parts *[]string) {
	switch typed := value.(type) {
	case map[string]any:
		partType := strings.ToLower(stringValue(typed["type"]))
		if partType == "text" {
			if text := firstTextField(typed, "text", "content", "message"); text != "" {
				*parts = append(*parts, text)
				return
			}
		}
		if partType == "reasoning" || partType == "tool" || partType == "file" {
			return
		}
		for key, nested := range typed {
			lowerKey := strings.ToLower(key)
			if lowerKey == "parts" || lowerKey == "messages" || lowerKey == "message" || lowerKey == "content" || lowerKey == "data" {
				collectQwenPawAnswerText(nested, parts)
			}
		}
	case []any:
		for _, item := range typed {
			collectQwenPawAnswerText(item, parts)
		}
	}
}

func collectQwenPawParts(value any, parts *[]string) {
	switch typed := value.(type) {
	case map[string]any:
		partType := strings.ToLower(stringValue(typed["type"]))
		switch partType {
		case "text", "reasoning":
			if text := firstTextField(typed, "text", "content", "message"); text != "" {
				*parts = append(*parts, text)
				return
			}
		case "tool":
			if label := renderToolPart(typed); label != "" {
				*parts = append(*parts, label)
				return
			}
		case "file":
			if file := renderFilePart(typed); file != "" {
				*parts = append(*parts, file)
				return
			}
		}
		for key, nested := range typed {
			lowerKey := strings.ToLower(key)
			if lowerKey == "parts" || lowerKey == "messages" || lowerKey == "message" || lowerKey == "content" || lowerKey == "data" {
				collectQwenPawParts(nested, parts)
			}
		}
	case []any:
		for _, item := range typed {
			collectQwenPawParts(item, parts)
		}
	case string:
		if strings.TrimSpace(typed) != "" {
			*parts = append(*parts, strings.TrimSpace(typed))
		}
	}
}

func firstTextField(values map[string]any, keys ...string) string {
	for _, key := range keys {
		if text := stringValue(values[key]); text != "" {
			return text
		}
	}
	return ""
}

func firstRawTextField(values map[string]any, keys ...string) string {
	for _, key := range keys {
		if text := rawStringValue(values[key]); text != "" {
			return text
		}
	}
	return ""
}

func containsString(values []string, target string) bool {
	target = strings.TrimSpace(target)
	for _, value := range values {
		if strings.TrimSpace(value) == target {
			return true
		}
	}
	return false
}

func classifyQwenPawHTTPError(statusCode int) error {
	switch {
	case statusCode == http.StatusNotFound:
		return &qwenPawCallError{Code: "not_found", Message: "QwenPaw API endpoint or agent was not found.", StatusCode: statusCode}
	case statusCode == http.StatusRequestTimeout || statusCode == http.StatusGatewayTimeout:
		return &qwenPawCallError{Code: "timeout", Message: "QwenPaw response timed out.", StatusCode: statusCode}
	case statusCode >= 400 && statusCode < 500:
		return &qwenPawCallError{Code: "bad_request", Message: "QwenPaw could not process the request.", StatusCode: statusCode}
	case statusCode >= 500:
		return &qwenPawCallError{Code: "server_error", Message: "QwenPaw server error.", StatusCode: statusCode}
	default:
		return &qwenPawCallError{Code: "unexpected", Message: "Could not parse the QwenPaw response.", StatusCode: statusCode}
	}
}

func classifyQwenPawRequestError(err error) error {
	var timeout interface{ Timeout() bool }
	if errors.As(err, &timeout) && timeout.Timeout() {
		return &qwenPawCallError{Code: "timeout", Message: "QwenPaw response timed out."}
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return &qwenPawCallError{Code: "timeout", Message: "QwenPaw response timed out."}
	}
	var dnsErr *net.DNSError
	var hostErr x509.HostnameError
	var caErr x509.UnknownAuthorityError
	if errors.As(err, &dnsErr) || errors.As(err, &hostErr) || errors.As(err, &caErr) {
		return &qwenPawCallError{Code: "connect_failed", Message: "Could not connect to the QwenPaw server."}
	}
	return &qwenPawCallError{Code: "connect_failed", Message: "Could not connect to the QwenPaw server."}
}

func userFacingQwenPawError(err error) string {
	var callErr *qwenPawCallError
	if errors.As(err, &callErr) {
		return callErr.Message
	}
	return "Could not connect to the QwenPaw server."
}

func stringValue(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case nil:
		return ""
	default:
		return strings.TrimSpace(fmt.Sprint(typed))
	}
}

func rawStringValue(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case nil:
		return ""
	default:
		return fmt.Sprint(typed)
	}
}

func qwenPawURL(base string, segments ...string) (string, error) {
	parsed, err := url.Parse(strings.TrimRight(base, "/"))
	if err != nil {
		return "", err
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("qwenpaw base URL must include scheme and host")
	}
	parsed.Path = joinURLPath(parsed.Path, segments...)
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String(), nil
}

func joinURLPath(base string, segments ...string) string {
	parts := splitPathSegments(base)
	for _, segment := range segments {
		segment = strings.Trim(segment, "/")
		if segment != "" {
			parts = append(parts, url.PathEscape(segment))
		}
	}
	if len(parts) == 0 {
		return "/"
	}
	return "/" + strings.Join(parts, "/")
}
