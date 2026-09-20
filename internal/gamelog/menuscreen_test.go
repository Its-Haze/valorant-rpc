package gamelog

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/its-haze/valorant-rpc/pkg/types"
)

// Lines copied from a real ShooterGame.log, trimmed to the columns that matter.
const (
	lineHomeOpen   = `[2026.09.20-11.16.59:564][277]LogMenuStackManager: Opening HomeScreen_PC_C`
	lineHomeClose  = `[2026.09.20-11.19.16:371][391]LogMenuStackManager: Closing HomeScreen_PC_C`
	lineProxyOpen  = `[2026.09.20-11.19.16:375][391]LogMenuStackManager: Opening WBP_Screen_ProxyShell_PC_C`
	lineLobby      = `[2026.09.20-11.19.16:376][391]LogUINavigationModel: Warning: Current Url: main/lobby`
	lineLobbyModal = `[2026.09.20-11.19.17:709][551]LogUINavigationModel: Warning: Current Url: main/lobby/modal/lobbymodeselectmodal`
	lineStore      = `[2026.09.19-17.05.40:051][716]LogUINavigationModel: Warning: Current Url: main/store/accessorystore`
	lineCareer     = `[2026.09.20-00.31.02:001][100]LogUINavigationModel: Warning: Current Url: main/career`
	lineSettings   = `[2026.09.20-11.24.20:093][354]LogUINavigationModel: Warning: Current Url: main/settingsingame`
	lineEmptyURL   = `[2026.09.20-11.24.29:445][992]LogUINavigationModel: Warning: Current Url: `
	lineNoise      = `[2026.09.20-11.17.01:318][277]LogGameFlowStateManager: Reconcile called with the current state: MainMenu.`
)

func readerOver(t *testing.T, lines ...string) *Reader {
	t.Helper()
	path := filepath.Join(t.TempDir(), "ShooterGame.log")
	body := ""
	for _, l := range lines {
		body += l + "\n"
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return New(Options{Path: path})
}

func TestMenuScreen(t *testing.T) {
	for name, tc := range map[string]struct {
		lines []string
		want  types.MenuScreen
	}{
		"a launch lands on the home screen": {
			[]string{lineNoise, lineHomeOpen, lineNoise},
			types.ScreenClient,
		},
		"clicking Play opens the lobby": {
			[]string{lineHomeOpen, lineHomeClose, lineProxyOpen, lineLobby},
			types.ScreenLobby,
		},
		"a modal over the lobby is still the lobby": {
			[]string{lineHomeClose, lineLobby, lineLobbyModal},
			types.ScreenLobby,
		},
		"going back to the home screen leaves the lobby": {
			[]string{lineLobby, lineHomeOpen},
			types.ScreenClient,
		},
		"the store is the client, not a lobby": {
			[]string{lineHomeClose, lineLobby, lineStore},
			types.ScreenClient,
		},
		"the career page is the client": {
			[]string{lineLobby, lineCareer},
			types.ScreenClient,
		},
		"settings opens over the lobby without leaving it": {
			[]string{lineLobby, lineSettings, lineEmptyURL},
			types.ScreenLobby,
		},
		"a log that has drawn no menu says nothing": {
			[]string{lineNoise},
			types.ScreenUnknown,
		},
		"an empty log says nothing": {
			nil,
			types.ScreenUnknown,
		},
	} {
		t.Run(name, func(t *testing.T) {
			got, err := readerOver(t, tc.lines...).MenuScreen()
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Errorf("MenuScreen() = %v, want %v", got, tc.want)
			}
		})
	}
}

// The log is append-only for a session, so a lobby visited an hour ago must
// not outvote the home screen the player is looking at now.
func TestMenuScreenAnswersFromTheLastMove(t *testing.T) {
	r := readerOver(t, lineLobby, lineNoise, lineStore, lineNoise, lineHomeOpen)
	if got, _ := r.MenuScreen(); got != types.ScreenClient {
		t.Errorf("MenuScreen() = %v, want the last line to win", got)
	}
}

func TestMenuScreenWithoutALogIsNotAFailure(t *testing.T) {
	_, err := New(Options{Path: filepath.Join(t.TempDir(), "absent.log")}).MenuScreen()
	if err != ErrNoLog {
		t.Errorf("err = %v, want ErrNoLog", err)
	}
}
