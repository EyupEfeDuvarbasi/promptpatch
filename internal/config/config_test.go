package config

import (
	"io"
	"os"
	"path/filepath"
	"runtime"
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
	if err != nil {
		t.Fatalf("mode=%v err=%v", info.Mode(), err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0600 {
		t.Fatalf("mode=%v", info.Mode())
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(content), "test-key") {
		t.Fatalf("environment API key was written to config: %s", content)
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

func TestConfigureChatContextAgainReopensChoice(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if _, err := ConfigureChatContext(path, strings.NewReader("1\n"), io.Discard); err != nil {
		t.Fatal(err)
	}
	config, err := ConfigureChatContextAgain(path, strings.NewReader("4\n"), io.Discard)
	if err != nil || config.ChatContextWords != 4000 || !config.ChatContextSet {
		t.Fatalf("config=%#v err=%v", config, err)
	}
}

func TestConfigureRemoteServerSavesEndpoint(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	config, err := ConfigureRemoteServer(path, strings.NewReader("y\nhttps://promptpatch.example.com\nsecret\n"), io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if config.ServerURL != "https://promptpatch.example.com" || config.ServerToken != "secret" || !config.ServerSet {
		t.Fatalf("config=%#v", config)
	}
	url, token, ok := RemoteServer(path)
	if !ok || url != "https://promptpatch.example.com" || token != "secret" {
		t.Fatalf("remote url=%q token=%q ok=%t", url, token, ok)
	}
}

func TestConfigureRemoteServerDefaultsToLocalWithoutInput(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	config, err := ConfigureRemoteServer(path, strings.NewReader(""), io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if config.ServerURL != "" || !config.ServerSet {
		t.Fatalf("config=%#v", config)
	}
	if _, _, ok := RemoteServer(path); ok {
		t.Fatal("remote server should be disabled")
	}
}

func TestRemoteServerUsesEnvironmentFirst(t *testing.T) {
	t.Setenv("PROMPTPATCH_API_URL", "https://env.example.com")
	t.Setenv("PROMPTPATCH_API_TOKEN", "env-token")
	url, token, ok := RemoteServer(filepath.Join(t.TempDir(), "missing.yaml"))
	if !ok || url != "https://env.example.com" || token != "env-token" {
		t.Fatalf("remote url=%q token=%q ok=%t", url, token, ok)
	}
}
