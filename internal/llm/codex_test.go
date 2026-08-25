package llm

import "testing"

func TestParseCodexEvents(t *testing.T) {
	data := []byte("{\"type\":\"item.completed\",\"item\":{\"type\":\"agent_message\",\"text\":\"{\\\"improved_prompt\\\":\\\"ok\\\"}\"}}\n{\"type\":\"turn.completed\",\"usage\":{\"input_tokens\":10,\"cached_input_tokens\":8,\"output_tokens\":2}}\n")
	message, usage, err := parseCodexEvents(data)
	if err != nil || message == "" || usage.InputTokens != 10 || usage.CachedInputTokens != 8 || usage.OutputTokens != 2 {
		t.Fatalf("message=%q usage=%+v err=%v", message, usage, err)
	}
}
