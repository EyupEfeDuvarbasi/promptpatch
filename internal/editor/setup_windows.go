//go:build windows

package editor

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func setupCodex() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	localAppData := os.Getenv("LOCALAPPDATA")
	if localAppData == "" {
		localAppData = filepath.Join(home, "AppData", "Local")
	}
	binDir := filepath.Join(localAppData, "PromptPatch", "bin")
	editorPath := filepath.Join(binDir, "promptpatch-codex-editor.cmd")
	launcherPath := filepath.Join(binDir, "promptpatch-codex.cmd")
	if err := os.MkdirAll(filepath.Dir(editorPath), 0700); err != nil {
		return err
	}
	if err := os.WriteFile(editorPath, []byte(wrapperScript(executable)), 0700); err != nil {
		return err
	}
	codexPath, err := codexExecutablePath()
	if err != nil {
		return err
	}
	if err := os.WriteFile(launcherPath, []byte(codexLauncherScript(codexPath, editorPath)), 0700); err != nil {
		return err
	}

	block := codexBlock(launcherPath)
	for _, profilePath := range powerShellProfilePaths(home) {
		if err := os.MkdirAll(filepath.Dir(profilePath), 0700); err != nil {
			return err
		}
		contents, _ := os.ReadFile(profilePath)
		updated := replaceCodexBlock(string(contents), block)
		if updated == string(contents) {
			continue
		}
		if err := os.WriteFile(profilePath, []byte(updated), 0600); err != nil {
			return err
		}
	}
	return nil
}

func powerShellProfilePaths(home string) []string {
	return []string{
		filepath.Join(home, "Documents", "PowerShell", "Microsoft.PowerShell_profile.ps1"),
		filepath.Join(home, "Documents", "WindowsPowerShell", "Microsoft.PowerShell_profile.ps1"),
	}
}

func codexExecutablePath() (string, error) {
	path, err := exec.LookPath("codex.exe")
	if err == nil {
		return path, nil
	}
	path, err = exec.LookPath("codex")
	if err == nil {
		return path, nil
	}
	return "codex.exe", nil
}

func codexBlock(launcherPath string) string {
	launcher := powerShellQuote(launcherPath)
	return "\n" + codexMarker + "\n" +
		"function codex {\n" +
		"  & " + launcher + " @args\n" +
		"}\n" +
		codexEndMarker + "\n"
}

func wrapperScript(executable string) string {
	return "@echo off\r\n\"" + executable + "\" edit %*\r\n"
}

func codexLauncherScript(codexPath, editorPath string) string {
	return "@echo off\r\n" +
		"set \"PROMPTPATCH_HOST=codex\"\r\n" +
		"set \"VISUAL=" + editorPath + "\"\r\n" +
		"set \"EDITOR=" + editorPath + "\"\r\n" +
		"\"" + codexPath + "\" %*\r\n"
}

func powerShellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}
