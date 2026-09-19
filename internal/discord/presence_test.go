package discord

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"

	"github.com/its-haze/valorant-rpc/internal/config"
	"github.com/its-haze/valorant-rpc/internal/content"
	"github.com/its-haze/valorant-rpc/internal/state"
	"github.com/its-haze/valorant-rpc/pkg/constants"
	"github.com/its-haze/valorant-rpc/pkg/types"
)

const (
	jettUUID  = "add6443a-41bd-e414-f6ad-e58d267f4e95"
	ascentURL = "/Game/Maps/Ascent/Ascent"
	rangeURL  = "/Game/Maps/Poveglia/Range"
	cardUUID  = "1711d20d-4b1c-c64a-14be-d4ae58a457c6"
)

// fixtureDoer serves the content package's committed payloads, so these
// tests join against the same data a real refresh would load.
type fixtureDoer struct{ t *testing.T }

func (d fixtureDoer) Do(req *http.Request) (*http.Response, error) {
	names := map[string]string{
		"/agents": "agents.json", "/maps": "maps.json",
		"/competitivetiers": "competitivetiers.json", "/gamemodes": "gamemodes.json",
		"/playercards": "playercards.json",
	}

	name, ok := names[req.URL.Path[strings.LastIndex(req.URL.Path, "/"):]]
	if !ok {
		d.t.Fatalf("unexpected catalogue request %s", req.URL.Path)
	}
	blob, err := os.ReadFile(filepath.Join("..", "content", "testdata", name))
	if err != nil {
		d.t.Fatalf("reading %s: %v", name, err)
	}
	return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(blob))}, nil
}

func testCatalogue(t *testing.T) *content.Catalogue {
	t.Helper()

	cache := content.New(content.Options{Doer: fixtureDoer{t}, Logger: zerolog.Nop()})
	if err := cache.Refresh(context.Background()); err != nil {
		t.Fatalf("loading the fixture catalogue: %v", err)
	}
	return cache.Snapshot()
}

func presenceConfig() *config.Config { return config.DefaultConfig() }

// inClientState is a player sitting in the menus with everything populated,
// which each test then narrows to the case it cares about.
func inClientState() *state.State {
	st := state.NewState()
	st.RiotID, st.Tagline = "Haze", "EUW"
	st.AccountLevel = 312
	st.PlayerCardID = cardUUID
	st.CompetitiveTier = 21
	st.SessionLoopState = types.SessionLoopMenus
	st.PartyState = types.PartyDefault
	st.PartySize, st.MaxPartySize = 2, 5
	st.ContextEnteredAt = time.Unix(1700000000, 0)
	return st
}

func inMatchState() *state.State {
	st := inClientState()
	st.SessionLoopState = types.SessionLoopInGame
	st.QueueID = "competitive"
	st.MapID = ascentURL
	st.ScoreAlly, st.ScoreEnemy = 7, 5
	st.ProvisioningFlow = types.ProvisioningFlowMatchmaking
	return st
}

// everyContextState returns one state per presence context, so a check that
// must hold everywhere does not have to rebuild them.
func everyContextState() map[types.PresenceContext]*state.State {
	queue := inClientState()
	queue.PartyState = types.PartyMatchmaking
	queue.QueueID = "competitive"

	custom := inClientState()
	custom.PartyState = types.PartyCustomGameSetup
	custom.MapID = ascentURL

	pregame := inClientState()
	pregame.SessionLoopState = types.SessionLoopPregame
	pregame.MapID = ascentURL
	pregame.QueueID = "competitive"

	return map[types.PresenceContext]*state.State{
		types.ContextInClient:    inClientState(),
		types.ContextInQueue:     queue,
		types.ContextCustomGame:  custom,
		types.ContextAgentSelect: pregame,
		types.ContextInMatch:     inMatchState(),
	}
}

func TestEveryContextBuildsAPresence(t *testing.T) {
	cat := testCatalogue(t)
	cfg := presenceConfig()

	for ctx, st := range everyContextState() {
		if got := st.PhaseContext(); got != ctx {
			t.Fatalf("state for %q reports context %q", ctx, got)
		}

		rpc := MapStateToPresence(st, cfg, cat)
		if rpc.Details == "" && rpc.State == "" {
			t.Errorf("%q built a blank presence", ctx)
		}
		if rpc.LargeImage == "" || rpc.SmallImage == "" {
			t.Errorf("%q is missing art: %+v", ctx, rpc)
		}
		// The competitive contexts hand the hover text to the rank instead.
		if rpc.SmallText == "" {
			t.Errorf("%q has no small text", ctx)
		}
		if rpc.Start != st.ContextEnteredAt.Unix() {
			t.Errorf("%q timer = %d, want the context entry %d", ctx, rpc.Start, st.ContextEnteredAt.Unix())
		}
		for _, line := range []string{rpc.Details, rpc.State} {
			if strings.Contains(line, "{") {
				t.Errorf("%q left a token unsubstituted: %q", ctx, line)
			}
		}
	}
}

// The lobby contexts show the card the game itself shows beside the player's
// name. In a match the agent takes over.
func TestLobbyContextsShowThePlayerCard(t *testing.T) {
	cat := testCatalogue(t)
	card, ok := cat.PlayerCard(cardUUID)
	if !ok {
		t.Fatal("the fixture card did not resolve")
	}

	rpc := MapStateToPresence(inClientState(), presenceConfig(), cat)
	if rpc.LargeImage != card.Icon {
		t.Errorf("large image = %q, want the player card %q", rpc.LargeImage, card.Icon)
	}
	if rpc.LargeText != "Haze#EUW" {
		t.Errorf("large text = %q, want the Riot ID", rpc.LargeText)
	}
}

func TestInMatchShowsTheAgentWhenOneIsKnown(t *testing.T) {
	cat := testCatalogue(t)
	st := inMatchState()
	st.AgentID = strings.ToUpper(jettUUID) // glz hands these back uppercase

	rpc := MapStateToPresence(st, presenceConfig(), cat)

	agent, _ := cat.Agent(jettUUID, types.DefaultLocale)
	if rpc.LargeImage != agent.Icon {
		t.Errorf("large image = %q, want Jett's icon", rpc.LargeImage)
	}
	if rpc.LargeText != "Jett" {
		t.Errorf("large text = %q, want Jett", rpc.LargeText)
	}
	// The art and its hover name the agent, so the state line must not.
	if strings.Contains(rpc.State, "Jett") {
		t.Errorf("state = %q, want the agent left to the art", rpc.State)
	}
}

// If Riot renames the log line the agent stops resolving. The match presence
// then falls back to the player card, not to the map and not to nothing.
func TestInMatchFallsBackToTheCardWithoutAnAgent(t *testing.T) {
	cat := testCatalogue(t)

	rpc := MapStateToPresence(inMatchState(), presenceConfig(), cat)

	world, _ := cat.Map(ascentURL, types.DefaultLocale)
	if rpc.LargeImage == world.Splash {
		t.Error("large image fell back to the map splash")
	}
	card, _ := cat.PlayerCard(inMatchState().PlayerCardID)
	if rpc.LargeImage != card.Icon {
		t.Errorf("large image = %q, want the player card", rpc.LargeImage)
	}
	if rpc.LargeText != "Haze#EUW" {
		t.Errorf("large text = %q, want the riot id", rpc.LargeText)
	}
}

// The setting keeps the card even when the agent resolved fine.
func TestInMatchHonoursTheCardSetting(t *testing.T) {
	cat := testCatalogue(t)

	st := inMatchState()
	st.AgentID = jettUUID

	cfg := presenceConfig()
	cfg.Display.Default.MatchImage = config.MatchImageCard

	rpc := MapStateToPresence(st, cfg, cat)

	agent, _ := cat.Agent(jettUUID, types.DefaultLocale)
	if rpc.LargeImage == agent.Icon {
		t.Error("large image used the agent despite the card setting")
	}
	card, _ := cat.PlayerCard(st.PlayerCardID)
	if rpc.LargeImage != card.Icon {
		t.Errorf("large image = %q, want the player card", rpc.LargeImage)
	}
}

func TestRankEmblemOnlyInACompetitiveGame(t *testing.T) {
	cat := testCatalogue(t)
	tier, ok := cat.Tier(21, types.DefaultLocale)
	if !ok {
		t.Fatal("tier 21 did not resolve")
	}

	ranked := MapStateToPresence(inMatchState(), presenceConfig(), cat)
	if ranked.SmallImage != tier.LargeIcon {
		t.Errorf("competitive small image = %q, want the rank emblem", ranked.SmallImage)
	}

	casual := inMatchState()
	casual.QueueID = "swiftplay"
	if got := MapStateToPresence(casual, presenceConfig(), cat).SmallImage; got != valorantLogoBorderlessURL {
		t.Errorf("swiftplay small image = %q, want the borderless mark", got)
	}
}

func TestRankEmblemHonoursTheShowRankToggle(t *testing.T) {
	cat := testCatalogue(t)
	cfg := presenceConfig()
	cfg.Display.Default.ShowRank = false

	rpc := MapStateToPresence(inMatchState(), cfg, cat)
	if rpc.SmallImage != valorantLogoBorderlessURL {
		t.Errorf("small image = %q, want the borderless mark with rank hidden", rpc.SmallImage)
	}
	if strings.Contains(rpc.State, "Immortal") || strings.Contains(rpc.Details, "Immortal") {
		t.Errorf("rank leaked into the text with the toggle off: %+v", rpc)
	}
}

// Riot reports 0-0 through the whole of the menus, so a zero score is not a
// score and must not render.
func TestScoreSuppressedUntilARoundIsWon(t *testing.T) {
	cat := testCatalogue(t)

	fresh := inMatchState()
	fresh.ScoreAlly, fresh.ScoreEnemy = 0, 0
	if got := MapStateToPresence(fresh, presenceConfig(), cat); strings.Contains(got.State, "0-0") {
		t.Errorf("state = %q, want no score at 0-0", got.State)
	}

	played := MapStateToPresence(inMatchState(), presenceConfig(), cat)
	if !strings.Contains(played.State, "7-5") {
		t.Errorf("state = %q, want the 7-5 score", played.State)
	}
}

func TestScoreHonoursTheShowStatsToggle(t *testing.T) {
	cat := testCatalogue(t)
	cfg := presenceConfig()
	cfg.Display.Default.ShowStats = false

	if got := MapStateToPresence(inMatchState(), cfg, cat); strings.Contains(got.State, "7-5") {
		t.Errorf("state = %q, want no score with stats hidden", got.State)
	}
}

// The range is a variant of being in a match, not a sixth context.
func TestTheRangeIsAnInMatchVariant(t *testing.T) {
	cat := testCatalogue(t)
	st := inMatchState()
	st.MapID = rangeURL
	st.ProvisioningFlow = types.ProvisioningFlowShootingRange
	st.QueueID = ""

	if got := st.PhaseContext(); got != types.ContextInMatch {
		t.Fatalf("the range reports context %q, want in-match", got)
	}

	rpc := MapStateToPresence(st, presenceConfig(), cat)
	if !strings.Contains(rpc.Details, rangeLabel) {
		t.Errorf("details = %q, want it to name %q", rpc.Details, rangeLabel)
	}
	if strings.Contains(rpc.State, "7-5") {
		t.Errorf("state = %q, the range has no round score", rpc.State)
	}
}

func TestIdleRendersInEveryContext(t *testing.T) {
	cat := testCatalogue(t)

	for _, st := range []*state.State{inClientState(), inMatchState()} {
		st.IsIdle = true
		cfg := presenceConfig()
		// Put the token somewhere guaranteed for this check, since not every
		// default carries {idle}.
		cfg.Presence.Templates[string(st.PhaseContext())] = config.TemplatePair{
			Details: "{idle}", State: "x",
		}

		if got := MapStateToPresence(st, cfg, cat); got.Details != "Idle" {
			t.Errorf("%q details = %q, want Idle", st.PhaseContext(), got.Details)
		}
	}
}

// The ticket asks for idle to swap the small image as well as render a
// token, in every context, so a glance at the icon says "not at the keyboard".
func TestIdleSwapsTheSmallImageInEveryContext(t *testing.T) {
	cat := testCatalogue(t)

	for ctx, st := range everyContextState() {
		if got := MapStateToPresence(st, presenceConfig(), cat).SmallImage; got == valorantLogoIdleURL {
			t.Fatalf("%q already shows the idle icon while active: %q", ctx, got)
		}

		st.IsIdle = true
		if got := MapStateToPresence(st, presenceConfig(), cat).SmallImage; got != valorantLogoIdleURL {
			t.Errorf("%q idle small image = %q, want the dimmed icon", ctx, got)
		}
	}
}

// Idle beats the rank emblem: the rank is already in the text, and being
// away is the fact that just changed.
func TestIdleBeatsTheRankEmblem(t *testing.T) {
	st := inMatchState() // competitive, tier 21, so the emblem would normally win
	st.IsIdle = true

	if got := MapStateToPresence(st, presenceConfig(), testCatalogue(t)).SmallImage; got != valorantLogoIdleURL {
		t.Errorf("small image = %q, want the idle icon to win over the rank emblem", got)
	}
}

func TestPartyRendersAsText(t *testing.T) {
	cat := testCatalogue(t)
	st := inClientState()
	st.PartyState = types.PartyMatchmaking
	st.QueueID = "competitive"

	// Parenthesised, matching league-rpc's "In Lobby (2/5)".
	if got := MapStateToPresence(st, presenceConfig(), cat); !strings.Contains(got.State, "(2/5)") {
		t.Errorf("state = %q, want the party size in parentheses", got.State)
	}

	// Riot publishes nothing worth rendering before a party exists.
	solo := inClientState()
	solo.PartyState = types.PartyMatchmaking
	solo.PartySize, solo.MaxPartySize = 0, 0
	if got := MapStateToPresence(solo, presenceConfig(), cat); strings.Contains(got.State, "/") {
		t.Errorf("state = %q, want no party text without a party", got.State)
	}
}

// Before the first catalogue fetch every lookup misses. Presence still has to
// render, with text and the app's own icon.
func TestPresenceRendersWithoutACatalogue(t *testing.T) {
	rpc := MapStateToPresence(inMatchState(), presenceConfig(), nil)

	if rpc.Details == "" && rpc.State == "" {
		t.Fatalf("a cold catalogue produced a blank presence: %+v", rpc)
	}
	if rpc.LargeImage != valorantLogoURL || rpc.SmallImage != valorantLogoBorderlessURL {
		t.Errorf("a cold catalogue should fall back to the app icon: %+v", rpc)
	}
}

func TestNilStateOrConfigBuildsNothing(t *testing.T) {
	if got := MapStateToPresence(nil, presenceConfig(), nil); !got.IsEmpty() {
		t.Errorf("nil state built %+v", got)
	}
	if got := MapStateToPresence(inClientState(), nil, nil); !got.IsEmpty() {
		t.Errorf("nil config built %+v", got)
	}
}

func TestShouldClearPresenceFollowsTheInClientToggle(t *testing.T) {
	cfg := presenceConfig()
	cfg.Presence.ShowInClient = false

	if !ShouldClearPresence(inClientState(), cfg) {
		t.Error("in-client presence not cleared with the toggle off")
	}
	if ShouldClearPresence(inMatchState(), cfg) {
		t.Error("the toggle cleared a match presence")
	}

	cfg.Presence.ShowInClient = true
	if ShouldClearPresence(inClientState(), cfg) {
		t.Error("in-client presence cleared with the toggle on")
	}
}

// The user's own templates win over the defaults.
func TestUserTemplatesOverrideTheDefaults(t *testing.T) {
	cfg := presenceConfig()
	cfg.Presence.Templates[string(types.ContextInClient)] = config.TemplatePair{
		Details: "lvl {account_level}", State: "{riot_id}",
	}

	rpc := MapStateToPresence(inClientState(), cfg, testCatalogue(t))
	if rpc.Details != "lvl 312" {
		t.Errorf("details = %q, want the override", rpc.Details)
	}
	if rpc.State != "Haze#EUW" {
		t.Errorf("state = %q, want the override", rpc.State)
	}
}

// Guards the frontend's duplicated list: presenceContexts.ts must match.
func TestContextKeysAreStable(t *testing.T) {
	want := []string{"in-client", "in-queue", "custom-game", "agent-select", "in-match"}

	blob, err := json.Marshal(want)
	if err != nil {
		t.Fatal(err)
	}
	var keys []string
	if err := json.Unmarshal(blob, &keys); err != nil {
		t.Fatal(err)
	}

	for _, key := range keys {
		if _, ok := builders[types.PresenceContext(key)]; !ok {
			t.Errorf("no builder registered for %q", key)
		}
	}
	if len(builders) != len(want) {
		t.Errorf("%d builders registered, want %d", len(builders), len(want))
	}
}

// The rank emblem replaces the app icon only in a competitive queue, and it
// takes the hover text with it so the tier is readable somewhere.
func TestCompetitiveQueueSwapsTheIconForTheRankEmblem(t *testing.T) {
	cat := testCatalogue(t)
	cfg := presenceConfig()

	st := inClientState()
	st.QueueID = "competitive"

	rpc := MapStateToPresence(st, cfg, cat)
	if rpc.SmallImage == valorantLogoBorderlessURL {
		t.Error("competitive still shows the app icon")
	}
	if rpc.SmallText == constants.SmallText || rpc.SmallText == "" {
		t.Errorf("small text = %q, want the tier name", rpc.SmallText)
	}

	st.QueueID = "unrated"
	unrated := MapStateToPresence(st, cfg, cat)
	if unrated.SmallImage != valorantLogoBorderlessURL {
		t.Errorf("unrated small image = %q, want the app icon", unrated.SmallImage)
	}
	if unrated.SmallText != constants.SmallText {
		t.Errorf("unrated small text = %q, want the build tooltip", unrated.SmallText)
	}
}

// Idle outranks the emblem, and the credit line comes back with it.
func TestIdleKeepsTheDimmedIconEvenInCompetitive(t *testing.T) {
	st := inClientState()
	st.QueueID = "competitive"
	st.IsIdle = true

	rpc := MapStateToPresence(st, presenceConfig(), testCatalogue(t))
	if rpc.SmallImage != valorantLogoIdleURL {
		t.Errorf("small image = %q, want the idle mark", rpc.SmallImage)
	}
	if rpc.SmallText != constants.SmallText {
		t.Errorf("small text = %q, want the build tooltip", rpc.SmallText)
	}
	if !strings.HasSuffix(rpc.State, "Idle") {
		t.Errorf("state = %q, want it to end in Idle", rpc.State)
	}
}

// A custom game names itself as the mode and hides its map everywhere, in
// the text and in the art, through every phase it passes through.
func TestCustomGameNamesItselfAndHidesTheMap(t *testing.T) {
	cat := testCatalogue(t)
	cfg := presenceConfig()
	world, _ := cat.Map(ascentURL, types.DefaultLocale)

	lobby := inClientState()
	lobby.PartyState = types.PartyCustomGameSetup

	// Riot leaves partyState on CUSTOM_GAME_SETUP once the game starts, and
	// sets the flow, so each phase is reachable by its own marker.
	pregame := inMatchState()
	pregame.SessionLoopState = types.SessionLoopPregame
	pregame.ProvisioningFlow = types.ProvisioningFlowCustomGame

	match := inMatchState()
	match.ProvisioningFlow = types.ProvisioningFlowCustomGame

	for name, st := range map[string]*state.State{"lobby": lobby, "pregame": pregame, "match": match} {
		st.MapID = ascentURL
		st.QueueID = ""

		rpc := MapStateToPresence(st, cfg, cat)
		for _, line := range []string{rpc.Details, rpc.State, rpc.LargeText} {
			if strings.Contains(line, "Ascent") {
				t.Errorf("%s leaked the map: %q", name, line)
			}
		}
		if rpc.LargeImage == world.Splash {
			t.Errorf("%s used the map splash as art", name)
		}
		if !strings.Contains(rpc.Details, customLabel) {
			t.Errorf("%s details = %q, want the custom label", name, rpc.Details)
		}
	}
}

// The custom lobby keeps the queue that was selected before it, so the rank
// emblem has to be excluded by the custom marker rather than by the queue.
func TestCustomGameNeverShowsTheRankEmblem(t *testing.T) {
	cat := testCatalogue(t)
	cfg := presenceConfig()

	st := inClientState()
	st.QueueID = "competitive"
	st.PartyState = types.PartyCustomGameSetup

	rpc := MapStateToPresence(st, cfg, cat)
	if rpc.SmallImage != valorantLogoBorderlessURL {
		t.Errorf("small image = %q, want the app icon", rpc.SmallImage)
	}
	if rpc.SmallText != constants.SmallText {
		t.Errorf("small text = %q, want the build tooltip", rpc.SmallText)
	}
}

// Riot republishes the presence every 60 to 90 seconds, so a deathmatch kill
// count is stale far more often than not. The default line shows no score at
// all there rather than a number that is usually wrong.
func TestDeathmatchShowsNoScoreByDefault(t *testing.T) {
	cat := testCatalogue(t)
	cfg := presenceConfig()

	st := inMatchState()
	st.QueueID = "deathmatch"
	st.GameScoreType = types.ScoreTypePoints
	st.ScoreAlly, st.ScoreEnemy = 19, 39

	rpc := MapStateToPresence(st, cfg, cat)
	for _, unwanted := range []string{"19", "39", "kill"} {
		if strings.Contains(rpc.State, unwanted) {
			t.Errorf("state = %q, want no score in a deathmatch", rpc.State)
		}
	}
	if !strings.Contains(rpc.State, "In a match") {
		t.Errorf("state = %q, want the match line to survive", rpc.State)
	}
}

// The count is available once the setting is on, for anyone who wants it
// despite the lag.
func TestDeathmatchKillsTokenStaysAvailable(t *testing.T) {
	cat := testCatalogue(t)

	cfg := presenceConfig()
	cfg.Display.Default.ShowKills = true
	cfg.Presence.Templates["in-match"] = config.TemplatePair{Details: "{mode}", State: "{kills}"}

	st := inMatchState()
	st.QueueID = "deathmatch"
	st.GameScoreType = types.ScoreTypePoints
	st.ScoreAlly, st.ScoreEnemy = 19, 39

	if rpc := MapStateToPresence(st, cfg, cat); rpc.State != "19 kills" {
		t.Errorf("state = %q, want the opted-in kill count", rpc.State)
	}
}

func TestKillsTokenIsSingularForOneKill(t *testing.T) {
	cat := testCatalogue(t)

	cfg := presenceConfig()
	cfg.Display.Default.ShowKills = true
	cfg.Presence.Templates["in-match"] = config.TemplatePair{Details: "{mode}", State: "{kills}"}

	st := inMatchState()
	st.QueueID = "deathmatch"
	st.GameScoreType = types.ScoreTypePoints
	st.ScoreAlly, st.ScoreEnemy = 1, 12

	if rpc := MapStateToPresence(st, cfg, cat); rpc.State != "1 kill" {
		t.Errorf("state = %q, want a singular kill", rpc.State)
	}
}

// Round-based modes are unchanged: that score really is two teams.
func TestRoundScoreStillRendersBothTeams(t *testing.T) {
	st := inMatchState()
	st.GameScoreType = types.ScoreTypeRounds
	st.ScoreAlly, st.ScoreEnemy = 7, 5

	rpc := MapStateToPresence(st, presenceConfig(), testCatalogue(t))
	if !strings.Contains(rpc.State, "7-5") {
		t.Errorf("state = %q, want the round score", rpc.State)
	}
}

// Without the setting a deathmatch shows no count at all, whatever the
// template asks for.
func TestDeathmatchKillsStayHiddenWhileTheSettingIsOff(t *testing.T) {
	cat := testCatalogue(t)

	cfg := presenceConfig()
	cfg.Display.Default.ShowKills = false
	cfg.Presence.Templates["in-match"] = config.TemplatePair{Details: "{mode}", State: "In a match · {kills}"}

	st := inMatchState()
	st.QueueID = "deathmatch"
	st.GameScoreType = types.ScoreTypePoints
	st.ScoreAlly, st.ScoreEnemy = 19, 39

	rpc := MapStateToPresence(st, cfg, cat)
	if strings.Contains(rpc.State, "19") || strings.Contains(rpc.State, "kill") {
		t.Errorf("state = %q, want no count while the setting is off", rpc.State)
	}
	if !strings.Contains(rpc.State, "In a match") {
		t.Errorf("state = %q, want the rest of the line intact", rpc.State)
	}
}

// Round scores are unaffected by the kills setting.
func TestRoundScoreIgnoresTheKillsSetting(t *testing.T) {
	cfg := presenceConfig()
	cfg.Display.Default.ShowKills = false

	st := inMatchState()
	st.GameScoreType = types.ScoreTypeRounds
	st.ScoreAlly, st.ScoreEnemy = 7, 5

	if rpc := MapStateToPresence(st, cfg, testCatalogue(t)); !strings.Contains(rpc.State, "7-5") {
		t.Errorf("state = %q, want the round score", rpc.State)
	}
}

// Team Deathmatch and Escalation also score in points, but theirs is a team
// score. Reading it as personal kills would put someone else's work in this
// player's presence.
func TestTeamPointsModesAreNotReadAsKills(t *testing.T) {
	cat := testCatalogue(t)

	cfg := presenceConfig()
	cfg.Display.Default.ShowKills = true

	for _, queue := range []string{"hurm", "ggteam"} {
		st := inMatchState()
		st.QueueID = types.QueueID(queue)
		st.GameScoreType = types.ScoreTypePoints
		st.ScoreAlly, st.ScoreEnemy = 19, 31

		rpc := MapStateToPresence(st, cfg, cat)
		if strings.Contains(rpc.State, "kill") {
			t.Errorf("%s state = %q, want a team score not kills", queue, rpc.State)
		}
		if !strings.Contains(rpc.State, "19-31") {
			t.Errorf("%s state = %q, want both team scores", queue, rpc.State)
		}
	}
}
