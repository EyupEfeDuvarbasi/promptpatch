// Package chat reads the current project's recent CLI conversation locally.
package chat

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type Message struct {
	Role string
	Text string
}

type Result struct {
	Text   string
	Source string
}

// UserPrompts returns only user-authored messages from the newest matching
// local session. Assistant responses and tool output are never included.
func UserPrompts(cwd, host string, limit int) []string {
	if limit < 1 {
		return nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	for _, source := range sources(host) {
		for _, path := range recentFiles(map[string]string{
			"codex": filepath.Join(home, ".codex", "sessions"), "gemini": filepath.Join(home, ".gemini", "tmp"),
			"claude": filepath.Join(home, ".claude", "projects"), "copilot": filepath.Join(home, ".copilot"),
		}[source], source) {
			messages, matched := readMessages(path, cwd, source)
			if !matched {
				continue
			}
			var prompts []string
			for _, message := range messages {
				if message.Role == "user" && strings.TrimSpace(message.Text) != "" {
					prompts = append(prompts, message.Text)
				}
			}
			if len(prompts) > limit {
				prompts = prompts[len(prompts)-limit:]
			}
			return prompts
		}
	}
	return nil
}

// Load returns recent whole user/assistant messages. It never sends or stores chat content.
func Load(cwd, host string, words int) Result {
	if words <= 0 {
		return Result{}
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return Result{}
	}
	for _, source := range sources(host) {
		if messages := loadSource(home, cwd, source); len(messages) > 0 {
			return Result{Text: trim(messages, words), Source: source}
		}
	}
	return Result{}
}

func sources(host string) []string {
	if host == "codex" || host == "gemini" || host == "claude" || host == "copilot" {
		return []string{host}
	}
	return []string{"codex", "gemini", "claude", "copilot"}
}

func loadSource(home, cwd, source string) []Message {
	root := map[string]string{
		"codex":   filepath.Join(home, ".codex", "sessions"),
		"gemini":  filepath.Join(home, ".gemini", "tmp"),
		"claude":  filepath.Join(home, ".claude", "projects"),
		"copilot": filepath.Join(home, ".copilot"),
	}[source]
	files := recentFiles(root, source)
	for _, path := range files {
		messages, matched := readMessages(path, cwd, source)
		if matched && len(messages) > 0 {
			return messages
		}
	}
	return nil
}

func recentFiles(root, source string) []string {
	type recentFile struct {
		path    string
		modTime int64
	}
	var files []recentFile
	_ = filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() || !strings.HasSuffix(path, ".jsonl") {
			return nil
		}
		if source == "gemini" && !strings.Contains(path, string(filepath.Separator)+"chats"+string(filepath.Separator)) {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return nil
		}
		files = append(files, recentFile{path: path, modTime: info.ModTime().UnixNano()})
		return nil
	})
	sort.Slice(files, func(i, j int) bool {
		return files[i].modTime > files[j].modTime
	})
	if len(files) > 20 {
		files = files[:20]
	}
	paths := make([]string, len(files))
	for i, file := range files {
		paths[i] = file.path
	}
	return paths
}

func readMessages(path, cwd, source string) ([]Message, bool) {
	file, err := os.Open(path)
	if err != nil {
		return nil, false
	}
	defer file.Close()
	matched := source == "claude" && strings.Contains(path, strings.ReplaceAll(cwd, "/", "-"))
	var messages []Message
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 4096), 2*1024*1024)
	first := true
	for scanner.Scan() {
		line := scanner.Bytes()
		if first && source == "codex" {
			first = false
			if !strings.Contains(string(line), cwd) {
				return nil, false
			}
		}
		if strings.Contains(string(line), cwd) {
			matched = true
		}
		messages = append(messages, lineMessages(line)...)
	}
	return messages, matched
}

func lineMessages(line []byte) []Message {
	var value any
	if json.Unmarshal(line, &value) != nil {
		return nil
	}
	var messages []Message
	collect(value, &messages)
	return messages
}

func collect(value any, messages *[]Message) {
	switch item := value.(type) {
	case []any:
		for _, child := range item {
			collect(child, messages)
		}
	case map[string]any:
		role, _ := item["role"].(string)
		if role == "" {
			role, _ = item["type"].(string)
		}
		if role == "user" || role == "assistant" {
			if text := textOf(item["content"]); text != "" {
				*messages = append(*messages, Message{Role: role, Text: text})
				return
			}
		}
		for _, child := range item {
			collect(child, messages)
		}
	}
}

func textOf(value any) string {
	switch item := value.(type) {
	case string:
		return strings.TrimSpace(item)
	case []any:
		parts := make([]string, 0, len(item))
		for _, child := range item {
			if object, ok := child.(map[string]any); ok {
				if text, ok := object["text"].(string); ok {
					parts = append(parts, text)
				}
			}
		}
		return strings.TrimSpace(strings.Join(parts, "\n"))
	}
	return ""
}

func trim(messages []Message, limit int) string {
	var selected []Message
	used := 0
	for i := len(messages) - 1; i >= 0; i-- {
		count := len(strings.Fields(messages[i].Text))
		if count == 0 || (used > 0 && used+count > limit) {
			continue
		}
		selected = append(selected, messages[i])
		used += count
		if used >= limit {
			break
		}
	}
	for left, right := 0, len(selected)-1; left < right; left, right = left+1, right-1 {
		selected[left], selected[right] = selected[right], selected[left]
	}
	parts := make([]string, 0, len(selected))
	for _, message := range selected {
		parts = append(parts, strings.ToUpper(message.Role)+": "+message.Text)
	}
	return strings.Join(parts, "\n\n")
}
