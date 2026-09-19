package constants

import (
	"fmt"

	"github.com/its-haze/valorant-rpc/internal/version"
)

const (
	// AppName is the product name shown in the window, the tray and the installer.
	AppName = "Valorant RPC"

	// DiscordAppIDDefault is the "Valorant" Discord application. A custom ID can
	// replace it in settings; there are no presets.
	DiscordAppIDDefault = "1550850471885938749"

	// DefaultUpdateInterval throttles presence writes, in milliseconds.
	DefaultUpdateInterval = 1500

	// Process Names
	DiscordProcessName            = "Discord.exe"
	DiscordPTBProcessName         = "DiscordPTB.exe"
	DiscordCanaryProcessName      = "DiscordCanary.exe"
	DiscordDevelopmentProcessName = "DiscordDevelopment.exe"

	// Third-party Discord clients that serve the RPC pipe themselves, via arRPC.
	VesktopProcessName = "Vesktop.exe"
	LegcordProcessName = "Legcord.exe"
	ArmCordProcessName = "ArmCord.exe"

	// The game gate is both of these: the Riot Client hosts the local API the
	// app reads, and the game itself says whether a session is actually running.
	ValorantProcessName   = "VALORANT-Win64-Shipping.exe"
	RiotClientProcessName = "RiotClientServices.exe"
)

// SmallText is the small-icon hover tooltip shown on every presence state.
// Built at load time so the tooltip reports the running build, not a literal.
var SmallText = fmt.Sprintf("its-haze/valorant-rpc @Github.com (%s)", version.Version())
