// Package riotchat decodes the Riot Client's chat presence into one
// normalized struct, across both payload shapes Riot is migrating between.
package riotchat

import "time"

// ProductValorant is the product tag on a Valorant presence entry. A friend
// on another Riot game publishes a differently shaped private blob.
const ProductValorant = "valorant"

const (
	// PresenceEvent is the websocket subscription carrying presence updates.
	PresenceEvent = "OnJsonApiEvent_chat_v4_presences"

	presencesEndpoint = "/chat/v4/presences"
	sessionEndpoint   = "/chat/v1/session"
)

// Presence is the local player's Valorant presence, normalized. Every field
// is best-effort: a shape Riot changes degrades to a zero value.
type Presence struct {
	PUUID    string
	GameName string
	Tagline  string

	// PlayerCardID is the card the player has equipped, which is the art
	// shown beside their name in the client.
	PlayerCardID string

	SessionLoopState string
	PartyState       string
	MatchMap         string
	QueueID          string
	ProvisioningFlow string

	ScoreAllyTeam  int
	ScoreEnemyTeam int

	PartySize          int
	MaxPartySize       int
	PartyAccessibility string
	PartyID            string

	CompetitiveTier     int
	LeaderboardPosition int
	AccountLevel        int

	IsIdle         bool
	QueueEntryTime time.Time
}
