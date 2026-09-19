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

// competitiveQueue is the one queue whose rank emblem replaces the app icon.
const competitiveQueue = "competitive"

// builder renders one context. Every builder gets the same resolved view, so
// none of them reach back into the state or the catalogue on their own.
type builder func(v view) *RPCData

var builders = map[types.PresenceContext]builder{
	types.ContextInClient:    buildInClient,
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

// ShouldClearPresence reports whether presence should be cleared rather than
// updated, which today is only the show-in-client toggle turned off.
func ShouldClearPresence(st *state.State, cfg *config.Config) bool {
	return !cfg.Presence.ShowInClient && st.PhaseContext() == types.ContextInClient
}

// resolve does every catalogue lookup and token decision once, so the five
// builders only choose a template context and which art to hang on it.
func resolve(st *state.State, cfg *config.Config, cat *content.Catalogue) view {
	locale := types.ResolveLocale(cfg.Display.Locale, st.ClientLocale)
	v := view{st: st, cfg: cfg, tokens: map[string]string{}}

	v.card, v.hasCard = cat.PlayerCard(st.PlayerCardID)
	v.agent, v.hasAgent = cat.Agent(st.AgentID, locale)
	v.world, v.hasMap = cat.Map(st.MapID, locale)
	v.tier, v.hasTier = cat.Tier(st.CompetitiveTier, locale)

	v.tokens["riot_id"] = riotID(st)
	if st.AccountLevel > 0 {
		v.tokens["account_level"] = fmt.Sprintf("%d", st.AccountLevel)
	}
	if st.IsIdle {
		v.tokens["idle"] = "Idle"
	}
	if cfg.Display.Default.ShowRank && v.hasTier {
		v.tokens["rank"] = v.tier.Name
	}
	if v.hasAgent {
		v.tokens["agent"] = v.agent.Name
	}

	v.tokens["mode"] = content.QueueName(string(st.QueueID))
	if v.hasMap {
		v.tokens["map"] = v.world.Name
	}
	if st.IsRange() {
		v.tokens["map"] = rangeLabel
		v.tokens["mode"] = rangeLabel
	}

	addPartyTokens(v.tokens, st)
	// The range has no rounds, so whatever the score fields still hold there
	// is left over from the last real match.
	if cfg.Display.Default.ShowStats && !st.IsRange() {
		addScoreTokens(v.tokens, st)
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
	tokens["party"] = fmt.Sprintf("%d/%d", st.PartySize, st.MaxPartySize)
}

// addScoreTokens fills the round score, but only once a round has been won.
// Riot reports 0-0 all through the menus, and that is not a score.
func addScoreTokens(tokens map[string]string, st *state.State) {
	if st.ScoreAlly <= 0 && st.ScoreEnemy <= 0 {
		return
	}
	tokens["score_ally"] = fmt.Sprintf("%d", st.ScoreAlly)
	tokens["score_enemy"] = fmt.Sprintf("%d", st.ScoreEnemy)
	tokens["score"] = fmt.Sprintf("%d-%d", st.ScoreAlly, st.ScoreEnemy)
}

func buildInClient(v view) *RPCData {
	return v.render(template.ContextInClient, v.cardImage(), v.tokens["riot_id"])
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
// looking at something other than their own profile.
func buildInMatch(v view) *RPCData {
	image, text := v.cardImage(), v.tokens["riot_id"]
	switch {
	case v.hasAgent:
		image, text = v.agent.Icon, v.agent.Name
	case v.st.IsRange():
		text = rangeLabel
	case v.hasMap:
		image, text = v.world.Splash, v.world.Name
	}
	return v.render(template.ContextInMatch, image, text)
}

// cardImage is the player's equipped card, the art the game itself shows
// beside their name.
func (v view) cardImage() string {
	if v.hasCard && v.card.Icon != "" {
		return v.card.Icon
	}
	return valorantLogoURL
}

// smallImage is the dimmed icon while the player is idle, the rank emblem in
// a ranked game, and the app's own icon everywhere else. Idle wins because
// the rank is already in the text and being away is the newer fact.
func (v view) smallImage() string {
	if v.st.IsIdle {
		return valorantLogoIdleURL
	}
	if v.cfg.Display.Default.ShowRank && v.hasTier && v.isRanked() {
		return v.tier.LargeIcon
	}
	return valorantLogoBorderlessURL
}

// isRanked reports the competitive queue, the only one with a rank worth
// showing next to the presence.
func (v view) isRanked() bool {
	return string(v.st.QueueID) == competitiveQueue
}

// render applies the user's templates for ctx and hangs the chosen art on
// the result. Every context is timed from when it was entered.
func (v view) render(ctx template.Context, largeImage, largeText string) *RPCData {
	pair := v.cfg.Presence.Templates[string(ctx)]
	details, stateLine, _ := template.RenderPair(ctx, pair.Details, pair.State, v.tokens)

	return &RPCData{
		LargeImage: largeImage,
		LargeText:  largeText,
		SmallImage: v.smallImage(),
		SmallText:  constants.SmallText,
		Details:    details,
		State:      stateLine,
		Start:      v.st.ContextEnteredAt.Unix(),
	}
}
