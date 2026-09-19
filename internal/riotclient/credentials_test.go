package riotclient

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const completeLockfile = "Riot Client:19484:53742:YyaTUtjvBvJQzZ1H0fUXPw:https"

func TestParseLockfileReadsEveryField(t *testing.T) {
	creds, err := ParseLockfile([]byte(completeLockfile))
	if err != nil {
		t.Fatalf("ParseLockfile: %v", err)
	}

	want := Credentials{
		Name:     "Riot Client",
		PID:      19484,
		Port:     53742,
		Password: "YyaTUtjvBvJQzZ1H0fUXPw",
		Protocol: "https",
	}
	if creds != want {
		t.Errorf("got %+v, want %+v", creds, want)
	}
}

func TestParseLockfileTolerantOfTrailingNewline(t *testing.T) {
	if _, err := ParseLockfile([]byte(completeLockfile + "\r\n")); err != nil {
		t.Fatalf("ParseLockfile: %v", err)
	}
}

// Every truncation of the real line is a partial write, so every prefix must
// be retryable. A prefix ending in "http" is the documented exception: it is
// indistinguishable from a genuine plaintext lockfile, and the health check
// rejects it instead.
func TestParseLockfileTreatsEveryPrefixAsPartial(t *testing.T) {
	for i := 0; i < len(completeLockfile); i++ {
		prefix := completeLockfile[:i]
		_, err := ParseLockfile([]byte(prefix))
		if strings.HasSuffix(prefix, ":http") {
			if err != nil {
				t.Errorf("prefix %q: got %v, want it to parse as a plaintext lockfile", prefix, err)
			}
			continue
		}
		if !errors.Is(err, ErrPartialLockfile) {
			t.Errorf("prefix %q: got %v, want ErrPartialLockfile", prefix, err)
		}
	}
}

func TestParseLockfileRejectsAnUnknownProtocol(t *testing.T) {
	if _, err := ParseLockfile([]byte("Riot Client:1:2:pw:ftp")); !errors.Is(err, ErrPartialLockfile) {
		t.Errorf("got %v, want ErrPartialLockfile", err)
	}
}

func TestParseLockfilePartialCases(t *testing.T) {
	cases := map[string]string{
		"empty":            "",
		"whitespace only":  "   \n",
		"no port yet":      "Riot Client:19484::YyaTUtjvBvJQzZ1H0fUXPw:https",
		"non-numeric port": "Riot Client:19484:port:pw:https",
		"non-numeric pid":  "Riot Client:pid:53742:pw:https",
	}
	for name, line := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := ParseLockfile([]byte(line)); !errors.Is(err, ErrPartialLockfile) {
				t.Errorf("got %v, want ErrPartialLockfile", err)
			}
		})
	}
}

// Extra fields can never come from a truncated write, so retrying will not
// fix them and the caller should see a real error.
func TestParseLockfileRejectsExtraFields(t *testing.T) {
	_, err := ParseLockfile([]byte(completeLockfile + ":extra"))
	if err == nil {
		t.Fatal("want an error for a six-field lockfile")
	}
	if errors.Is(err, ErrPartialLockfile) {
		t.Error("a six-field lockfile is malformed, not partial")
	}
}

func TestCredentialsBaseURLAndAuthHeader(t *testing.T) {
	creds := Credentials{Port: 53742, Password: "s3cret", Protocol: "https"}

	if got, want := creds.BaseURL(), "https://127.0.0.1:53742"; got != want {
		t.Errorf("BaseURL() = %q, want %q", got, want)
	}
	// base64("riot:s3cret")
	if got, want := creds.AuthHeader(), "Basic cmlvdDpzM2NyZXQ="; got != want {
		t.Errorf("AuthHeader() = %q, want %q", got, want)
	}
}

func TestCredentialsBaseURLDefaultsToHTTPS(t *testing.T) {
	creds := Credentials{Port: 1234}
	if got, want := creds.BaseURL(), "https://127.0.0.1:1234"; got != want {
		t.Errorf("BaseURL() = %q, want %q", got, want)
	}
}

const realCmdline = `"C:\Riot Games\Riot Client\RiotClientServices.exe" --app-port=53742 ` +
	`--remoting-auth-token=YyaTUt-jvBv_JQzZ1H0fUXPw --launch-product=valorant --launch-patchline=live`

func TestParseCmdlineExtractsPortAndToken(t *testing.T) {
	creds, err := ParseCmdline(realCmdline)
	if err != nil {
		t.Fatalf("ParseCmdline: %v", err)
	}
	if creds.Port != 53742 {
		t.Errorf("Port = %d, want 53742", creds.Port)
	}
	if want := "YyaTUt-jvBv_JQzZ1H0fUXPw"; creds.Password != want {
		t.Errorf("Password = %q, want %q", creds.Password, want)
	}
	if creds.Protocol != "https" {
		t.Errorf("Protocol = %q, want https", creds.Protocol)
	}
}

func TestParseCmdlineRejectsIncompleteCommandLines(t *testing.T) {
	cases := map[string]string{
		"neither flag": `"RiotClientServices.exe" --launch-product=valorant`,
		"port only":    `"RiotClientServices.exe" --app-port=53742`,
		"token only":   `"RiotClientServices.exe" --remoting-auth-token=abc`,
	}
	for name, cmdline := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := ParseCmdline(cmdline); err == nil {
				t.Error("want an error")
			}
		})
	}
}

type fakeLister struct {
	procs []ProcessInfo
	err   error
}

func (f fakeLister) RiotClientProcesses() ([]ProcessInfo, error) { return f.procs, f.err }

func TestCredentialsFromProcessesSkipsUnusableCommandLines(t *testing.T) {
	lister := fakeLister{procs: []ProcessInfo{
		{PID: 1, Cmdline: `"RiotClientServices.exe" --launch-product=valorant`},
		{PID: 2, Cmdline: realCmdline},
	}}

	creds, err := credentialsFromProcesses(lister)
	if err != nil {
		t.Fatalf("credentialsFromProcesses: %v", err)
	}
	if creds.PID != 2 || creds.Port != 53742 {
		t.Errorf("got %+v, want the second process", creds)
	}
}

func TestCredentialsFromProcessesErrorsWhenNoneUsable(t *testing.T) {
	lister := fakeLister{procs: []ProcessInfo{{PID: 1, Cmdline: "RiotClientServices.exe"}}}
	if _, err := credentialsFromProcesses(lister); err == nil {
		t.Error("want an error")
	}
}

func TestReadLockfileTreatsAMissingFileAsPartial(t *testing.T) {
	path := filepath.Join(t.TempDir(), "lockfile")
	if _, err := readLockfile(path); !errors.Is(err, ErrPartialLockfile) {
		t.Errorf("got %v, want ErrPartialLockfile", err)
	}
}

func TestReadLockfileParsesAWrittenFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "lockfile")
	if err := os.WriteFile(path, []byte(completeLockfile), 0o600); err != nil {
		t.Fatal(err)
	}

	creds, err := readLockfile(path)
	if err != nil {
		t.Fatalf("readLockfile: %v", err)
	}
	if creds.Port != 53742 {
		t.Errorf("Port = %d, want 53742", creds.Port)
	}
}

func TestDefaultLockfilePathUsesLocalAppData(t *testing.T) {
	t.Setenv("LOCALAPPDATA", filepath.Join("C:", "Users", "x", "AppData", "Local"))

	path, err := DefaultLockfilePath()
	if err != nil {
		t.Fatalf("DefaultLockfilePath: %v", err)
	}
	want := filepath.Join("C:", "Users", "x", "AppData", "Local", "Riot Games", "Riot Client", "Config", "lockfile")
	if path != want {
		t.Errorf("got %q, want %q", path, want)
	}
}

func TestDefaultLockfilePathErrorsWithoutLocalAppData(t *testing.T) {
	t.Setenv("LOCALAPPDATA", "")
	if _, err := DefaultLockfilePath(); err == nil {
		t.Error("want an error when LOCALAPPDATA is unset")
	}
}
