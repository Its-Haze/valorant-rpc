// Package state holds everything the presence builders read, and the manager
// that guards it.
package state

import (
	"strings"
	"time"

	"github.com/its-haze/valorant-rpc/pkg/types"
)

// State is the whole of what a presence is built from. Every field comes
// from the local presence blob, except AgentID, which waits for v0.2.
type State struct {
	// Player
	Availability types.Availability `json:"availability"`
	RiotID       string             `json:"riot_id"`
	Tagline      string             `json:"tagline"`
	AccountLevel int                `json:"account_level"`

	// Phase
	SessionLoopState types.SessionLoopState `json:"session_loop_state"`
	PartyState       types.PartyState       `json:"party_state"`
	ProvisioningFlow string                 `json:"provisioning_flow"`
	IsIdle           bool                   `json:"is_idle"`

	// Match
	MapID      string        `json:"map_id"` // Riot's map path, joined case-insensitively
	QueueID    types.QueueID `json:"queue_id"`
	AgentID    string        `json:"agent_id"` // v0.2; the local blob never carries it
	ScoreAlly  int           `json:"score_ally"`
	ScoreEnemy int           `json:"score_enemy"`

	// Party
	PartySize          int       `json:"party_size"`
	MaxPartySize       int       `json:"max_party_size"`
	PartyAccessibility string    `json:"party_accessibility"`
	QueueEntryTime     time.Time `json:"queue_entry_time"`

	// Rank
	CompetitiveTier     int `json:"competitive_tier"`
	LeaderboardPosition int `json:"leaderboard_position"`

	// ContextEnteredAt is when PhaseContext() last changed. No Riot payload
	// carries a match start time, so the elapsed timer is measured here.
	ContextEnteredAt time.Time `json:"context_entered_at"`
}

// NewState returns the state of a player who is online and nowhere in
// particular, which is what an unconnected daemon knows. The context is
// stamped here too: starting up already in the client is a context entry.
func NewState() *State {
	return &State{Availability: types.AvailabilityOnline, ContextEnteredAt: time.Now()}
}

// PhaseContext derives one of the five presence contexts. Only MENUS defers
// to the party state, and anything unrecognized degrades to the client.
func (s *State) PhaseContext() types.PresenceContext {
	loop := types.SessionLoopState(strings.ToUpper(string(s.SessionLoopState)))
	switch loop {
	case types.SessionLoopPregame:
		return types.ContextAgentSelect
	case types.SessionLoopInGame:
		return types.ContextInMatch
	}

	// A loop state Riot added since this was written is not MENUS, and the
	// party state keeps reporting MATCHMAKING through a whole match.
	if loop != types.SessionLoopMenus && loop != "" {
		return types.ContextInClient
	}

	switch types.PartyState(strings.ToUpper(string(s.PartyState))) {
	case types.PartyMatchmaking:
		return types.ContextInQueue
	case types.PartyCustomGameSetup:
		return types.ContextCustomGame
	}
	return types.ContextInClient
}

// IsRange reports the shooting range, which the in-match builder renders as
// a variant rather than a context of its own.
func (s *State) IsRange() bool {
	return strings.EqualFold(s.ProvisioningFlow, types.ProvisioningFlowShootingRange)
}

// Equals compares every field, so a field added later counts towards change
// detection without anyone remembering to extend this. Timestamps compare by
// instant: == on a time.Time also weighs its monotonic reading and location.
func (s *State) Equals(other *State) bool {
	if other == nil {
		return false
	}
	if !s.QueueEntryTime.Equal(other.QueueEntryTime) || !s.ContextEnteredAt.Equal(other.ContextEnteredAt) {
		return false
	}

	a, b := *s, *other
	a.QueueEntryTime, b.QueueEntryTime = time.Time{}, time.Time{}
	a.ContextEnteredAt, b.ContextEnteredAt = time.Time{}, time.Time{}
	return a == b
}

// Copy returns an independent state. Handing callers a copy is what lets
// Get() be read without a lock held.
func (s *State) Copy() *State {
	if s == nil {
		return nil
	}
	c := *s
	return &c
}
