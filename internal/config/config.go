// Package config manages promptcheck's local provider preference and API key.
package config

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/EyupEfeDuvarbasi/promptpatch/internal/llm"
)

type Config struct {
	Provider         llm.Provider
	APIKey           string
	Model            string
	ChatContextWords int
	ChatContextSet   bool
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
	keys := map[llm.Provider]string{
		llm.Anthropic: os.Getenv("ANTHROPIC_API_KEY"),
		llm.OpenAI:    os.Getenv("OPENAI_API_KEY"),
		llm.Gemini:    os.Getenv("GEMINI_API_KEY"),
	}
	if config.Provider == "" {
		config.Provider, err = chooseProvider(in, out, keys)
		if err != nil {
			return llm.Client{}, err
		}
	}
	key := keys[config.Provider]
	if key == "" {
		key = config.APIKey
	}
	if key == "" {
		fmt.Fprintf(out, "%s API anahtarı: ", config.Provider)
		key, err = bufio.NewReader(in).ReadString('\n')
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
	keys := map[llm.Provider]string{llm.Gemini: os.Getenv("GEMINI_API_KEY"), llm.OpenAI: os.Getenv("OPENAI_API_KEY"), llm.Anthropic: os.Getenv("ANTHROPIC_API_KEY")}
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
	key := keys[config.Provider]
	if key == "" {
		key = config.APIKey
	}
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
			config.APIKey = strings.TrimSpace(value)
		case "model":
			config.Model = strings.TrimSpace(value)
		case "chat_context_words":
			var words int
			if _, err := fmt.Sscan(value, &words); err != nil || (words != 0 && words != 800 && words != 2000 && words != 4000) {
				return Config{}, errors.New("geçersiz chat_context_words")
			}
			config.ChatContextWords = words
		case "chat_context_configured":
			config.ChatContextSet = strings.TrimSpace(value) == "true"
		}
	}
	if config.Provider != "" && config.Provider != llm.OpenAI && config.Provider != llm.Gemini && config.Provider != llm.Anthropic {
		return Config{}, fmt.Errorf("unsupported provider %q", config.Provider)
	}
	return config, nil
}

func Save(path string, config Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	content := fmt.Sprintf("provider: %s\napi_key: %s\nmodel: %s\nchat_context_words: %d\nchat_context_configured: %t\n", config.Provider, config.APIKey, config.Model, config.ChatContextWords, config.ChatContextSet)
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		return err
	}
	return os.Chmod(path, 0600)
}

// ConfigureChatContext asks once during setup how much nearby conversation may be used.
func ConfigureChatContext(path string, in io.Reader, out io.Writer) (Config, error) {
	config, err := Load(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return Config{}, err
	}
	if config.ChatContextSet {
		return config, nil
	}
	fmt.Fprintln(out, "Yakın sohbet bağlamı seçin:")
	fmt.Fprintln(out, "1) Kapalı")
	fmt.Fprintln(out, "2) Kısa — son 800 kelime")
	fmt.Fprintln(out, "3) Dengeli — son 2000 kelime (önerilen)")
	fmt.Fprintln(out, "4) Geniş — son 4000 kelime")
	fmt.Fprint(out, "> ")
	choice, err := bufio.NewReader(in).ReadString('\n')
	if err != nil && len(choice) == 0 {
		return Config{}, errors.New("sohbet bağlamı seçimi gerekli")
	}
	options := map[string]int{"1": 0, "2": 800, "3": 2000, "4": 4000}
	choice = strings.TrimSpace(choice)
	if choice == "" {
		choice = "3"
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

func chooseProvider(in io.Reader, out io.Writer, keys map[llm.Provider]string) (llm.Provider, error) {
	providers := []llm.Provider{llm.OpenAI, llm.Gemini, llm.Anthropic}
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
	choice, err := bufio.NewReader(in).ReadString('\n')
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
