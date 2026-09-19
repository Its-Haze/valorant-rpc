// Package riotclient is a transport for the Riot Client's local API:
// credential discovery, loopback HTTP and the WAMP event websocket.
package riotclient

import (
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	gopsutilprocess "github.com/shirou/gopsutil/v4/process"
)

// lockfileFields is the field count of a complete name:PID:port:password:protocol line.
const lockfileFields = 5

// ErrPartialLockfile reports a lockfile caught mid-write. The Riot Client
// writes it non-atomically, so the read is worth retrying rather than failing.
var ErrPartialLockfile = errors.New("riotclient: lockfile is incomplete")

// Credentials are the local API's connection details, published by the Riot
// Client's lockfile and repeated on its command line.
type Credentials struct {
	Name     string
	PID      int
	Port     int
	Password string
	Protocol string
}

// BaseURL is the origin every local API request goes to. The Riot Client
// binds loopback only, so the host is never configurable.
func (c Credentials) BaseURL() string {
	scheme := c.Protocol
	if scheme == "" {
		scheme = "https"
	}
	return fmt.Sprintf("%s://127.0.0.1:%d", scheme, c.Port)
}

// WebSocketURL is the WAMP event socket's address on the same port.
func (c Credentials) WebSocketURL() string {
	scheme := "wss"
	if c.Protocol == "http" {
		scheme = "ws"
	}
	return fmt.Sprintf("%s://127.0.0.1:%d/", scheme, c.Port)
}

// AuthHeader is the Authorization value the local API expects: the fixed
// user "riot" against the lockfile password.
func (c Credentials) AuthHeader() string {
	return "Basic " + base64.StdEncoding.EncodeToString([]byte("riot:"+c.Password))
}

// ParseLockfile reads the name:PID:port:password:protocol line. A truncated
// write is always a prefix of the full line, so short input is retryable.
func ParseLockfile(data []byte) (Credentials, error) {
	line := strings.TrimSpace(string(data))
	if line == "" {
		return Credentials{}, ErrPartialLockfile
	}

	parts := strings.Split(line, ":")
	if len(parts) > lockfileFields {
		return Credentials{}, fmt.Errorf("riotclient: lockfile has %d fields, want %d", len(parts), lockfileFields)
	}
	if len(parts) < lockfileFields {
		return Credentials{}, ErrPartialLockfile
	}
	for _, p := range parts {
		if p == "" {
			return Credentials{}, ErrPartialLockfile
		}
	}

	pid, err := strconv.Atoi(parts[1])
	if err != nil {
		return Credentials{}, ErrPartialLockfile
	}
	port, err := strconv.Atoi(parts[2])
	if err != nil {
		return Credentials{}, ErrPartialLockfile
	}
	// The protocol is the last field, so a truncated write lands here most
	// often. "http" is the one prefix of "https" no parser can rule out.
	if parts[4] != "http" && parts[4] != "https" {
		return Credentials{}, ErrPartialLockfile
	}

	return Credentials{
		Name:     parts[0],
		PID:      pid,
		Port:     port,
		Password: parts[3],
		Protocol: parts[4],
	}, nil
}

// DefaultLockfilePath is %LOCALAPPDATA%\Riot Games\Riot Client\Config\lockfile.
func DefaultLockfilePath() (string, error) {
	local := os.Getenv("LOCALAPPDATA")
	if local == "" {
		return "", errors.New("riotclient: LOCALAPPDATA is not set")
	}
	return filepath.Join(local, "Riot Games", "Riot Client", "Config", "lockfile"), nil
}

// readLockfile parses the lockfile at path. A missing file is reported as a
// partial write too: the client may not have written it yet.
func readLockfile(path string) (Credentials, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Credentials{}, ErrPartialLockfile
		}
		return Credentials{}, fmt.Errorf("riotclient: reading lockfile: %w", err)
	}
	return ParseLockfile(data)
}

var (
	appPortPattern   = regexp.MustCompile(`--app-port=(\d+)`)
	authTokenPattern = regexp.MustCompile(`--remoting-auth-token=([\w-]+)`)
)

// ParseCmdline pulls the port and password out of a Riot Client command
// line, which carries both when the lockfile is unreadable.
func ParseCmdline(cmdline string) (Credentials, error) {
	port := appPortPattern.FindStringSubmatch(cmdline)
	token := authTokenPattern.FindStringSubmatch(cmdline)
	if len(port) < 2 || len(token) < 2 {
		return Credentials{}, errors.New("riotclient: command line carries no --app-port and --remoting-auth-token pair")
	}

	n, err := strconv.Atoi(port[1])
	if err != nil {
		return Credentials{}, fmt.Errorf("riotclient: invalid --app-port: %w", err)
	}

	return Credentials{Name: "Riot Client", Port: n, Password: token[1], Protocol: "https"}, nil
}

// ProcessInfo is one running Riot Client process.
type ProcessInfo struct {
	PID     int
	Cmdline string
}

// ProcessLister lists the running Riot Client processes. Faked in tests so
// discovery runs without a Riot Client.
type ProcessLister interface {
	RiotClientProcesses() ([]ProcessInfo, error)
}

// riotClientExe is the process hosting the local API, per pkg/constants.
const riotClientExe = "riotclientservices.exe"

type gopsutilLister struct{}

func (gopsutilLister) RiotClientProcesses() ([]ProcessInfo, error) {
	procs, err := gopsutilprocess.Processes()
	if err != nil {
		return nil, err
	}

	var found []ProcessInfo
	for _, p := range procs {
		name, err := p.Name()
		if err != nil || !strings.EqualFold(name, riotClientExe) {
			// Processes can exit between listing and inspection; skip them
			// rather than failing the whole scan.
			continue
		}
		cmdline, err := p.Cmdline()
		if err != nil {
			continue
		}
		found = append(found, ProcessInfo{PID: int(p.Pid), Cmdline: cmdline})
	}
	if len(found) == 0 {
		return nil, errors.New("riotclient: no Riot Client process is running")
	}
	return found, nil
}

// credentialsFromProcesses returns the first process whose command line
// carries a usable port and token pair.
func credentialsFromProcesses(lister ProcessLister) (Credentials, error) {
	procs, err := lister.RiotClientProcesses()
	if err != nil {
		return Credentials{}, err
	}
	for _, p := range procs {
		creds, err := ParseCmdline(p.Cmdline)
		if err != nil {
			continue
		}
		creds.PID = p.PID
		return creds, nil
	}
	return Credentials{}, errors.New("riotclient: no Riot Client process exposes the local API")
}
