package chat

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTrimUsesNewestWholeMessagesWithinWordBudget(t *testing.T) {
	got := trim([]Message{{"user", "ilk mesaj burada"}, {"assistant", "ikinci mesaj burada"}, {"user", "son mesaj"}}, 5)
	if got != "ASSISTANT: ikinci mesaj burada\n\nUSER: son mesaj" {
		t.Fatalf("context=%q", got)
	}
}

func TestReadMessagesUsesCodexUserAndAssistantOnly(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rollout.jsonl")
	content := `{"type":"session_meta","payload":{"cwd":"/repo"}}` + "\n" +
		`{"type":"response_item","payload":{"type":"message","role":"developer","content":[{"text":"ignore"}]}}` + "\n" +
		`{"type":"response_item","payload":{"type":"message","role":"user","content":[{"text":"İstek"}]}}` + "\n" +
		`{"type":"response_item","payload":{"type":"message","role":"assistant","content":[{"text":"Yanıt"}]}}` + "\n"
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	messages, matched := readMessages(path, "/repo", "codex")
	if !matched || len(messages) != 2 || strings.Join([]string{messages[0].Text, messages[1].Text}, "|") != "İstek|Yanıt" {
		t.Fatalf("matched=%v messages=%#v", matched, messages)
	}
}
