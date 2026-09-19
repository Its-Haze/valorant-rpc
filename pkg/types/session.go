// Package types holds the vocabulary the daemon, the state model and the
// presence builders share.
package types

// SessionLoopState is Riot's coarse phase, straight from the presence blob.
// Unknown values are expected: Riot adds states without telling anyone.
type SessionLoopState string

const (
	SessionLoopMenus   SessionLoopState = "MENUS"
	SessionLoopPregame SessionLoopState = "PREGAME"
	SessionLoopInGame  SessionLoopState = "INGAME"
)

// PartyState subdivides MENUS. It carries values outside this set while the
// player is in a match, which is why only MENUS ever consults it.
type PartyState string

const (
	PartyDefault         PartyState = "DEFAULT"
	PartyMatchmaking     PartyState = "MATCHMAKING"
	PartyCustomGameSetup PartyState = "CUSTOM_GAME_SETUP"
)

// Provisioning flows worth naming. The range is a variant of being in a
// match, so it is a predicate on the state rather than a context.
const (
	// Score types. Points is deathmatch, where the ally score is the player's
	// own kills rather than a team's rounds.
	ScoreTypeRounds = "Rounds"
	ScoreTypePoints = "Points"
)

const (
	ProvisioningFlowMatchmaking   = "Matchmaking"
	ProvisioningFlowCustomGame    = "CustomGame"
	ProvisioningFlowShootingRange = "ShootingRange"
)

// PresenceContext names one of the five phases a presence is built for. The
// values are the template context keys, and the frontend duplicates them.
type PresenceContext string

const (
	ContextInClient    PresenceContext = "in-client"
	ContextInQueue     PresenceContext = "in-queue"
	ContextCustomGame  PresenceContext = "custom-game"
	ContextAgentSelect PresenceContext = "agent-select"
	ContextInMatch     PresenceContext = "in-match"
)

// QueueID is Riot's queue string, e.g. "competitive" or "hurm". There is no
// published list, so an unrecognized one has to render rather than break.
type QueueID string

// Availability is the player's chat status.
type Availability string

const (
	AvailabilityOnline Availability = "Online"
	AvailabilityAway   Availability = "Away"
	AvailabilityDND    Availability = "dnd"
)
