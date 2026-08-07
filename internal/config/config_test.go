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
