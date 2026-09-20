package daemon

import (
	"errors"

	"github.com/rs/zerolog"

	"github.com/its-haze/valorant-rpc/internal/gamelog"
	"github.com/its-haze/valorant-rpc/internal/state"
	"github.com/its-haze/valorant-rpc/pkg/types"
)

// ScreenReader reports which section of the out-of-game client is open.
// *gamelog.Reader satisfies it.
type ScreenReader interface {
	MenuScreen() (types.MenuScreen, error)
}

// GameLogScreens is the MenuLookup backed by Valorant's own log. Riot's
// presence payload cannot tell the two menu halves apart; this can.
type GameLogScreens struct {
	reader ScreenReader
	state  *state.Manager
	logger zerolog.Logger

	// errLogged keeps an unreadable log to one line rather than one per
	// poll, because the poll runs for as long as the client is open.
	errLogged bool
}

// NewGameLogScreens builds the lookup. The reader is an interface so the
// daemon tests drive it without a log file.
func NewGameLogScreens(reader ScreenReader, stateMgr *state.Manager, logger zerolog.Logger) *GameLogScreens {
	return &GameLogScreens{reader: reader, state: stateMgr, logger: logger.With().Str("component", "screens").Logger()}
}

// Read applies the current menu screen to the state. An unreadable log
// leaves the screen unknown, which reads as the client and never the lobby.
func (g *GameLogScreens) Read() {
	screen, err := g.reader.MenuScreen()
	if err != nil {
		if !errors.Is(err, gamelog.ErrNoLog) && !g.errLogged {
			g.errLogged = true
			g.logger.Debug().Err(err).Msg("Could not read the game log for the menu screen")
		}
		return
	}

	g.errLogged = false
	g.state.Apply(func(st *state.State) { st.MenuScreen = screen })
}
