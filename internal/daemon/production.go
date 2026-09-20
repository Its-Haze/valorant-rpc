package daemon

import (
	"time"

	"github.com/its-haze/valorant-rpc/internal/discord"
	"github.com/its-haze/valorant-rpc/internal/riotclient"
)

// Compile-time checks that the real clients satisfy Connector.
var (
	_ Connector = (*discord.Client)(nil)
	_ Connector = (*riotclient.Client)(nil)
	_ Connector = (*RiotSource)(nil)
)

// Default retry/poll cadences for local process/IPC checks.
const (
	DefaultRetryInterval       = 3 * time.Second
	DefaultConnectPollInterval = 5 * time.Second
	DefaultProcessPollInterval = 5 * time.Second
)

// DefaultPresencePollInterval is the cadence of Daemon's own presence-mode loop.
const DefaultPresencePollInterval = 3 * time.Second

// discordGate is a Gate that waits for Valorant and Discord to both be
// running before allowing a connect attempt. See ADR-0003.
type discordGate struct {
	checker ProcessChecker
	game    GameDetector
}

func (g *discordGate) Ready() (bool, error) {
	if !g.game.GameRunning() {
		return false, nil
	}
	return g.checker.IsRunning(discordProcessNames...)
}

// NewDiscordSupervisor builds the production Discord Connection Supervisor.
// game is typically the already-built RiotSupervisor.
func NewDiscordSupervisor(client *discord.Client, checker ProcessChecker, game GameDetector) *DiscordSupervisor {
	return newDiscordSupervisor(client, checker, game, DefaultRetryInterval, DefaultConnectPollInterval, DefaultProcessPollInterval,
		WithGate(&discordGate{checker: checker, game: game}))
}

// NewProductionRiotSupervisor builds the production Riot Connection Supervisor.
func NewProductionRiotSupervisor(source *RiotSource, checker ProcessChecker) *RiotSupervisor {
	return NewRiotSupervisor(source, checker, DefaultRetryInterval, DefaultConnectPollInterval, DefaultProcessPollInterval)
}
