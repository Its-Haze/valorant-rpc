package daemon

import (
	"github.com/rs/zerolog"

	"github.com/its-haze/valorant-rpc/internal/config"
	"github.com/its-haze/valorant-rpc/internal/content"
	"github.com/its-haze/valorant-rpc/internal/discord"
	"github.com/its-haze/valorant-rpc/internal/process"
	"github.com/its-haze/valorant-rpc/internal/riotchat"
	"github.com/its-haze/valorant-rpc/internal/riotclient"
	"github.com/its-haze/valorant-rpc/internal/state"
)

// Wire builds a fully connected Daemon from a config Store and logger. The
// GUI app and the headless launcher both call this so they run the same graph.
func Wire(store *config.Store, logger zerolog.Logger) *Daemon {
	stateMgr := state.NewManager(logger)
	discordClient := discord.NewClient(store, logger)
	updater := discord.NewUpdater(discordClient, store, logger)
	checker := process.NewChecker()
	catalogue := content.New(content.Options{Logger: logger})

	riotClient := riotclient.New(riotclient.Options{Logger: logger})
	source := NewRiotSource(riotClient, stateMgr, logger, func(onUpdate func(riotchat.Presence)) presenceWatcher {
		return riotchat.NewWatcher(riotClient, logger, onUpdate)
	}, WithLocaleReader(riotClient))

	riotSup := NewProductionRiotSupervisor(source, checker)
	discordSup := NewDiscordSupervisor(discordClient, checker, riotSup)

	return New(discordSup, riotSup, updater, stateMgr, logger,
		DefaultPresencePollInterval, DefaultPlaceholderInterval,
		WithCatalogue(catalogue))
}
