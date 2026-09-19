package daemon

import "github.com/its-haze/valorant-rpc/pkg/constants"

// discordProcessNames decide whether Discord is running before an IPC
// connect. Forks count: arRPC serves the same pipe.
var discordProcessNames = []string{
	constants.DiscordProcessName,
	constants.DiscordPTBProcessName,
	constants.DiscordCanaryProcessName,
	constants.DiscordDevelopmentProcessName,
	constants.VesktopProcessName,
	constants.LegcordProcessName,
	constants.ArmCordProcessName,
}

// The game gate wants both of these running, not either. The Riot Client
// hosts the local API, and only the game process means a session exists.
var (
	riotClientProcessNames = []string{constants.RiotClientProcessName}
	valorantProcessNames   = []string{constants.ValorantProcessName}
)
