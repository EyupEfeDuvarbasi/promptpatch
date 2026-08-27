//go:build !windows

package editor

import (
	"os"
	"path/filepath"
	"strings"
)

func setupCodex() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	rc := filepath.Join(home, ".zshrc")
	if strings.HasSuffix(os.Getenv("SHELL"), "bash") {
		rc = filepath.Join(home, ".bashrc")
	}
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	editorPath := filepath.Join(home, ".local", "share", "promptpatch", "bin", "promptpatch-codex-editor")
	if err := os.MkdirAll(filepath.Dir(editorPath), 0700); err != nil {
		return err
	}
	if err := os.WriteFile(editorPath, []byte(wrapperScript(executable)), 0700); err != nil {
		return err
	}
	contents, _ := os.ReadFile(rc)
	block := codexBlock(editorPath)
	updated := replaceCodexBlock(string(contents), block)
	if updated == string(contents) {
		return nil
	}
	return os.WriteFile(rc, []byte(updated), 0600)
}
func uninstallCodex() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	for _, name := range []string{".zshrc", ".bashrc"} {
		path := filepath.Join(home, name)
		data, e := os.ReadFile(path)
		if e == nil {
			if e = os.WriteFile(path, []byte(removeCodexBlock(string(data))), 0600); e != nil {
				return e
			}
		}
	}
	return nil
}

func codexBlock(editorPath string) string {
	editor := shellQuote(editorPath)
	return "\n" + codexMarker + "\ncodex() { PROMPTPATCH_HOST=codex VISUAL=" + editor + " EDITOR=" + editor + " command codex \"$@\"; }\n" + codexEndMarker + "\n"
}

func wrapperScript(executable string) string {
	return "#!/bin/sh\nexec " + shellQuote(executable) + " edit \"$@\"\n"
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\\"'\\\"'") + "'"
}
