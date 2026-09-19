package template

import "maps"

// The five presence contexts. These strings are the config keys for a user's
// templates and are duplicated in frontend/src/lib/presenceContexts.ts.
const (
	ContextInClient    Context = "in-client"
	ContextInQueue     Context = "in-queue"
	ContextCustomGame  Context = "custom-game"
	ContextAgentSelect Context = "agent-select"
	ContextInMatch     Context = "in-match"
)

// mid is the separator the defaults join on. It is in the engine's separator
// set, so a token collapsing to empty takes its neighbouring middot with it.
const mid = " · "

// Tokens every context carries: who the player is, and the idle flag, which
// cuts across phase rather than belonging to one.
var commonTokens = []string{"riot_id", "account_level", "rank", "idle"}

// partyTokens carry the lobby size. party is the pre-joined "2/5", the same
// convenience score is for the two round counts; the halves stay available.
var partyTokens = []string{"party", "party_size", "max_party_size"}

func tokensFor(extra ...string) []string {
	out := append([]string{}, extra...)
	return append(out, commonTokens...)
}

func init() {
	knownTokens[ContextInClient] = tokensFor()
	knownTokens[ContextInQueue] = tokensFor(append([]string{"mode"}, partyTokens...)...)
	knownTokens[ContextCustomGame] = tokensFor(append([]string{"map", "mode"}, partyTokens...)...)
	knownTokens[ContextAgentSelect] = tokensFor(append([]string{"map", "mode", "agent"}, partyTokens...)...)
	knownTokens[ContextInMatch] = tokensFor(append([]string{
		"map", "mode", "agent", "score", "score_ally", "score_enemy",
	}, partyTokens...)...)

	// Each default anchors its state line on a literal, so a presence still
	// reads as something when every token in it is empty.
	defaults[ContextInClient] = [2]string{"In the client", "{rank}" + mid + "{idle}"}
	defaults[ContextInQueue] = [2]string{"{mode}", "In queue" + mid + "{party}" + mid + "{idle}"}
	defaults[ContextCustomGame] = [2]string{"{map}", "Custom game" + mid + "{party}" + mid + "{idle}"}
	defaults[ContextAgentSelect] = [2]string{"{mode}" + mid + "{map}", "Agent select" + mid + "{agent}" + mid + "{party}"}
	defaults[ContextInMatch] = [2]string{"{mode}" + mid + "{map}", "In a match" + mid + "{score}" + mid + "{agent}"}

	// Sample values for the settings-screen preview. agent is left out on
	// purpose: v0.1 never resolves one, and a preview should not promise it.
	sampleData[ContextInClient] = sample(nil)
	sampleData[ContextInQueue] = sample(map[string]string{
		"mode": "Competitive", "party": "2/5", "party_size": "2", "max_party_size": "5",
	})
	sampleData[ContextCustomGame] = sample(map[string]string{
		"map": "Ascent", "mode": "Custom", "party": "5/10", "party_size": "5", "max_party_size": "10",
	})
	sampleData[ContextAgentSelect] = sample(map[string]string{
		"map": "Ascent", "mode": "Competitive", "party": "2/5", "party_size": "2", "max_party_size": "5",
	})
	sampleData[ContextInMatch] = sample(map[string]string{
		"map": "Ascent", "mode": "Competitive", "party": "2/5", "party_size": "2", "max_party_size": "5",
		"score": "7-5", "score_ally": "7", "score_enemy": "5",
	})

	order = append(order, ContextInClient, ContextInQueue, ContextCustomGame, ContextAgentSelect, ContextInMatch)
}

// sample merges the per-context values over the ones every context shares.
func sample(extra map[string]string) map[string]string {
	out := map[string]string{
		"riot_id":       "Haze",
		"account_level": "312",
		"rank":          "Immortal 2",
		"idle":          "Idle",
	}
	maps.Copy(out, extra)
	return out
}
