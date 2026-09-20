package gamelog

import (
	"regexp"
	"strings"

	"github.com/its-haze/valorant-rpc/pkg/types"
)

// screenRe matches the two lines that move the out-of-game UI. The menu
// stack names the home screen; the navigation model names every other route.
var screenRe = regexp.MustCompile(`LogMenuStackManager: (Opening|Closing) HomeScreen_PC_C|LogUINavigationModel: Warning: Current Url: (\S*)`)

// lobbyRoute is the Play section. Its modals nest underneath it, so the mode
// and map pickers count as the lobby too.
const lobbyRoute = "main/lobby"

// settingsRoute is skipped rather than answered. Settings opens over
// whatever was behind it, and closing it logs no route at all.
const settingsRoute = "main/settingsingame"

// MenuScreen reports which section of the client is open, from the last line
// that moved the UI. Empty routes and the settings overlay are passed over.
func (r *Reader) MenuScreen() (types.MenuScreen, error) {
	blob, err := r.readTail()
	if err != nil {
		return types.ScreenUnknown, err
	}

	matches := screenRe.FindAllStringSubmatch(string(blob), -1)
	for i := len(matches) - 1; i >= 0; i-- {
		switch {
		case matches[i][1] == "Opening":
			return types.ScreenClient, nil
		case matches[i][1] == "Closing":
			// The home screen closes on the way to somewhere, and the route
			// it went to is the next line. Nothing to answer from here.
			continue
		}

		route := matches[i][2]
		if route == "" || strings.HasPrefix(route, settingsRoute) {
			continue
		}
		if route == lobbyRoute || strings.HasPrefix(route, lobbyRoute+"/") {
			return types.ScreenLobby, nil
		}
		return types.ScreenClient, nil
	}
	return types.ScreenUnknown, nil
}
