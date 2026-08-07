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
	Provider llm.Provider
	APIKey   string
	Model    string
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
	content := fmt.Sprintf("provider: %s\napi_key: %s\nmodel: %s\n", config.Provider, config.APIKey, config.Model)
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		return err
	}
	return os.Chmod(path, 0600)
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
