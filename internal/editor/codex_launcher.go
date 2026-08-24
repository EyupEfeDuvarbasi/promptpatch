package editor

// codexLauncherScript returns the Windows command wrapper used to launch
// Codex with promptpatch's editor configured through the environment.
//
// It is kept in a platform-neutral file because the editor tests reference
// the script generator even when they are running on a non-Windows host.
func codexLauncherScript(codexPath, editorPath string) string {
	return "@echo off\r\n" +
		"set \"PROMPTPATCH_HOST=codex\"\r\n" +
		"set \"VISUAL=" + editorPath + "\"\r\n" +
		"set \"EDITOR=" + editorPath + "\"\r\n" +
		"\"" + codexPath + "\" %*\r\n"
}
