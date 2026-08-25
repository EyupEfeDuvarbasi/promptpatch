package main

import (
	"strings"
	"testing"
)

func TestCorpusAndEventParsing(t *testing.T) {
	cases, err := loadCases("../../test/model-cases.jsonl", "all", 0)
	if err != nil || len(cases) != 60 {
		t.Fatalf("cases=%d err=%v", len(cases), err)
	}
	core, err := loadCases("../../test/model-cases.jsonl", "core", 0)
	if err != nil || len(core) != 20 {
		t.Fatalf("core=%d err=%v", len(core), err)
	}
	full, err := loadCases("../../test/model-cases.jsonl", "full", 0)
	if err != nil || len(full) != 40 {
		t.Fatalf("full=%d err=%v", len(full), err)
	}
	selected := selectCases(cases, []string{"bug-context-reference", "deployment-rollback"})
	if len(selected) != 2 {
		t.Fatalf("selected=%d", len(selected))
	}
	message, used, err := parseEvents([]byte("{\"type\":\"item.completed\",\"item\":{\"type\":\"agent_message\",\"text\":\"{\\\"improved_prompt\\\":\\\"ok\\\"}\"}}\n{\"type\":\"turn.completed\",\"usage\":{\"input_tokens\":10,\"output_tokens\":2}}\n"))
	if err != nil || !strings.Contains(message, "improved_prompt") || used.InputTokens != 10 || used.OutputTokens != 2 {
		t.Fatalf("message=%q usage=%+v err=%v", message, used, err)
	}
}
