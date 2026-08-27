package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestWorkspaceDoesNotExposeAbsolutePath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("LOCALAPPDATA", home)
	r := httptest.NewRecorder()
	New(Config{}).ServeHTTP(r, httptest.NewRequest(http.MethodGet, "/v1/workspace", nil))
	if r.Code != http.StatusOK || strings.Contains(r.Body.String(), `"/home/`) || strings.Contains(r.Body.String(), `"assistant"`) {
		t.Fatalf("unsafe workspace response: %s", r.Body.String())
	}
}

func TestProjectWithoutHistoryReturnsEmptyPromptArray(t *testing.T) {
	project := readProject(t.TempDir())
	if project.Prompts == nil || len(project.Prompts) != 0 {
		t.Fatalf("prompts=%#v", project.Prompts)
	}
}

func TestRemoveProjectOnlyRemovesMapping(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("LOCALAPPDATA", home)
	projectDir := t.TempDir()
	project, err := saveProject(projectDir)
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRecorder()
	New(Config{}).ServeHTTP(r, httptest.NewRequest(http.MethodDelete, "/v1/projects/"+project.ID, nil))
	if r.Code != http.StatusNoContent {
		t.Fatalf("status=%d body=%s", r.Code, r.Body.String())
	}
	if _, err := os.Stat(projectDir); err != nil {
		t.Fatalf("project folder changed: %v", err)
	}
}
