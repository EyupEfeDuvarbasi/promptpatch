package editor

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/EyupEfeDuvarbasi/promptpatch/internal/score"
)

func TestWrapDoesNotExceedColumnWidth(t *testing.T) {
	for _, line := range wrap("bu prompt yatay kaydırma gerektirmeden okunabilir olmalı", 12) {
		if len([]rune(line)) > 12 {
			t.Fatalf("line %q exceeds width", line)
		}
	}
}

func TestReadKeyAcceptsLoneEscape(t *testing.T) {
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	defer writer.Close()
	previous := terminalInput
	terminalInput = reader
	defer func() { terminalInput = previous }()
	if _, err := writer.Write([]byte{27}); err != nil {
		t.Fatal(err)
	}
	started := time.Now()
	if got := readKey(); got != "esc" || time.Since(started) > 200*time.Millisecond {
		t.Fatalf("key=%q elapsed=%s", got, time.Since(started))
	}
}

func TestReplaceCodexBlockUsesAbsoluteEditor(t *testing.T) {
	got := replaceCodexBlock("before\n# promptcheck Codex editor\ncodex() { old; }\nafter\n", codexBlock("/opt/bin/promptpatch-codex-editor"))
	if strings.Contains(got, "old") {
		t.Fatalf("block=%q", got)
	}
	if runtime.GOOS == "windows" {
		if !strings.Contains(got, "function codex") || !strings.Contains(got, "& '/opt/bin/promptpatch-codex-editor' @args") {
			t.Fatalf("windows block=%q", got)
		}
		return
	}
	if !strings.Contains(got, "PROMPTPATCH_HOST=codex VISUAL='/opt/bin/promptpatch-codex-editor'") {
		t.Fatalf("unix block=%q", got)
	}
}

func TestReplaceCodexBlockReplacesMarkedMultilineBlock(t *testing.T) {
	contents := "before\n" + codexMarker + "\nold line 1\nold line 2\n" + codexEndMarker + "\nafter\n"
	got := replaceCodexBlock(contents, codexBlock("/opt/bin/promptpatch-codex-editor"))
	if strings.Contains(got, "old line") || !strings.Contains(got, "before\n") || !strings.Contains(got, "after\n") {
		t.Fatalf("block=%q", got)
	}
}

func TestWrapperPassesEditorFile(t *testing.T) {
	got := wrapperScript("/opt/bin/promptcheck")
	if runtime.GOOS == "windows" {
		if got != "@echo off\r\n\"/opt/bin/promptcheck\" edit %*\r\n" {
			t.Fatalf("script=%q", got)
		}
		return
	}
	if got != "#!/bin/sh\nexec '/opt/bin/promptcheck' edit \"$@\"\n" {
		t.Fatalf("script=%q", got)
	}
}

func TestCodexLauncherSetsEditorEnvironment(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows-specific launcher script")
	}
	got := codexLauncherScript(`C:\Codex\codex.exe`, `C:\PromptPatch\promptpatch-codex-editor.cmd`)
	for _, want := range []string{
		`set "PROMPTPATCH_HOST=codex"`,
		`set "VISUAL=C:\PromptPatch\promptpatch-codex-editor.cmd"`,
		`set "EDITOR=C:\PromptPatch\promptpatch-codex-editor.cmd"`,
		`"C:\Codex\codex.exe" %*`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("script=%q, missing %q", got, want)
		}
	}
}

func TestCodexLauncherRunsCodexWithEditorEnvironment(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows-specific launcher script")
	}
	dir := t.TempDir()
	fakeCodex := dir + "\\fake-codex.cmd"
	output := dir + "\\env.txt"
	editorPath := dir + "\\promptpatch-codex-editor.cmd"
	launcher := dir + "\\promptpatch-codex.cmd"
	fakeScript := "@echo off\r\n" +
		"echo %PROMPTPATCH_HOST%>%1\r\n" +
		"echo %VISUAL%>>%1\r\n" +
		"echo %EDITOR%>>%1\r\n" +
		"echo %2>>%1\r\n"
	if err := os.WriteFile(fakeCodex, []byte(fakeScript), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(launcher, []byte(codexLauncherScript(fakeCodex, editorPath)), 0700); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("cmd", "/c", launcher, output, "sentinel")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("launcher failed: %v\n%s", err, out)
	}
	got, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"codex", editorPath, "sentinel"} {
		if !strings.Contains(string(got), want) {
			t.Fatalf("env=%q, missing %q", got, want)
		}
	}
}

func TestSetupCodexWritesWindowsProfileAndWrapper(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows-specific setup-codex integration")
	}
	root := t.TempDir()
	t.Setenv("USERPROFILE", root)
	t.Setenv("LOCALAPPDATA", root+"\\AppData\\Local")
	if err := setupCodex(); err != nil {
		t.Fatal(err)
	}
	for _, profilePath := range []string{
		root + "\\Documents\\PowerShell\\Microsoft.PowerShell_profile.ps1",
		root + "\\Documents\\WindowsPowerShell\\Microsoft.PowerShell_profile.ps1",
	} {
		profile, err := os.ReadFile(profilePath)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(profile), "function codex") || !strings.Contains(string(profile), "promptpatch-codex.cmd") {
			t.Fatalf("profile=%q", profile)
		}
	}
	wrapper, err := os.ReadFile(root + "\\AppData\\Local\\PromptPatch\\bin\\promptpatch-codex-editor.cmd")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(wrapper), " edit %*") {
		t.Fatalf("wrapper=%q", wrapper)
	}
	launcher, err := os.ReadFile(root + "\\AppData\\Local\\PromptPatch\\bin\\promptpatch-codex.cmd")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`set "PROMPTPATCH_HOST=codex"`, `set "VISUAL=`, `set "EDITOR=`, "codex"} {
		if !strings.Contains(string(launcher), want) {
			t.Fatalf("launcher=%q, missing %q", launcher, want)
		}
	}
}

func TestScreenLinesUseCarriageReturn(t *testing.T) {
	if got := screenText("a\nb"); got != "a\r\nb" {
		t.Fatalf("line ending=%q", got)
	}
}

func TestRemoveLastRunePreservesTurkishText(t *testing.T) {
	if got := string(removeLastRune([]byte("çerez"))); got != "çere" {
		t.Fatalf("answer=%q", got)
	}
}

func TestUsableRewriteRejectsRawQA(t *testing.T) {
	if !usableRewrite("şunu düzelt", "src/parser.go dosyasındaki parseInput fonksiyonunda boş girdi için hata dönsün.") {
		t.Fatal("expected usable rewrite")
	}
	if usableRewrite("şunu düzelt", "Soru: Hangi dosya? Cevap: src/parser.go") {
		t.Fatal("raw Q&A must be rejected")
	}
}

func TestUsableRewriteAllowsStructuredPrompt(t *testing.T) {
	candidate := "## Görev\nsrc/parser.go içindeki parseInput fonksiyonunda boş girdi hatasını düzelt.\n\n## Başarı ölçütleri\nBoş girdi hata döndürmeli."
	if !usableRewrite("şunu düzelt", candidate) {
		t.Fatal("structured prompt should be usable")
	}
}

func TestImproveWithRemoteServerUsesConfiguredAPI(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Fatalf("authorization=%q", r.Header.Get("Authorization"))
		}
		_, _ = w.Write([]byte(`{"original_score":30,"improved_score":80,"improved_prompt":"src/parser.go dosyasındaki boş girdi hatasını düzelt ve JSON hata açıklaması döndür.","source":"ollama"}`))
	}))
	defer server.Close()
	t.Setenv("PROMPTPATCH_API_URL", server.URL)
	t.Setenv("PROMPTPATCH_API_TOKEN", "test-token")

	response, ok := improveWithRemoteServer(context.Background(), "şunu düzelt", "", nil, nil)

	if !ok || !strings.Contains(response.ImprovedPrompt, "src/parser.go") {
		t.Fatalf("response=%#v ok=%t", response, ok)
	}
}

func TestImproveWithRemoteServerRejectsRegressedScore(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"original_score":80,"improved_score":30,"improved_prompt":"src/parser.go dosyasındaki boş girdi hatasını düzelt ve JSON hata açıklaması döndür.","source":"ollama"}`))
	}))
	defer server.Close()
	t.Setenv("PROMPTPATCH_API_URL", server.URL)

	if response, ok := improveWithRemoteServer(context.Background(), "şunu düzelt", "", nil, nil); ok {
		t.Fatalf("regressed server rewrite should be rejected: %#v", response)
	}
}

func TestImproveWithRemoteServerRejectsSameScore(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"original_score":24,"improved_score":24,"improved_prompt":"src/parser.go dosyasındaki boş girdi hatasını düzelt ve JSON hata açıklaması döndür.","source":"local"}`))
	}))
	defer server.Close()
	t.Setenv("PROMPTPATCH_API_URL", server.URL)

	if response, ok := improveWithRemoteServer(context.Background(), "şunu düzelt", "", nil, nil); ok {
		t.Fatalf("same-score server rewrite should be rejected: %#v", response)
	}
}

func TestShowableImprovementRequiresScoreIncrease(t *testing.T) {
	same := editorImprovement{
		Prompt:   "src/parser.go dosyasındaki boş girdi hatasını düzelt ve JSON hata açıklaması döndür.",
		Original: score.Result{Score: 24},
		Improved: score.Result{Score: 24},
	}
	if showableImprovement(same) {
		t.Fatal("same-score rewrite must not be shown as an improvement")
	}
	better := same
	better.Improved.Score = 60
	if !showableImprovement(better) {
		t.Fatal("higher-score rewrite should be shown")
	}
	better.Prompt = " "
	if showableImprovement(better) {
		t.Fatal("blank rewrite must not be shown")
	}
}

func TestImproveWithBestAvailableAsksLocalQuestionsBeforeBackend(t *testing.T) {
	questions := []string{}
	_, nextQuestions, complete := improveWithBestAvailable(context.Background(), "şunu düzelt", "", questions, nil)
	if !complete || len(nextQuestions) == 0 || len(nextQuestions) > 2 {
		t.Fatalf("complete=%t questions=%v", complete, nextQuestions)
	}
}

func TestImproveWithBestAvailablePassesAnswersToLocalFallback(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Nanosecond)
	defer cancel()
	questions := []string{"Hangi dosya?", "Beklenen sonuç?"}
	answers := []string{"src/parser.go", "boş girdi hata dönsün"}
	improvement, nextQuestions, complete := improveWithBestAvailable(ctx, "şunu düzelt", "", questions, answers)
	if !complete || len(nextQuestions) != 0 {
		t.Fatalf("complete=%t questions=%v", complete, nextQuestions)
	}
	if !strings.Contains(improvement.Prompt, "src/parser.go") || !strings.Contains(improvement.Prompt, "boş girdi hata dönsün") {
		t.Fatalf("fallback lost answers: %q", improvement.Prompt)
	}
}

func TestScoreSummaryIsDecisionFocused(t *testing.T) {
	if scoreBadge(64) != "64/100" || scoreDelta(15) != "  (+15)" || scoreDelta(-4) != "  (-4)" || scoreDelta(0) != "  (aynı)" {
		t.Fatal("score summary is not stable")
	}
}

func TestPromptPreviewTruncatesToTerminalBudget(t *testing.T) {
	preview := promptPreview("bir iki üç dört beş altı yedi sekiz dokuz on", 8, 3)
	if len(preview) != 3 || !strings.HasPrefix(preview[2], "… (") {
		t.Fatalf("preview=%q", preview)
	}
}
