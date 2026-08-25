// Package config manages promptcheck's local provider preference and API key.
package config

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/EyupEfeDuvarbasi/promptpatch/internal/llm"
)

const chatContextWords = 3000

type Config struct {
	Provider         llm.Provider
	APIKey           string
	Model            string
	ChatContextWords int
	ChatContextSet   bool
	ServerURL        string
	ServerSet        bool
	RemoteContext    bool
}

type persistedConfig struct {
	Provider         llm.Provider `json:"provider,omitempty"`
	Model            string       `json:"model,omitempty"`
	ChatContextWords int          `json:"chat_context_words"`
	ChatContextSet   bool         `json:"chat_context_configured"`
	ServerURL        string       `json:"server_url,omitempty"`
	ServerSet        bool         `json:"server_configured"`
	RemoteContext    bool         `json:"remote_context_enabled"`
}

type keyResolver interface {
	Key(provider llm.Provider, config Config) string
}

type envConfigKeyResolver struct {
	env map[llm.Provider]string
}

func (r envConfigKeyResolver) Key(provider llm.Provider, config Config) string {
	if key := r.env[provider]; key != "" {
		return key
	}
	return config.APIKey
}

func DefaultPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "promptcheck", "config.yaml"), nil
}

func Resolve(path string, in io.Reader, out io.Writer) (llm.Client, error) {
	config, err := Load(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return llm.Client{}, err
	}
	keys := providerKeys()
	if config.Provider == "" {
		config.Provider, err = chooseProvider(in, out, keys)
		if err != nil {
			return llm.Client{}, err
		}
	}
	var resolver keyResolver = envConfigKeyResolver{env: keys}
	key := resolver.Key(config.Provider, config)
	if key == "" {
		fmt.Fprintf(out, "%s API anahtarı: ", config.Provider)
		key, err = buffered(in).ReadString('\n')
		if err != nil && len(key) == 0 {
			return llm.Client{}, errors.New("API anahtarı gerekli")
		}
		key = strings.TrimSpace(key)
		config.APIKey = key
	}
	client, err := llm.New(config.Provider, key)
	if err != nil {
		return llm.Client{}, err
	}
	if config.Model != "" {
		client.Model = config.Model
	} else {
		config.Model = client.Model
	}
	if err := Save(path, config); err != nil {
		return llm.Client{}, err
	}
	return client, nil
}

// ResolveAutomatic uses a saved provider or the least-cost available default without prompting.
func ResolveAutomatic(path string) (llm.Client, error) {
	config, err := Load(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return llm.Client{}, err
	}
	keys := providerKeys()
	if config.Provider == "" {
		for _, provider := range []llm.Provider{llm.Gemini, llm.OpenAI, llm.Anthropic} {
			if keys[provider] != "" {
				config.Provider = provider
				break
			}
		}
	}
	if config.Provider == "" {
		return llm.Client{}, errors.New("gerçek iyileştirme için model erişimi gerekli")
	}
	var resolver keyResolver = envConfigKeyResolver{env: keys}
	key := resolver.Key(config.Provider, config)
	client, err := llm.New(config.Provider, key)
	if err != nil {
		return llm.Client{}, err
	}
	if config.Model != "" {
		client.Model = config.Model
	}
	return client, nil
}

func Load(path string) (Config, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	var persisted persistedConfig
	if json.Unmarshal(content, &persisted) == nil {
		config := Config{
			Provider: persisted.Provider, Model: persisted.Model,
			ChatContextWords: persisted.ChatContextWords, ChatContextSet: persisted.ChatContextSet,
			ServerURL: persisted.ServerURL, ServerSet: persisted.ServerSet, RemoteContext: persisted.RemoteContext,
		}
		if !validContextWords(config.ChatContextWords) {
			return Config{}, errors.New("geçersiz chat_context_words")
		}
		config.ChatContextWords = migratedContextWords(config.ChatContextWords)
		if config.Provider != "" && !isConfigurableProvider(config.Provider) {
			return Config{}, fmt.Errorf("unsupported provider %q", config.Provider)
		}
		return config, nil
	}
	var config Config
	for _, line := range strings.Split(string(content), "\n") {
		key, value, ok := strings.Cut(line, ":")
		if !ok || strings.TrimSpace(line) == "" || strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		switch strings.TrimSpace(key) {
		case "provider":
			config.Provider = llm.Provider(strings.TrimSpace(value))
		case "api_key":
			// Legacy secrets are intentionally ignored; use the environment or prompt.
		case "model":
			config.Model = strings.TrimSpace(value)
		case "chat_context_words":
			var words int
			if _, err := fmt.Sscan(value, &words); err != nil || !validContextWords(words) {
				return Config{}, errors.New("geçersiz chat_context_words")
			}
			config.ChatContextWords = migratedContextWords(words)
		case "chat_context_configured":
			config.ChatContextSet = strings.TrimSpace(value) == "true"
		case "server_url":
			config.ServerURL = strings.TrimSpace(value)
		case "server_token":
			// Legacy server tokens are intentionally ignored; use PROMPTPATCH_API_TOKEN.
		case "server_configured":
			config.ServerSet = strings.TrimSpace(value) == "true"
		case "remote_context_enabled":
			config.RemoteContext = strings.TrimSpace(value) == "true"
		}
	}
	if config.Provider != "" && !isConfigurableProvider(config.Provider) {
		return Config{}, fmt.Errorf("unsupported provider %q", config.Provider)
	}
	return config, nil
}

func validContextWords(words int) bool {
	return words == 0 || words == 800 || words == 2000 || words == chatContextWords || words == 4000
}

func migratedContextWords(words int) int {
	if words > 0 {
		return chatContextWords
	}
	return 0
}

func Save(path string, config Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	content, err := json.MarshalIndent(persistedConfig{
		Provider: config.Provider, Model: config.Model,
		ChatContextWords: config.ChatContextWords, ChatContextSet: config.ChatContextSet,
		ServerURL: config.ServerURL, ServerSet: config.ServerSet, RemoteContext: config.RemoteContext,
	}, "", "  ")
	if err != nil {
		return err
	}
	content = append(content, '\n')
	if err := os.WriteFile(path, content, 0600); err != nil {
		return err
	}
	return os.Chmod(path, 0600)
}

func RemoteServer(path string) (string, string, bool) {
	url := strings.TrimSpace(os.Getenv("PROMPTPATCH_API_URL"))
	token := strings.TrimSpace(os.Getenv("PROMPTPATCH_API_TOKEN"))
	if url != "" {
		return url, token, token != ""
	}
	config, err := Load(path)
	if err != nil || !config.ServerSet || strings.TrimSpace(config.ServerURL) == "" {
		return "", "", false
	}
	return strings.TrimSpace(config.ServerURL), token, token != ""
}

// RemoteContextEnabled requires a separate opt-in before conversation history leaves the device.
func RemoteContextEnabled(path string) bool {
	if strings.TrimSpace(os.Getenv("PROMPTPATCH_REMOTE_CONTEXT")) == "1" {
		return true
	}
	config, err := Load(path)
	return err == nil && config.RemoteContext
}

// ConfigureRemoteServer asks once during setup whether Ctrl-G should call a central PromptPatch API.
func ConfigureRemoteServer(path string, in io.Reader, out io.Writer) (Config, error) {
	config, err := Load(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return Config{}, err
	}
	if config.ServerSet {
		return config, nil
	}
	reader := buffered(in)
	fmt.Fprintln(out, "Ctrl-G iyileştirmesi merkezi PromptPatch server üzerinden mi çalışsın? (y/N)")
	fmt.Fprint(out, "> ")
	choice, err := reader.ReadString('\n')
	if err != nil && len(choice) == 0 {
		config.ServerSet = true
		return config, Save(path, config)
	}
	choice = strings.ToLower(strings.TrimSpace(choice))
	if choice != "y" && choice != "yes" && choice != "e" && choice != "evet" {
		config.ServerSet = true
		return config, Save(path, config)
	}
	fmt.Fprint(out, "PromptPatch server URL: ")
	url, err := reader.ReadString('\n')
	if err != nil && len(url) == 0 {
		return Config{}, errors.New("server URL gerekli")
	}
	url = strings.TrimRight(strings.TrimSpace(url), "/")
	if url == "" {
		return Config{}, errors.New("server URL gerekli")
	}
	config.ServerURL = url
	config.ServerSet = true
	fmt.Fprintln(out, "Token saklanmaz. Codex'i PROMPTPATCH_API_TOKEN=<token> ile başlatın.")
	if config.ChatContextSet && config.ChatContextWords > 0 {
		fmt.Fprint(out, "Yakın sohbet bağlamı uzak sunucuya gönderilsin mi? (y/N)\n> ")
		choice, _ := reader.ReadString('\n')
		choice = strings.ToLower(strings.TrimSpace(choice))
		config.RemoteContext = choice == "y" || choice == "yes" || choice == "e" || choice == "evet"
	}
	if err := Save(path, config); err != nil {
		return Config{}, err
	}
	return config, nil
}

// ConfigureChatContext asks once whether nearby conversation may be used.
func ConfigureChatContext(path string, in io.Reader, out io.Writer) (Config, error) {
	config, err := Load(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return Config{}, err
	}
	if config.ChatContextSet {
		return config, nil
	}
	fmt.Fprintln(out, "Yakın sohbet bağlamı kullanılsın mı?")
	fmt.Fprintln(out, "1) Kapalı")
	fmt.Fprintln(out, "2) Açık — son 3000 kelime (önerilen)")
	fmt.Fprint(out, "> ")
	choice, err := buffered(in).ReadString('\n')
	if err != nil && len(choice) == 0 {
		return Config{}, errors.New("sohbet bağlamı seçimi gerekli")
	}
	options := map[string]int{"1": 0, "2": chatContextWords}
	choice = strings.TrimSpace(choice)
	if choice == "" {
		choice = "2"
	}
	words, ok := options[choice]
	if !ok {
		return Config{}, errors.New("geçersiz sohbet bağlamı seçimi")
	}
	config.ChatContextWords = words
	config.ChatContextSet = true
	if err := Save(path, config); err != nil {
		return Config{}, err
	}
	return config, nil
}

// ConfigureChatContextAgain explicitly reopens the context choice.
func ConfigureChatContextAgain(path string, in io.Reader, out io.Writer) (Config, error) {
	config, err := Load(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return Config{}, err
	}
	config.ChatContextSet = false
	if err := Save(path, config); err != nil {
		return Config{}, err
	}
	return ConfigureChatContext(path, in, out)
}

func chooseProvider(in io.Reader, out io.Writer, keys map[llm.Provider]string) (llm.Provider, error) {
	providers := configurableProviderNames()
	available := []llm.Provider{}
	for _, provider := range providers {
		if keys[provider] != "" {
			available = append(available, provider)
		}
	}
	if len(available) == 1 {
		return available[0], nil
	}
	if len(available) == 0 {
		available = providers
	}
	fmt.Fprintln(out, "Varsayılan sağlayıcıyı seçin:")
	for i, provider := range available {
		fmt.Fprintf(out, "%d) %s\n", i+1, provider)
	}
	fmt.Fprint(out, "> ")
	choice, err := buffered(in).ReadString('\n')
	if err != nil && len(choice) == 0 {
		return "", errors.New("sağlayıcı seçimi gerekli")
	}
	choice = strings.TrimSpace(choice)
	for i, provider := range available {
		if choice == fmt.Sprint(i+1) || choice == string(provider) {
			return provider, nil
		}
	}
	return "", errors.New("geçersiz sağlayıcı seçimi")
}

func providerKeys() map[llm.Provider]string {
	keys := map[llm.Provider]string{}
	for _, provider := range llm.ConfigurableProviders() {
		keys[provider.Provider] = os.Getenv(provider.EnvKey)
	}
	return keys
}

func configurableProviderNames() []llm.Provider {
	infos := llm.ConfigurableProviders()
	providers := make([]llm.Provider, 0, len(infos))
	for _, info := range infos {
		providers = append(providers, info.Provider)
	}
	return providers
}

func isConfigurableProvider(provider llm.Provider) bool {
	info, ok := llm.ProviderDetails(provider)
	return ok && info.Configurable
}

func buffered(in io.Reader) *bufio.Reader {
	if reader, ok := in.(*bufio.Reader); ok {
		return reader
	}
	return bufio.NewReader(in)
}
