package daemon

import (
	"github.com/its-haze/valorant-rpc/internal/riotchat"
	"github.com/its-haze/valorant-rpc/internal/state"
	"github.com/its-haze/valorant-rpc/pkg/types"
)

// applyPresence copies a decoded presence onto the state, zero values and
// all: a presence is a full snapshot of what Riot publishes, not a patch.
func applyPresence(p riotchat.Presence) func(*state.State) {
	return func(st *state.State) {
		if p.GameName != "" {
			st.RiotID = p.GameName
		}
		if p.Tagline != "" {
			st.Tagline = p.Tagline
		}
		st.AccountLevel = p.AccountLevel
		if p.PlayerCardID != "" {
			st.PlayerCardID = p.PlayerCardID
		}

		st.SessionLoopState = types.SessionLoopState(p.SessionLoopState)
		st.PartyState = types.PartyState(p.PartyState)
		st.ProvisioningFlow = p.ProvisioningFlow
		st.IsIdle = p.IsIdle

		st.MapID = p.MatchMap
		st.QueueID = types.QueueID(p.QueueID)
		st.ScoreAlly = p.ScoreAllyTeam
		st.ScoreEnemy = p.ScoreEnemyTeam

		st.PartySize = p.PartySize
		st.MaxPartySize = p.MaxPartySize
		st.PartyAccessibility = p.PartyAccessibility
		st.QueueEntryTime = p.QueueEntryTime

		st.CompetitiveTier = p.CompetitiveTier
		st.LeaderboardPosition = p.LeaderboardPosition

		st.Availability = availabilityFor(p.IsIdle)
	}
}

// availabilityFor derives the chat status from the away flag in the private
// blob. The entry's own availability field is not decoded.
func availabilityFor(idle bool) types.Availability {
	if idle {
		return types.AvailabilityAway
	}
	return types.AvailabilityOnline
}
