package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestApplyQwenPawSSEEventHandlesConsoleChatOutput(t *testing.T) {
	state := qwenPawStreamState{}

	event := mustQwenPawEvent(t, map[string]any{
		"sequence_number": 2,
		"object":          "response",
		"status":          "in_progress",
		"output": []any{
			map[string]any{
				"role": "assistant",
				"content": []any{
					map[string]any{"type": "text", "text": "Hello from QwenPaw"},
				},
			},
		},
	})

	if !applyQwenPawSSEEvent(&state, "session-1", event) {
		t.Fatal("event was not applied")
	}
	if got, want := state.Text, "Hello from QwenPaw"; got != want {
		t.Fatalf("state.Text = %q, want %q", got, want)
	}
	if state.Done {
		t.Fatal("state.Done = true, want false")
	}
}

func TestApplyQwenPawSSEEventCompletesConsoleChatOutput(t *testing.T) {
	state := qwenPawStreamState{}

	event := mustQwenPawEvent(t, map[string]any{
		"sequence_number": 3,
		"object":          "response",
		"status":          "completed",
		"output": []any{
			map[string]any{
				"role": "assistant",
				"content": []any{
					map[string]any{"type": "text", "text": "Final answer"},
				},
			},
		},
	})

	if !applyQwenPawSSEEvent(&state, "session-1", event) {
		t.Fatal("event was not applied")
	}
	if got, want := state.Text, "Final answer"; got != want {
		t.Fatalf("state.Text = %q, want %q", got, want)
	}
	if !state.Done {
		t.Fatal("state.Done = false, want true")
	}
}

func TestFinalQwenPawMessageKeepsThinkingAndTools(t *testing.T) {
	state := qwenPawStreamState{
		Text: "<think>plan first</think>answer",
		Tools: []string{
			renderToolBlock("bash go test ./... (completed)", "", "ok\t./server\t0.1s"),
		},
	}

	final := finalQwenPawMessageOrEmpty(state)

	if !strings.Contains(final, "plan first") {
		t.Fatalf("final message does not include thinking: %q", final)
	}
	if !strings.Contains(final, "answer") {
		t.Fatalf("final message does not include answer: %q", final)
	}
	if !strings.Contains(final, "go test ./...") || !strings.Contains(final, "ok\t./server") {
		t.Fatalf("final message does not include tool output: %q", final)
	}
}

func mustQwenPawEvent(t *testing.T, value map[string]any) qwenPawSSEEvent {
	t.Helper()
	body, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return qwenPawSSEEvent{Data: body}
}
