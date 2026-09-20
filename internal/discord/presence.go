package discord

import (
	"fmt"

	"github.com/its-haze/valorant-rpc/internal/config"
	"github.com/its-haze/valorant-rpc/internal/content"
	"github.com/its-haze/valorant-rpc/internal/presence/template"
	"github.com/its-haze/valorant-rpc/internal/state"
	"github.com/its-haze/valorant-rpc/pkg/constants"
	"github.com/its-haze/valorant-rpc/pkg/types"
)

// rangeLabel is what the shooting range renders as. It is a variant of being
// in a match rather than a context, so the in-match builder swaps it in.
const rangeLabel = "The Range"

// customLabel is the mode a custom game renders as. Riot sends an empty
// queueId there, so without this the mode collapses out of every line.
const customLabel = "Custom game"

// competitiveQueue is the one queue whose rank emblem replaces the app icon.
const competitiveQueue = "competitive"

// deathmatchQueue is the only queue where the ally score belongs to the
// player alone. Team Deathmatch and Escalation also score in points, but
// theirs is the team's, so gameScoreType is not enough to tell them apart.
const deathmatchQueue = "deathmatch"

// The two words the availability token renders, mirroring league-rpc's own
// chat availability line.
const (
	availabilityOnline = "Online"
	availabilityAway   = "Away"
)

// builder renders one context. Every builder gets the same resolved view, so
// none of them reach back into the state or the catalogue on their own.
type builder func(v view) *RPCData

var builders = map[types.PresenceContext]builder{
	types.ContextInClient:    buildInClient,
	types.ContextInLobby:     buildInLobby,
	types.ContextInQueue:     buildInQueue,
	types.ContextCustomGame:  buildCustomGame,
	types.ContextAgentSelect: buildAgentSelect,
	types.ContextInMatch:     buildInMatch,
}

// view is everything a builder needs, resolved once: the tokens its template
// can name, and the art the catalogue could supply.
type view struct {
	st  *state.State
	cfg *config.Config

	tokens map[string]string

	card  content.PlayerCard
	agent content.Agent
	world content.Map
	tier  content.Tier

	hasCard  bool
	hasAgent bool
	hasMap   bool
	hasTier  bool
}

// MapStateToPresence routes state to the builder for its context. cat may be
func MapStateToPresence(st *state.State, cfg *config.Config, cat *content.Catalogue) *RPCData {
	if st == nil || cfg == nil {
		return &RPCData{}
	}

	ctx := st.PhaseContext()
	build, ok := builders[ctx]
	if !ok {
		build = buildInClient
	}
	return build(resolve(st, cfg, cat))
}

// ShouldClearPresence reports whether the show-in-client toggle is hiding
// this state. It covers the lobby too: both are sitting in the client.
func ShouldClearPresence(st *state.State, cfg *config.Config) bool {
	if cfg.Presence.ShowInClient {
		return false
	}
	ctx := st.PhaseContext()
	return ctx == types.ContextInClient || ctx == types.ContextInLobby
}

// resolve does every catalogue lookup and token decision once, so the six
// builders only choose a template context and which art to hang on it.
func resolve(st *state.State, cfg *config.Config, cat *content.Catalogue) view {
	// English everywhere: the rank was the only name a language picker ever
	// changed, so the setting was dropped rather than kept for one string.
	locale := types.DefaultLocale
	v := view{st: st, cfg: cfg, tokens: map[string]string{}}

	v.card, v.hasCard = cat.PlayerCard(st.PlayerCardID)
	v.agent, v.hasAgent = cat.Agent(st.AgentID, locale)
	v.world, v.hasMap = cat.Map(st.MapID, locale)
	v.tier, v.hasTier = cat.Tier(st.CompetitiveTier, locale)

	v.tokens["riot_id"] = riotID(st)
	if st.AccountLevel > 0 {
		v.tokens["account_level"] = fmt.Sprintf("%d", st.AccountLevel)
	}
	v.tokens["availability"] = availabilityOnline
	if st.IsIdle {
		v.tokens["availability"] = availabilityAway
		v.tokens["idle"] = "Idle"
	}
	// No rank in client: the tier follows the preselected queue, not anything
	// the player picked. It returns the moment they open a competitive lobby.
	if cfg.Display.Default.ShowRank && v.hasTier && st.PhaseContext() != types.ContextInClient {
		v.tokens["rank"] = v.tier.Name
	}
	if v.hasAgent {
		v.tokens["agent"] = v.agent.Name
	}

	v.tokens["mode"] = content.QueueName(string(st.QueueID))

	// A custom game names itself and hides its map, in every phase and in the
	// art as well as the text. Dropping hasMap covers both at once.
	if st.IsCustomGame() {
		v.tokens["mode"], v.hasMap = customLabel, false
	}
	if v.hasMap {
		v.tokens["map"] = v.world.Name
	}
	if st.IsRange() {
		v.tokens["map"] = rangeLabel
		v.tokens["mode"] = rangeLabel
	}

	addPartyTokens(v.tokens, st)
	// Only in a match. Riot keeps publishing the last match's score through
	// the menus, and the range has no rounds of its own either.
	if cfg.Display.Default.ShowStats && st.PhaseContext() == types.ContextInMatch && !st.IsRange() {
		addScoreTokens(v.tokens, st, cfg.Display.Default.ShowKills)
	}
	return v
}

// riotID is the player's name with their tag, or just the name when Riot has
// not published one yet.
func riotID(st *state.State) string {
	if st.RiotID != "" && st.Tagline != "" {
		return st.RiotID + "#" + st.Tagline
	}
	return st.RiotID
}

// addPartyTokens fills the party trio. A party of one is still a party, but
// a missing maximum means Riot published nothing worth rendering.
func addPartyTokens(tokens map[string]string, st *state.State) {
	if st.PartySize <= 0 || st.MaxPartySize <= 0 {
		return
	}
	tokens["party_size"] = fmt.Sprintf("%d", st.PartySize)
	tokens["max_party_size"] = fmt.Sprintf("%d", st.MaxPartySize)
	tokens["party"] = fmt.Sprintf("(%d/%d)", st.PartySize, st.MaxPartySize)
}

// addScoreTokens fills the score, 0-0 included. Callers gate it on being in
// a match, where a score of zero is a real score rather than a missing one.
func addScoreTokens(tokens map[string]string, st *state.State, showKills bool) {
	// Deathmatch is the one mode with no teams, so its ally score is the
	// player's own kills. It is off by default: Riot republishes the presence
	// every 60 to 90 seconds, measured, which is three updates across a whole
	// deathmatch, so the count is usually behind the scoreboard.
	if st.GameScoreType == types.ScoreTypePoints && string(st.QueueID) == deathmatchQueue {
		if !showKills {
			return
		}
		tokens["kills"] = pluralKills(st.ScoreAlly)
		tokens["score"] = pluralKills(st.ScoreAlly)
		tokens["score_ally"] = fmt.Sprintf("%d", st.ScoreAlly)
		tokens["score_enemy"] = fmt.Sprintf("%d", st.ScoreEnemy)
		return
	}
	tokens["score_ally"] = fmt.Sprintf("%d", st.ScoreAlly)
	tokens["score_enemy"] = fmt.Sprintf("%d", st.ScoreEnemy)
	tokens["score"] = fmt.Sprintf("%d-%d", st.ScoreAlly, st.ScoreEnemy)
}

// pluralKills spells the count out, because "1 kills" is the kind of detail
// people notice.
func pluralKills(kills int) string {
	if kills == 1 {
		return "1 kill"
	}
	return fmt.Sprintf("%d kills", kills)
}

func buildInClient(v view) *RPCData {
	return v.render(template.ContextInClient, v.cardImage(), v.tokens["riot_id"])
}

func buildInLobby(v view) *RPCData {
	return v.render(template.ContextInLobby, v.cardImage(), v.tokens["riot_id"])
}

func buildInQueue(v view) *RPCData {
	return v.render(template.ContextInQueue, v.cardImage(), v.tokens["riot_id"])
}

func buildCustomGame(v view) *RPCData {
	return v.render(template.ContextCustomGame, v.cardImage(), v.tokens["riot_id"])
}

func buildAgentSelect(v view) *RPCData {
	return v.render(template.ContextAgentSelect, v.cardImage(), v.tokens["riot_id"])
}

// buildInMatch shows the agent, which is the one context where the player is
// looking at something other than their own profile. It falls back to the
// card rather than to the map, so a Riot log change costs the agent art and
// nothing else. The setting picks the same fallback deliberately.
func buildInMatch(v view) *RPCData {
	image, text := v.cardImage(), v.tokens["riot_id"]
	switch {
	case v.hasAgent && v.wantsAgentArt():
		image, text = v.agent.Icon, v.agent.Name
	case v.st.IsRange():
		text = rangeLabel
	}
	return v.render(template.ContextInMatch, image, text)
}

// wantsAgentArt reports the agent setting. An unset value has already been
// repaired to the default by the config's clamp.
func (v view) wantsAgentArt() bool {
	return v.cfg.Display.Default.MatchImage != config.MatchImageCard
}

// cardImage is the player's equipped card, the art the game itself shows
// beside their name.
func (v view) cardImage() string {
	if v.hasCard && v.card.Icon != "" {
		return v.card.Icon
	}
	return valorantLogoURL
}

// smallIcon is the icon and its hover text: the dimmed mark while the player
// is idle, the rank emblem in a competitive queue, and the app's own mark
// everywhere else. Idle wins, because being away is the newer fact.
func (v view) smallIcon() (image, text string) {
	if v.st.IsIdle {
		return valorantLogoIdleURL, constants.SmallText
	}
	if v.cfg.Display.Default.ShowRank && v.hasTier && v.isRanked() {
		return v.tier.LargeIcon, v.tier.Name
	}
	return valorantLogoBorderlessURL, constants.SmallText
}

// isRanked reports a competitive queue the player actually chose. In client
// and in a custom game the queue is left over, so the emblem is not theirs.
func (v view) isRanked() bool {
	if v.st.PhaseContext() == types.ContextInClient {
		return false
	}
	return string(v.st.QueueID) == competitiveQueue && !v.st.IsCustomGame()
}

// render applies the user's templates for ctx and hangs the chosen art on
// the result. Every context is timed from when it was entered.
func (v view) render(ctx template.Context, largeImage, largeText string) *RPCData {
	pair := v.cfg.Presence.Templates[string(ctx)]
	details, stateLine, _ := template.RenderPair(ctx, pair.Details, pair.State, v.tokens)

	// Unlike league-rpc, the credit line is not moved up to the large art
	// when the emblem claims the hover: that art names the agent or the player.
	smallImage, smallText := v.smallIcon()

	return &RPCData{
		LargeImage: largeImage,
		LargeText:  largeText,
		SmallImage: smallImage,
		SmallText:  smallText,
		Details:    details,
		State:      stateLine,
		Start:      v.st.ContextEnteredAt.Unix(),
	}
}
