package main

import (
	"os"
	"regexp"
	"testing"

	"github.com/its-haze/valorant-rpc/internal/startup"
)

// installer is the NSIS script that restates identity Go already owns. Nothing
// but this test stops the two from drifting.
const installer = "../../build/windows/nsis/project.nsi"

// define pulls the value of an NSIS !define out of the installer script.
func define(t *testing.T, name string) string {
	t.Helper()
	body, err := os.ReadFile(installer)
	if err != nil {
		t.Fatalf("read %s: %v", installer, err)
	}
	re := regexp.MustCompile(`(?m)^!define\s+` + name + `\s+"([^"]*)"`)
	m := re.FindSubmatch(body)
	if m == nil {
		t.Fatalf("%s defines no %s", installer, name)
	}
	return string(m[1])
}

// Wails derives the single-instance mutex from the UniqueID, and the installer
// opens that mutex to refuse overwriting a running app.
func TestInstallerMutexMatchesSingleInstanceID(t *testing.T) {
	got := define(t, "APP_MUTEX")
	if want := "wails-app-" + singleInstanceID + "-sim"; got != want {
		t.Errorf("APP_MUTEX = %q, want %q; the installer would not see a running app", got, want)
	}
}

// The uninstaller deletes this value, so a mismatch leaves Windows launching a
// binary that no longer exists.
func TestInstallerAutorunValueMatchesStartup(t *testing.T) {
	if got := define(t, "AUTORUN_VALUE"); got != startup.ValueName {
		t.Errorf("AUTORUN_VALUE = %q, want %q", got, startup.ValueName)
	}
}

// NSIS has no upgrade code: this registry key is the product identity. Sharing
// one with another product means each installer overwrites the other's entry.
func TestInstallerUninstallKeyIsThisProduct(t *testing.T) {
	const want = "ValorantRPC"
	if got := define(t, "UNINST_KEY_NAME"); got != want {
		t.Errorf("UNINST_KEY_NAME = %q, want %q", got, want)
	}
}
