//go:build darwin

// Package dialog opens the OS's native folder-choose dialog. A browser
// <input type=file> can't return a real filesystem directory to save
// into (sandboxed by design) — this app needs an actual path to write
// converted images to, so the picker has to come from the OS, not the
// page.
package dialog

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// ChooseFolder opens the native Finder folder-choose dialog and returns
// the chosen absolute path, or "" if the user canceled.
func ChooseFolder() (string, error) {
	return runAppleScript(`tell application "Finder" to activate
POSIX path of (choose folder with prompt "Choose where to save converted images")`)
}

// runAppleScript activates Finder first — without it, the dialog is
// owned by a background process (this binary has no Dock icon/foreground
// app status of its own), so macOS can open it behind the Chrome
// app-mode window instead of in front, which looks exactly like the
// button did nothing.
func runAppleScript(script string) (string, error) {
	cmd := exec.Command("osascript", "-e", script)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		if strings.Contains(stderr.String(), "User canceled") {
			return "", nil
		}
		return "", fmt.Errorf("%s", strings.TrimSpace(stderr.String()))
	}
	return strings.TrimSpace(string(out)), nil
}
