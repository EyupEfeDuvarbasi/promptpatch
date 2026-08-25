package config

import (
	"bufio"
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
	t.Setenv("PROMPTPATCH_API_TOKEN", "secret")
	config, err := ConfigureRemoteServer(path, strings.NewReader("y\nhttps://promptpatch.example.com\n"), io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if config.ServerURL != "https://promptpatch.example.com" || !config.ServerSet {
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

func TestRemoteServerRequiresEnvironmentToken(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := Save(path, Config{ServerURL: "https://server.example.com", ServerSet: true}); err != nil {
		t.Fatal(err)
	}
	if _, _, ok := RemoteServer(path); ok {
		t.Fatal("remote server should require an environment token")
	}
	t.Setenv("PROMPTPATCH_API_TOKEN", "token")
	if url, token, ok := RemoteServer(path); !ok || url != "https://server.example.com" || token != "token" {
		t.Fatalf("url=%q token=%q ok=%t", url, token, ok)
	}
}

func TestLoadDropsLegacyServerToken(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("server_url: https://server.example.com\nserver_token: secret\nserver_configured: true\n"), 0600); err != nil {
		t.Fatal(err)
	}
	config, err := Load(path)
	if err != nil || config.ServerURL == "" {
		t.Fatalf("config=%#v err=%v", config, err)
	}
	if _, _, ok := RemoteServer(path); ok {
		t.Fatal("legacy token must not be used")
	}
}

func TestRemoteContextNeedsSeparateOptIn(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := Save(path, Config{RemoteContext: true}); err != nil {
		t.Fatal(err)
	}
	if !RemoteContextEnabled(path) {
		t.Fatal("saved opt-in should enable remote context")
	}
	t.Setenv("PROMPTPATCH_REMOTE_CONTEXT", "")
	if !RemoteContextEnabled(path) {
		t.Fatal("empty environment must not disable saved opt-in")
	}
}

func TestSetupReadersCanSharePipedInput(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	input := bufio.NewReader(strings.NewReader("1\ny\nhttps://server.example.com\n"))
	if _, err := ConfigureChatContext(path, input, io.Discard); err != nil {
		t.Fatal(err)
	}
	config, err := ConfigureRemoteServer(path, input, io.Discard)
	if err != nil || config.ServerURL != "https://server.example.com" {
		t.Fatalf("config=%#v err=%v", config, err)
	}
}
