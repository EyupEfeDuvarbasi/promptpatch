package server

import (
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

func (s *Server) setupStatus(w http.ResponseWriter, r *http.Request) {
	_, codexErr := exec.LookPath("codex")
	home, _ := os.UserHomeDir()
	wrapper := filepath.Join(home, ".local", "share", "promptpatch", "bin", "promptpatch-codex-editor")
	if runtime.GOOS == "windows" {
		wrapper = filepath.Join(os.Getenv("LOCALAPPDATA"), "PromptPatch", "bin", "promptpatch-codex-editor.cmd")
	}
	_, wrapperErr := os.Stat(wrapper)
	projects, prompts := projectPaths(), 0
	for _, path := range projects {
		prompts += len(readProject(path).Prompts)
	}
	users, _ := readUsers()
	_, signedIn := s.sessionUser(r)
	writeJSON(w, http.StatusOK, map[string]any{"codex": codexErr == nil, "wrapper": wrapperErr == nil, "account": len(users) > 0 || signedIn, "projects": len(projects), "prompts": prompts})
}
