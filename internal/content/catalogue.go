// Package content resolves Valorant's UUIDs and internal identifiers to
// display names and hotlinked image URLs, from valorant-api.com.
package content

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"unicode"

	"github.com/its-haze/valorant-rpc/pkg/types"
)

// RadiantTier is the top competitive tier. Tiers 1 and 2 are "Unused"
// placeholders, so the ladder is 0 and 3 through 27 with gaps in between.
const RadiantTier = 27

// The four payloads a catalogue is built from. Every one asks for every
// locale, so switching language later costs no round trip.
const (
	agentsPath = "/agents?isPlayableCharacter=true&language=all"
	mapsPath   = "/maps?language=all"
	tiersPath  = "/competitivetiers?language=all"
	modesPath  = "/gamemodes?language=all"

	// Cards are looked up for art only, and their names cost roughly another
	// megabyte across every locale, so this one endpoint skips them.
	cardsPath = "/playercards"
)

// invalidDivision marks the "Unused" tier rows. They have no icon and no
// player holds them, so they are dropped rather than rendered.
const invalidDivision = "ECompetitiveDivision::INVALID"

// Agent is one playable character, localized.
type Agent struct {
	UUID           string
	Name           string
	Icon           string   // displayIcon, the full-size portrait head
	IconSmall      string   // displayIconSmall, for the small presence image
	GradientColors []string // backgroundGradientColors, carried for later use
}

// Map is one map entry. URL is Riot's own path, which is what the presence
// blob reports and what the lookup joins on.
type Map struct {
	UUID         string
	URL          string
	Name         string
	ListViewIcon string
	Splash       string
}

// Tier is one rung of the competitive ladder. Name carries the division and
// the number, cased for display as in "Iron 1".
type Tier struct {
	Tier      int
	Name      string
	LargeIcon string
}

// GameMode is one entry of /v1/gamemodes, keyed by its asset path. Its
// queueID is always null, which is why QueueName exists.
type GameMode struct {
	UUID      string
	Name      string
	AssetPath string
	Icon      string
}

// PlayerCard is the art a player has equipped beside their name. Only the
// images are carried; the catalogue never fetches card names.
type PlayerCard struct {
	UUID     string
	Icon     string // displayIcon, the square art Discord shows best
	WideArt  string
	LargeArt string
}

// localized is a displayName under ?language=all: one string per locale.
type localized map[string]string

// pick returns the requested locale, falling back to English and then to
// nothing. It never returns an arbitrary locale, because map order is random.
func (l localized) pick(locale string) string {
	if name, ok := l[locale]; ok && name != "" {
		return name
	}
	return l[types.DefaultLocale]
}

// Catalogue is one resolved snapshot of the four payloads. It is built once
// per refresh and never mutated, so readers need no lock of their own.
type Catalogue struct {
	agents    map[string]agentEntry // keyed by lowercased UUID
	agentDevs map[string]string     // lowercased developerName to UUID
	maps      map[string]mapEntry   // keyed by lowercased mapUrl
	tiers     map[int]tierEntry
	modes     map[string]modeEntry // keyed by lowercased assetPath
	cards     map[string]cardEntry // keyed by lowercased UUID
}

type agentEntry struct {
	UUID           string    `json:"uuid"`
	DeveloperName  string    `json:"developerName"`
	DisplayName    localized `json:"displayName"`
	DisplayIcon    string    `json:"displayIcon"`
	DisplayIconSml string    `json:"displayIconSmall"`
	GradientColors []string  `json:"backgroundGradientColors"`
}

type mapEntry struct {
	UUID         string    `json:"uuid"`
	DisplayName  localized `json:"displayName"`
	ListViewIcon string    `json:"listViewIcon"`
	Splash       string    `json:"splash"`
	MapURL       string    `json:"mapUrl"`
}

type tierTable struct {
	UUID  string      `json:"uuid"`
	Tiers []tierEntry `json:"tiers"`
}

type tierEntry struct {
	Tier      int       `json:"tier"`
	TierName  localized `json:"tierName"`
	Division  string    `json:"division"` // the raw ECompetitiveDivision enum
	LargeIcon string    `json:"largeIcon"`
}

type modeEntry struct {
	UUID        string    `json:"uuid"`
	DisplayName localized `json:"displayName"`
	DisplayIcon string    `json:"displayIcon"`
	AssetPath   string    `json:"assetPath"`
}

type cardEntry struct {
	UUID        string `json:"uuid"`
	DisplayIcon string `json:"displayIcon"`
	WideArt     string `json:"wideArt"`
	LargeArt    string `json:"largeArt"`
}

// envelope is valorant-api's uniform {status, data} wrapper. The HTTP status
// already gates the read, so only the payload is kept.
type envelope[T any] struct {
	Data T `json:"data"`
}

// Empty reports a catalogue nothing has been loaded into yet.
func (c *Catalogue) Empty() bool {
	return c == nil || (len(c.agents) == 0 && len(c.maps) == 0 && len(c.tiers) == 0 &&
		len(c.modes) == 0 && len(c.cards) == 0)
}

// Agent resolves an agent UUID. glz returns them uppercase and valorant-api
// returns them lowercase, so the join folds case.
func (c *Catalogue) Agent(uuid, locale string) (Agent, bool) {
	if c == nil {
		return Agent{}, false
	}
	entry, ok := c.agents[foldKey(uuid)]
	if !ok {
		return Agent{}, false
	}
	return Agent{
		UUID:           entry.UUID,
		Name:           entry.DisplayName.pick(locale),
		Icon:           entry.DisplayIcon,
		IconSmall:      entry.DisplayIconSml,
		GradientColors: slices.Clone(entry.GradientColors),
	}, true
}

// AgentUUIDByDeveloperName resolves Riot's internal codename for an agent,
// which is what the game log names, to the UUID the catalogue is keyed by.
// A codename nothing matches is not an agent: the game logs its UI shells
// with the same line.
func (c *Catalogue) AgentUUIDByDeveloperName(name string) (string, bool) {
	if c == nil {
		return "", false
	}
	uuid, ok := c.agentDevs[foldKey(name)]
	return uuid, ok
}

// Map resolves the presence blob's matchMap, which is Riot's own map path.
func (c *Catalogue) Map(mapURL, locale string) (Map, bool) {
	if c == nil {
		return Map{}, false
	}
	entry, ok := c.maps[foldKey(mapURL)]
	if !ok {
		return Map{}, false
	}
	return Map{
		UUID:         entry.UUID,
		URL:          entry.MapURL,
		Name:         entry.DisplayName.pick(locale),
		ListViewIcon: entry.ListViewIcon,
		Splash:       entry.Splash,
	}, true
}

// Tier resolves a numeric competitive tier. The "Unused" rows never resolve.
func (c *Catalogue) Tier(tier int, locale string) (Tier, bool) {
	if c == nil {
		return Tier{}, false
	}
	entry, ok := c.tiers[tier]
	if !ok {
		return Tier{}, false
	}
	return Tier{
		Tier:      entry.Tier,
		Name:      tierDisplayName(entry.TierName.pick(locale)),
		LargeIcon: entry.LargeIcon,
	}, true
}

// tierDisplayName softens valorant-api's shouted tier names, "GOLD 2" into
// "Gold 2". A name that already carries lower case is left exactly as it is.
func tierDisplayName(name string) string {
	if name != strings.ToUpper(name) {
		return name
	}
	words := strings.Fields(name)
	for i, w := range words {
		runes := []rune(strings.ToLower(w))
		runes[0] = unicode.ToUpper(runes[0])
		words[i] = string(runes)
	}
	return strings.Join(words, " ")
}

// GameMode resolves a game mode by its asset path.
func (c *Catalogue) GameMode(assetPath, locale string) (GameMode, bool) {
	if c == nil {
		return GameMode{}, false
	}
	entry, ok := c.modes[foldKey(assetPath)]
	if !ok {
		return GameMode{}, false
	}
	return GameMode{
		UUID:      entry.UUID,
		Name:      entry.DisplayName.pick(locale),
		AssetPath: entry.AssetPath,
		Icon:      entry.DisplayIcon,
	}, true
}

// PlayerCard resolves the equipped card's art. It takes no locale: the
// catalogue holds no card names to localize.
func (c *Catalogue) PlayerCard(uuid string) (PlayerCard, bool) {
	if c == nil {
		return PlayerCard{}, false
	}
	entry, ok := c.cards[foldKey(uuid)]
	if !ok {
		return PlayerCard{}, false
	}
	return PlayerCard{
		UUID:     entry.UUID,
		Icon:     entry.DisplayIcon,
		WideArt:  entry.WideArt,
		LargeArt: entry.LargeArt,
	}, true
}

func foldKey(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

// parseCatalogue builds one snapshot from the four payloads. All four have
// to parse, so a half-loaded catalogue is never published.
func parseCatalogue(agents, maps, tiers, modes, cards []byte) (*Catalogue, error) {
	agentList, err := decode[[]agentEntry](agents, agentsPath)
	if err != nil {
		return nil, err
	}
	mapList, err := decode[[]mapEntry](maps, mapsPath)
	if err != nil {
		return nil, err
	}
	tierTables, err := decode[[]tierTable](tiers, tiersPath)
	if err != nil {
		return nil, err
	}
	modeList, err := decode[[]modeEntry](modes, modesPath)
	if err != nil {
		return nil, err
	}
	cardList, err := decode[[]cardEntry](cards, cardsPath)
	if err != nil {
		return nil, err
	}

	cat := &Catalogue{
		agents: make(map[string]agentEntry, len(agentList)),
		maps:   make(map[string]mapEntry, len(mapList)),
		tiers:  make(map[int]tierEntry),
		modes:  make(map[string]modeEntry, len(modeList)),
		cards:  make(map[string]cardEntry, len(cardList)),

		agentDevs: make(map[string]string, len(agentList)),
	}
	for _, a := range agentList {
		cat.agents[foldKey(a.UUID)] = a
		if a.DeveloperName != "" {
			cat.agentDevs[foldKey(a.DeveloperName)] = a.UUID
		}
	}
	for _, m := range mapList {
		if m.MapURL == "" {
			continue
		}
		cat.maps[foldKey(m.MapURL)] = m
	}
	for _, g := range modeList {
		cat.modes[foldKey(g.AssetPath)] = g
	}
	for _, c := range cardList {
		cat.cards[foldKey(c.UUID)] = c
	}

	// Riot ships one table per episode and only the last one is current.
	if len(tierTables) > 0 {
		for _, t := range tierTables[len(tierTables)-1].Tiers {
			if t.Division == invalidDivision {
				continue
			}
			cat.tiers[t.Tier] = t
		}
	}

	return cat, nil
}

func decode[T any](blob []byte, path string) (T, error) {
	var env envelope[T]
	if err := json.Unmarshal(blob, &env); err != nil {
		return env.Data, fmt.Errorf("content: decoding %s: %w", path, err)
	}
	return env.Data, nil
}

// Agents returns every playable agent, ordered by UUID so a caller sampling
// the list gets the same entries on every run.
func (c *Catalogue) Agents(locale string) []Agent {
	if c == nil {
		return nil
	}
	out := make([]Agent, 0, len(c.agents))
	for _, key := range sortedKeys(c.agents) {
		agent, _ := c.Agent(key, locale)
		out = append(out, agent)
	}
	return out
}

// Maps returns every map entry, ordered by Riot's own map path.
func (c *Catalogue) Maps(locale string) []Map {
	if c == nil {
		return nil
	}
	out := make([]Map, 0, len(c.maps))
	for _, key := range sortedKeys(c.maps) {
		world, _ := c.Map(key, locale)
		out = append(out, world)
	}
	return out
}

// Tiers returns every competitive tier that resolves, in ladder order. The
// "Unused" rows are absent, the same as they are from Tier.
func (c *Catalogue) Tiers(locale string) []Tier {
	if c == nil {
		return nil
	}
	numbers := make([]int, 0, len(c.tiers))
	for number := range c.tiers {
		numbers = append(numbers, number)
	}
	slices.Sort(numbers)

	out := make([]Tier, 0, len(numbers))
	for _, number := range numbers {
		tier, _ := c.Tier(number, locale)
		out = append(out, tier)
	}
	return out
}

// PlayerCards returns every card, ordered by UUID.
func (c *Catalogue) PlayerCards() []PlayerCard {
	if c == nil {
		return nil
	}
	out := make([]PlayerCard, 0, len(c.cards))
	for _, key := range sortedKeys(c.cards) {
		card, _ := c.PlayerCard(key)
		out = append(out, card)
	}
	return out
}

// sortedKeys keeps every enumeration deterministic, because Go randomizes
// map order and a sampled asset check has to be reproducible.
func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
}
