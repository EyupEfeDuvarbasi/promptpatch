package server

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/EyupEfeDuvarbasi/promptpatch/internal/chat"
	"github.com/EyupEfeDuvarbasi/promptpatch/internal/score"
)

type workspacePrompt struct {
	Summary string `json:"summary"`
	Score   int    `json:"score"`
	Kind    string `json:"kind"`
	Project string `json:"project"`
}
type workspaceProject struct {
	ID      string            `json:"id"`
	Name    string            `json:"name"`
	Folder  string            `json:"folder"`
	Git     bool              `json:"git"`
	Prompts []workspacePrompt `json:"prompts"`
}
type workspaceData struct {
	Project  string             `json:"project"`
	Folder   string             `json:"folder"`
	Git      bool               `json:"git"`
	Prompts  []workspacePrompt  `json:"prompts"`
	Projects []workspaceProject `json:"projects"`
}

func projectStorePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	if runtime.GOOS == "windows" && os.Getenv("LOCALAPPDATA") != "" {
		return filepath.Join(os.Getenv("LOCALAPPDATA"), "Prompter", "projects.json"), nil
	}
	return filepath.Join(home, ".local", "share", "prompter", "projects.json"), nil
}

func projectPaths() []string {
	paths := []string{}
	store, err := projectStorePath()
	if err == nil {
		if data, err := os.ReadFile(store); err == nil {
			_ = json.Unmarshal(data, &paths)
		}
	}
	valid := paths[:0]
	for _, path := range paths {
		if info, err := os.Stat(path); err == nil && info.IsDir() && !contains(valid, path) {
			valid = append(valid, path)
		}
	}
	return valid
}

func contains(values []string, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}

func readProject(path string) workspaceProject {
	name := filepath.Base(path)
	id := fmt.Sprintf("%x", sha256.Sum256([]byte(path)))[:16]
	project := workspaceProject{ID: id, Name: name, Folder: name, Prompts: []workspacePrompt{}}
	if _, err := os.Stat(filepath.Join(path, ".git")); err == nil {
		project.Git = true
	}
	for _, text := range chat.UserPrompts(path, "codex", 50) {
		result := score.Evaluate(text)
		summary := strings.Join(strings.Fields(text), " ")
		if len([]rune(summary)) > 160 {
			summary = string([]rune(summary)[:157]) + "…"
		}
		project.Prompts = append(project.Prompts, workspacePrompt{Summary: summary, Score: result.Score, Kind: string(result.Kind), Project: name})
	}
	return project
}

func (s *Server) removeProject(w http.ResponseWriter, r *http.Request) {
	if !isLoopbackAddr(s.config.Addr) {
		writeError(w, 403, "proje kaldırma yalnızca yerel sunucuda kullanılabilir")
		return
	}
	id := r.PathValue("id")
	paths := projectPaths()
	kept := make([]string, 0, len(paths))
	found := false
	for _, path := range paths {
		candidate := fmt.Sprintf("%x", sha256.Sum256([]byte(path)))[:16]
		if candidate == id {
			found = true
			continue
		}
		kept = append(kept, path)
	}
	if !found {
		writeError(w, 404, "proje bulunamadı")
		return
	}
	store, err := projectStorePath()
	if err != nil {
		writeError(w, 500, "proje kaldırılamadı")
		return
	}
	data, _ := json.Marshal(kept)
	if os.WriteFile(store, data, 0600) != nil {
		writeError(w, 500, "proje kaldırılamadı")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) workspace(w http.ResponseWriter, _ *http.Request) {
	data := workspaceData{Prompts: []workspacePrompt{}, Projects: []workspaceProject{}}
	for _, path := range projectPaths() {
		project := readProject(path)
		data.Projects = append(data.Projects, project)
		data.Prompts = append(data.Prompts, project.Prompts...)
	}
	if len(data.Projects) > 0 {
		data.Project, data.Folder, data.Git = data.Projects[0].Name, data.Projects[0].Folder, data.Projects[0].Git
	}
	writeJSON(w, http.StatusOK, data)
}

func (s *Server) addProject(w http.ResponseWriter, r *http.Request) {
	if !isLoopbackAddr(s.config.Addr) {
		writeError(w, http.StatusForbidden, "klasör ekleme yalnızca yerel sunucuda kullanılabilir")
		return
	}
	var request struct {
		Path string `json:"path"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16*1024))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&request) != nil {
		writeError(w, http.StatusBadRequest, "geçerli bir klasör yolu gerekli")
		return
	}
	path, err := filepath.Abs(strings.TrimSpace(request.Path))
	if err != nil {
		writeError(w, http.StatusBadRequest, "klasör yolu geçersiz")
		return
	}
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		writeError(w, http.StatusBadRequest, "klasör bulunamadı")
		return
	}
	project, err := saveProject(path)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "proje listesi kaydedilemedi")
		return
	}
	writeJSON(w, http.StatusCreated, project)
}

func saveProject(path string) (workspaceProject, error) {
	paths := projectPaths()
	if !contains(paths, path) {
		paths = append(paths, path)
	}
	store, err := projectStorePath()
	if err != nil {
		return workspaceProject{}, err
	}
	if err := os.MkdirAll(filepath.Dir(store), 0700); err != nil {
		return workspaceProject{}, err
	}
	data, _ := json.Marshal(paths)
	if err := os.WriteFile(store, data, 0600); err != nil {
		return workspaceProject{}, err
	}
	return readProject(path), nil
}

func (s *Server) pickProject(w http.ResponseWriter, r *http.Request) {
	if !isLoopbackAddr(s.config.Addr) {
		writeError(w, http.StatusForbidden, "klasör seçme yalnızca yerel sunucuda kullanılabilir")
		return
	}
	command, err := folderPicker(r)
	if err != nil {
		writeError(w, http.StatusNotImplemented, "sistem klasör seçicisi açılamadı")
		return
	}
	output, err := command.Output()
	if err != nil {
		if command.ProcessState != nil && command.ProcessState.ExitCode() == 1 {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		writeError(w, http.StatusNotImplemented, "sistem klasör seçicisi açılamadı")
		return
	}
	path := strings.TrimSpace(string(output))
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		writeError(w, http.StatusBadRequest, "klasör bulunamadı")
		return
	}
	project, err := saveProject(path)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "proje listesi kaydedilemedi")
		return
	}
	writeJSON(w, http.StatusCreated, project)
}

func folderPicker(r *http.Request) (*exec.Cmd, error) {
	switch runtime.GOOS {
	case "darwin":
		if _, err := exec.LookPath("osascript"); err == nil {
			return exec.CommandContext(r.Context(), "osascript", "-e", `POSIX path of (choose folder with prompt "Prompter — Proje klasörünü seç")`), nil
		}
	case "windows":
		if _, err := exec.LookPath("powershell"); err == nil {
			return exec.CommandContext(r.Context(), "powershell", "-NoProfile", "-Command", `Add-Type -AssemblyName System.Windows.Forms;$d=New-Object System.Windows.Forms.FolderBrowserDialog;$d.Description='Prompter - Proje klasörünü seç';if($d.ShowDialog() -eq 'OK'){$d.SelectedPath}else{exit 1}`), nil
		}
	default:
		for _, name := range []string{"zenity", "yad"} {
			if _, err := exec.LookPath(name); err == nil {
				return exec.CommandContext(r.Context(), name, "--file-selection", "--directory", "--title=Prompter — Proje klasörünü seç"), nil
			}
		}
	}
	return nil, errors.New("folder picker unavailable")
}
