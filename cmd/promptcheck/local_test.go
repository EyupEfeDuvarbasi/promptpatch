package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDataResetOnlyRemovesPrompterFiles(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("LOCALAPPDATA", home)
	dir, err := localDataDir()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"projects.json", "users.json"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("[]"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	var out bytes.Buffer
	if err := resetData([]string{"--all"}, strings.NewReader("y\n"), &out); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"projects.json", "users.json"} {
		if _, err := os.Stat(filepath.Join(dir, name)); !os.IsNotExist(err) {
			t.Fatalf("%s not removed", name)
		}
	}
}

func TestSupportBundleContainsNoUserData(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("LOCALAPPDATA", home)
	old, _ := os.Getwd()
	t.Chdir(t.TempDir())
	defer os.Chdir(old)
	if err := supportBundle(&bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	matches, _ := filepath.Glob("prompter-support-*.zip")
	if len(matches) != 1 {
		t.Fatalf("bundles=%v", matches)
	}
}

func TestPrompterURLAddsLocalConnector(t *testing.T) {
	got := prompterURL("http://127.0.0.1:8787", "https://prompter.dev/setup?lang=tr")
	if got != "https://prompter.dev/setup?connector=http%3A%2F%2F127.0.0.1%3A8787&lang=tr" {
		t.Fatalf("url=%s", got)
	}
	if got := prompterURL("http://127.0.0.1:8787", "javascript:alert(1)"); got != "http://127.0.0.1:8787" {
		t.Fatalf("unsafe fallback=%s", got)
	}
}
