package config

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/EyupEfeDuvarbasi/promptpatch/internal/llm"
)

func TestResolveSavesEnvironmentProvider(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "test-key")
	t.Setenv("GEMINI_API_KEY", "")
	t.Setenv("ANTHROPIC_API_KEY", "")
	path := filepath.Join(t.TempDir(), "config.yaml")
	client, err := Resolve(path, strings.NewReader(""), io.Discard)
	if err != nil || client.Provider != llm.OpenAI {
		t.Fatalf("client=%#v err=%v", client, err)
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("mode=%v err=%v", info.Mode(), err)
	}
}

func TestConfigureChatContextSavesSelectedWordLimit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	config, err := ConfigureChatContext(path, strings.NewReader("3\n"), io.Discard)
	if err != nil || config.ChatContextWords != 2000 || !config.ChatContextSet {
		t.Fatalf("config=%#v err=%v", config, err)
	}
	loaded, err := Load(path)
	if err != nil || loaded.ChatContextWords != 2000 || !loaded.ChatContextSet {
		t.Fatalf("loaded=%#v err=%v", loaded, err)
	}
}

func TestConfigureChatContextUsesBalancedDefault(t *testing.T) {
	config, err := ConfigureChatContext(filepath.Join(t.TempDir(), "config.yaml"), strings.NewReader("\n"), io.Discard)
	if err != nil || config.ChatContextWords != 2000 {
		t.Fatalf("config=%#v err=%v", config, err)
	}
}
