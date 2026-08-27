package main

import (
	"archive/zip"
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/EyupEfeDuvarbasi/promptpatch/internal/editor"
)

func localDataDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	if runtime.GOOS == "windows" && os.Getenv("LOCALAPPDATA") != "" {
		return filepath.Join(os.Getenv("LOCALAPPDATA"), "Prompter"), nil
	}
	return filepath.Join(home, ".local", "share", "prompter"), nil
}

func localCommand(args []string, in io.Reader, out io.Writer) error {
	switch args[0] {
	case "doctor":
		return doctor(out)
	case "support-bundle":
		return supportBundle(out)
	case "uninstall":
		return uninstall(args[1:], in, out)
	case "data":
		if len(args) < 2 {
			return errors.New("data için status veya reset gerekli")
		}
		if args[1] == "status" {
			return dataStatus(out)
		}
		if args[1] == "reset" {
			return resetData(args[2:], in, out)
		}
	}
	return errors.New("bilinmeyen yerel komut")
}

func dataStatus(out io.Writer) error {
	dir, err := localDataDir()
	if err != nil {
		return err
	}
	fmt.Fprintln(out, "Prompter veri dizini:", dir)
	for _, name := range []string{"projects.json", "users.json", "session.key"} {
		info, err := os.Stat(filepath.Join(dir, name))
		if err == nil {
			fmt.Fprintf(out, "  %s: %d bayt\n", name, info.Size())
		} else {
			fmt.Fprintf(out, "  %s: yok\n", name)
		}
	}
	return nil
}

func resetData(args []string, in io.Reader, out io.Writer) error {
	dir, err := localDataDir()
	if err != nil {
		return err
	}
	names := []string{"projects.json"}
	label := "proje eşleşmeleri"
	if containsArg(args, "--auth") {
		names = []string{"users.json", "session.key"}
		label = "hesap ve oturumlar"
	}
	if containsArg(args, "--all") {
		names = []string{"projects.json", "users.json", "session.key"}
		label = "tüm Prompter verisi"
	}
	fmt.Fprintf(out, "Silinecek: %s. Codex oturumlarına dokunulmayacak. Devam? [y/N] ", label)
	answer, _ := bufio.NewReader(in).ReadString('\n')
	if strings.ToLower(strings.TrimSpace(answer)) != "y" {
		fmt.Fprintln(out, "İptal edildi.")
		return nil
	}
	for _, name := range names {
		if err := os.Remove(filepath.Join(dir, name)); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	fmt.Fprintln(out, "Yerel veri sıfırlandı.")
	return nil
}

func doctor(out io.Writer) error {
	dir, _ := localDataDir()
	fmt.Fprintf(out, "Prompter doctor\n  sürüm: %s\n  platform: %s/%s\n", version, runtime.GOOS, runtime.GOARCH)
	for _, command := range []string{"codex", folderPickerCommand()} {
		if command == "" {
			fmt.Fprintln(out, "  klasör seçici: bulunamadı")
			continue
		}
		if _, err := exec.LookPath(command); err != nil {
			fmt.Fprintf(out, "  %s: bulunamadı\n", command)
		} else {
			fmt.Fprintf(out, "  %s: hazır\n", command)
		}
	}
	for _, name := range []string{"projects.json", "users.json", "session.key"} {
		_, err := os.Stat(filepath.Join(dir, name))
		fmt.Fprintf(out, "  %s: %s\n", name, map[bool]string{true: "hazır", false: "yok"}[err == nil])
	}
	return nil
}

func supportBundle(out io.Writer) error {
	dir, err := localDataDir()
	if err != nil {
		return err
	}
	name := fmt.Sprintf("prompter-support-%s.zip", time.Now().Format("20060102-150405"))
	file, err := os.Create(name)
	if err != nil {
		return err
	}
	defer file.Close()
	archive := zip.NewWriter(file)
	entry, _ := archive.Create("diagnostics.json")
	payload := map[string]any{"version": version, "os": runtime.GOOS, "arch": runtime.GOARCH, "codex": commandExists("codex"), "folder_picker": folderPickerCommand() != ""}
	for _, item := range []string{"projects.json", "users.json"} {
		data, e := os.ReadFile(filepath.Join(dir, item))
		if e == nil {
			var rows []json.RawMessage
			if json.Unmarshal(data, &rows) == nil {
				payload[strings.TrimSuffix(item, ".json")+"_count"] = len(rows)
			}
		}
	}
	_ = json.NewEncoder(entry).Encode(payload)
	if err := archive.Close(); err != nil {
		return err
	}
	fmt.Fprintln(out, name)
	return nil
}

func uninstall(args []string, in io.Reader, out io.Writer) error {
	if err := editor.UninstallCodex(); err != nil {
		return err
	}
	if runtime.GOOS != "windows" {
		home, _ := os.UserHomeDir()
		for _, name := range []string{".profile", ".zshrc"} {
			path := filepath.Join(home, name)
			data, err := os.ReadFile(path)
			if err == nil {
				updated := strings.ReplaceAll(string(data), "\n# Prompter PATH\nexport PATH=\"$HOME/.local/bin:$PATH\"\n", "\n")
				_ = os.WriteFile(path, []byte(updated), 0600)
			}
		}
	}
	fmt.Fprintln(out, "Codex entegrasyonu kaldırıldı. Binary dosyasını installer veya paket yöneticisi kaldırabilir.")
	if containsArg(args, "--delete-data") {
		return resetData([]string{"--all"}, in, out)
	}
	return nil
}
func containsArg(args []string, want string) bool {
	for _, arg := range args {
		if arg == want {
			return true
		}
	}
	return false
}
func commandExists(name string) bool { _, err := exec.LookPath(name); return err == nil }
func folderPickerCommand() string {
	switch runtime.GOOS {
	case "darwin":
		if commandExists("osascript") {
			return "osascript"
		}
	case "windows":
		if commandExists("powershell") {
			return "powershell"
		}
	default:
		for _, name := range []string{"zenity", "yad"} {
			if commandExists(name) {
				return name
			}
		}
	}
	return ""
}
