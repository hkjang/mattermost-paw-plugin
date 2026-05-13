package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"sync"

	"github.com/gorilla/mux"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"
	"github.com/mattermost/mattermost/server/public/pluginapi"
)

type Plugin struct {
	plugin.MattermostPlugin

	client *pluginapi.Client
	router *mux.Router

	configurationLock sync.RWMutex
	configuration     *configuration

	botLock sync.RWMutex
	bot     botAccount
}

type botAccount struct {
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	UserID      string `json:"user_id"`
	Active      bool   `json:"active"`
	LastError   string `json:"last_error,omitempty"`
	UpdatedAt   int64  `json:"updated_at"`
}

type incomingMessage struct {
	Channel *model.Channel
	User    *model.User
	Prompt  string
	RootID  string
	IsDM    bool
}

func (p *Plugin) OnActivate() error {
	p.client = pluginapi.NewClient(p.API, p.Driver)
	if err := p.OnConfigurationChange(); err != nil {
		return err
	}
	p.router = p.initRouter()
	if err := p.ensureBot(); err != nil {
		p.API.LogError("Failed to ensure paw bot during activation", "error", err)
	}
	return nil
}

func (p *Plugin) OnDeactivate() error {
	return nil
}

func (p *Plugin) MessageHasBeenPosted(_ *plugin.Context, post *model.Post) {
	if post == nil || post.UserId == "" || p.isBotUserID(post.UserId) {
		return
	}
	if post.GetProp("from_bot") != nil || post.GetProp("from_plugin") != nil || post.GetProp("from_webhook") != nil {
		return
	}
	if post.RemoteId != nil && *post.RemoteId != "" {
		return
	}

	go func() {
		if err := p.handlePostedMessage(post); err != nil {
			p.API.LogError("Failed to process paw post trigger", "error", err, "post_id", post.Id)
		}
	}()
}

func (p *Plugin) handlePostedMessage(post *model.Post) error {
	cfg, err := p.getRuntimeConfiguration()
	if err != nil {
		return err
	}
	account, err := p.getOrEnsureBot()
	if err != nil {
		return err
	}

	channel, appErr := p.API.GetChannel(post.ChannelId)
	if appErr != nil {
		return fmt.Errorf("failed to get channel: %w", appErr)
	}

	if channel.Type == model.ChannelTypeDirect {
		if account.UserID == "" || !strings.Contains(channel.Name, account.UserID) {
			return nil
		}
	}

	user, appErr := p.API.GetUser(post.UserId)
	if appErr != nil {
		return fmt.Errorf("failed to get user: %w", appErr)
	}
	if user.IsBot {
		return nil
	}

	prompt, triggered := extractPrompt(cfg.BotUsername, account.UserID, channel, post.Message)
	if !triggered {
		return nil
	}
	message := incomingMessage{
		Channel: channel,
		User:    user,
		Prompt:  strings.TrimSpace(prompt),
		RootID:  responseRootID(post, channel),
		IsDM:    channel.Type == model.ChannelTypeDirect,
	}
	if len(post.FileIds) > 0 {
		return p.postText(channel.Id, message.RootID, "File attachments are not supported yet.")
	}
	if message.Prompt == "" {
		return p.postText(channel.Id, message.RootID, buildUsageMessage(cfg.BotUsername))
	}
	return p.processPawMessage(context.Background(), cfg, message)
}

func extractPrompt(botUsername, botUserID string, channel *model.Channel, message string) (string, bool) {
	message = strings.TrimSpace(message)
	if channel == nil || message == "" {
		return "", false
	}

	mentionPatterns := []string{"@" + strings.ToLower(botUsername)}
	if botUserID != "" {
		mentionPatterns = append(mentionPatterns, "@"+strings.ToLower(botUserID))
	}

	lowerMessage := strings.ToLower(message)
	mentionFound := false
	for _, mention := range mentionPatterns {
		if strings.Contains(lowerMessage, mention) {
			mentionFound = true
			break
		}
	}

	switch channel.Type {
	case model.ChannelTypeDirect:
		return stripMentions(message, botUsername, botUserID), true
	case model.ChannelTypeGroup:
		if !mentionFound {
			return "", false
		}
		return stripMentions(message, botUsername, botUserID), true
	default:
		if !mentionFound {
			return "", false
		}
		return stripMentions(message, botUsername, botUserID), true
	}
}

func stripMentions(message, botUsername, botUserID string) string {
	tokens := []string{regexp.QuoteMeta(botUsername)}
	if botUserID != "" {
		tokens = append(tokens, regexp.QuoteMeta(botUserID))
	}
	pattern := regexp.MustCompile(`(?i)@(` + strings.Join(tokens, "|") + `)\b`)
	return strings.TrimSpace(pattern.ReplaceAllString(message, ""))
}

func responseRootID(post *model.Post, channel *model.Channel) string {
	if post == nil || channel == nil {
		return ""
	}
	if post.RootId != "" {
		return post.RootId
	}
	return post.Id
}

func (p *Plugin) processPawMessage(ctx context.Context, cfg *runtimeConfiguration, msg incomingMessage) error {
	domainUserID, err := resolveMappedUserID(msg.User, cfg)
	if err != nil {
		return p.postText(msg.Channel.Id, msg.RootID, "Could not map your Mattermost user ID to a QwenPaw domain. Please contact an administrator.")
	}
	hubUserID := strings.ToLower(strings.TrimSpace(msg.User.Username))

	switch classifyUserCommand(msg.Prompt) {
	case "start":
		if !cfg.AllowUserStartServer {
			return p.postText(msg.Channel.Id, msg.RootID, "Server start is disabled by the administrator.")
		}
		return p.startUserServer(ctx, cfg, msg.Channel.Id, msg.RootID, hubUserID)
	case "stop":
		if !cfg.AllowUserStopServer {
			return p.postText(msg.Channel.Id, msg.RootID, "Server stop is disabled by the administrator.")
		}
		return p.stopUserServer(ctx, cfg, msg.Channel.Id, msg.RootID, hubUserID)
	case "status":
		status, err := p.getUserServerStatus(ctx, cfg, hubUserID)
		if err != nil {
			return p.postText(msg.Channel.Id, msg.RootID, "Could not check your personal QwenPaw server status.")
		}
		if status.Ready {
			return p.postText(msg.Channel.Id, msg.RootID, "Your personal QwenPaw server is running.")
		}
		return p.postText(msg.Channel.Id, msg.RootID, "Your personal QwenPaw server is stopped.")
	case "session_reset":
		return p.handleSessionReset(ctx, cfg, msg, domainUserID)
	case "session_abort":
		return p.handleSessionAbort(ctx, cfg, msg, domainUserID)
	case "session_info":
		return p.handleSessionInfo(msg, domainUserID)
	}

	status, err := p.getUserServerStatus(ctx, cfg, hubUserID)
	if err != nil {
		return p.postText(msg.Channel.Id, msg.RootID, "Could not check your personal QwenPaw server status.")
	}
	if !status.Ready {
		if !cfg.AutoStartServer {
			return p.postText(msg.Channel.Id, msg.RootID, "Start your server first by sending `start` or `/start`.")
		}
		if err := p.startUserServerAndWait(ctx, cfg, hubUserID); err != nil {
			return p.postText(msg.Channel.Id, msg.RootID, "Could not start your personal QwenPaw server.")
		}
	}

	baseURL := buildUserQwenPawURL(domainUserID, cfg.BaseDomainSuffix)
	sessionID, err := p.getOrCreateSessionID(ctx, cfg, baseURL, msg, domainUserID)
	if err != nil {
		return p.postText(msg.Channel.Id, msg.RootID, userFacingQwenPawError(err))
	}
	output, err := p.streamQwenPawMessage(ctx, cfg, baseURL, sessionID, domainUserID, msg.Channel.Id, msg.RootID, msg.Prompt)
	if err != nil {
		var callErr *qwenPawCallError
		if errors.As(err, &callErr) && callErr.StatusCode == http.StatusNotFound {
			if newSessionID, resetErr := p.resetSessionID(ctx, cfg, baseURL, msg, domainUserID); resetErr == nil {
				output, err = p.streamQwenPawMessage(ctx, cfg, baseURL, newSessionID, domainUserID, msg.Channel.Id, msg.RootID, msg.Prompt)
			}
		}
	}
	if err != nil {
		return p.postText(msg.Channel.Id, msg.RootID, userFacingQwenPawError(err))
	}
	if strings.TrimSpace(output) == "" {
		output = "The response was empty."
	}
	return nil
}

func classifyUserCommand(prompt string) string {
	normalized := strings.ToLower(strings.TrimSpace(prompt))
	normalized = strings.Join(strings.Fields(normalized), " ")
	switch normalized {
	case "start", "/start", "server start":
		return "start"
	case "stop", "/stop", "server stop":
		return "stop"
	case "status", "/status", "server status":
		return "status"
	case "reset", "/reset", "new session", "/new":
		return "session_reset"
	case "abort", "/abort", "cancel", "/cancel":
		return "session_abort"
	case "session", "/session", "session info":
		return "session_info"
	default:
		return ""
	}
}

func buildUsageMessage(botUsername string) string {
	return strings.Join([]string{
		fmt.Sprintf("Send a question after `@%s`, or send a direct message to the bot.", botUsername),
		"",
		"Server commands: `start`, `stop`, `status`.",
		"Session commands: `session`, `reset`, `abort`.",
	}, "\n")
}

const sessionResetHint = "The current session may be stale. Send `reset` to start a new session."

func (p *Plugin) handleSessionReset(ctx context.Context, cfg *runtimeConfiguration, msg incomingMessage, domainUserID string) error {
	baseURL := buildUserQwenPawURL(domainUserID, cfg.BaseDomainSuffix)
	previous, _ := p.getStoredSessionID(msg, domainUserID)
	newSessionID, err := p.resetSessionID(ctx, cfg, baseURL, msg, domainUserID)
	if err != nil {
		return p.postText(msg.Channel.Id, msg.RootID, "Could not reset the session: "+userFacingQwenPawError(err))
	}
	body := fmt.Sprintf("Started a new session.\n- New session ID: `%s`", newSessionID)
	if previous != "" && previous != newSessionID {
		body += fmt.Sprintf("\n- Previous session ID: `%s`", previous)
	}
	body += "\n\nSend your next question when ready."
	return p.postText(msg.Channel.Id, msg.RootID, body)
}

func (p *Plugin) handleSessionAbort(ctx context.Context, cfg *runtimeConfiguration, msg incomingMessage, domainUserID string) error {
	sessionID, err := p.getStoredSessionID(msg, domainUserID)
	if err != nil {
		return p.postText(msg.Channel.Id, msg.RootID, "Could not load the session.")
	}
	if sessionID == "" {
		return p.postText(msg.Channel.Id, msg.RootID, "There is no active session in this scope.")
	}
	baseURL := buildUserQwenPawURL(domainUserID, cfg.BaseDomainSuffix)
	if err := p.abortQwenPawSession(ctx, cfg, baseURL, sessionID); err != nil {
		return p.postText(msg.Channel.Id, msg.RootID, "Could not abort the session: "+userFacingQwenPawError(err))
	}
	return p.postText(msg.Channel.Id, msg.RootID, fmt.Sprintf("Session `%s` was aborted.", sessionID))
}

func (p *Plugin) handleSessionInfo(msg incomingMessage, domainUserID string) error {
	sessionID, err := p.getStoredSessionID(msg, domainUserID)
	if err != nil {
		return p.postText(msg.Channel.Id, msg.RootID, "Could not load the session.")
	}
	scope := "DM"
	if msg.Channel != nil && !msg.IsDM {
		if msg.RootID != "" {
			scope = fmt.Sprintf("thread root `%s`", msg.RootID)
		} else {
			channelName := msg.Channel.Name
			if channelName == "" {
				channelName = msg.Channel.Id
			}
			scope = "channel `" + channelName + "`"
		}
	}
	if sessionID == "" {
		return p.postText(msg.Channel.Id, msg.RootID, fmt.Sprintf("No session exists for %s yet. It will be created automatically on the first question.", scope))
	}
	body := strings.Join([]string{
		fmt.Sprintf("- Scope: %s", scope),
		fmt.Sprintf("- Session ID: `%s`", sessionID),
		"",
		"Commands: `reset` starts a new session, `abort` cancels the current operation.",
	}, "\n")
	return p.postText(msg.Channel.Id, msg.RootID, body)
}

func (p *Plugin) postText(channelID, rootID, message string) error {
	account, err := p.getOrEnsureBot()
	if err != nil {
		return err
	}
	if err := p.ensureBotInChannel(channelID, account.UserID); err != nil {
		return p.postAsPluginFallback(channelID, rootID, "Could not add the bot to the channel. Please check channel permissions.")
	}
	_, appErr := p.API.CreatePost(&model.Post{
		UserId:    account.UserID,
		ChannelId: channelID,
		RootId:    rootID,
		Type:      "custom_paw_bot",
		Message:   strings.TrimSpace(message),
		Props: map[string]any{
			"from_bot": "true",
			"paw_bot":  "true",
		},
	})
	if appErr != nil {
		return fmt.Errorf("failed to create paw post: %w", appErr)
	}
	return nil
}

func (p *Plugin) postLongText(channelID, rootID, message string) error {
	parts := splitMessage(message, defaultMaxMattermostMessageLength)
	for _, part := range parts {
		if err := p.postText(channelID, rootID, part); err != nil {
			return err
		}
	}
	return nil
}

func (p *Plugin) postAsPluginFallback(channelID, rootID, message string) error {
	_, appErr := p.API.CreatePost(&model.Post{
		ChannelId: channelID,
		RootId:    rootID,
		Message:   strings.TrimSpace(message),
		Props: map[string]any{
			"from_plugin": "true",
			"paw_bot":     "true",
		},
	})
	if appErr != nil {
		return fmt.Errorf("failed to create fallback post: %w", appErr)
	}
	return nil
}
