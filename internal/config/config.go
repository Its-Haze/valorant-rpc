package config

import (
	"errors"
	"fmt"

	"github.com/its-haze/valorant-rpc/internal/presence/template"
	"github.com/its-haze/valorant-rpc/pkg/constants"
	"github.com/its-haze/valorant-rpc/pkg/types"
)

// CurrentSchemaVersion is the version stamped on every config the app writes.
const CurrentSchemaVersion = 1

// Config is the versioned settings tree. It is persisted as JSON and edited
// through the GUI; nothing reads settings from CLI flags.
type Config struct {
	SchemaVersion      int    `json:"schema_version"`
	DiscordAppID       string `json:"discord_app_id"`
	Theme              string `json:"theme"`               // system | light | dark
	OnboardingComplete bool   `json:"onboarding_complete"` // first-run walkthrough finished

	Display  DisplayConfig  `json:"display"`
	Presence PresenceConfig `json:"presence"`
	Behavior BehaviorConfig `json:"behavior"`
	Advanced AdvancedConfig `json:"advanced"`
}

// DisplayConfig holds what presence shows.
type DisplayConfig struct {
	Default DisplayDefaults `json:"default"`

	// Locale picks the language agent, map and rank names render in, or
	// types.LocaleAuto to follow whatever the Riot Client is running in.
	Locale string `json:"locale"`
}

// DisplayDefaults is the global state for the display settings.
type DisplayDefaults struct {
	ShowRank  bool `json:"show_rank"`  // rank emblem and tier name
	ShowStats bool `json:"show_stats"` // match detail such as the round score

	// MatchImage picks the large art during a match: the agent being played,
	// or the player card kept from the menus. See MatchImageAgent.
	MatchImage string `json:"match_image"`
}

// PresenceConfig holds presence-wide text settings.
type PresenceConfig struct {
	ShowInClient bool                    `json:"show_in_client"` // presence while sitting in the client
	Templates    map[string]TemplatePair `json:"templates"`      // per-context text, keyed by context
}

// TemplatePair is the editable text for one presence context: the two lines
// Discord renders.
type TemplatePair struct {
	Details string `json:"details"`
	State   string `json:"state"`
}

// BehaviorConfig holds launch-related settings.
type BehaviorConfig struct {
	LaunchAtStartup bool   `json:"launch_at_startup"` // start with Windows
	CloseAction     string `json:"close_action"`      // ask | tray | quit
	NotifyUpdates   bool   `json:"notify_updates"`    // show system notifications when update available

	// ShowPlaceholderPresence covers the gap between Valorant starting and its
	// first presence arriving, which would otherwise show nothing at all.
	ShowPlaceholderPresence bool `json:"show_placeholder_presence"`
}

// AdvancedConfig holds tuning knobs and debug options.
type AdvancedConfig struct {
	UpdateInterval int  `json:"update_interval"` // RPC update throttle, ms
	DebugMode      bool `json:"debug_mode"`      // verbose logging
}

// DefaultConfig returns a fully populated tree at the current schema version.
func DefaultConfig() *Config {
	return &Config{
		SchemaVersion:      CurrentSchemaVersion,
		DiscordAppID:       constants.DiscordAppIDDefault,
		Theme:              ThemeSystem,
		OnboardingComplete: false,
		Display: DisplayConfig{
			Default: DisplayDefaults{ShowRank: true, ShowStats: true, MatchImage: MatchImageAgent},
			Locale:  types.LocaleAuto,
		},
		Presence: PresenceConfig{
			ShowInClient: true,
			Templates:    defaultTemplates(),
		},
		Behavior: BehaviorConfig{
			LaunchAtStartup:         true,
			CloseAction:             CloseAsk,
			NotifyUpdates:           true,
			ShowPlaceholderPresence: true,
		},
		Advanced: AdvancedConfig{
			UpdateInterval: 1500,
			DebugMode:      false,
		},
	}
}

// defaultTemplates returns the built-in per-context presence templates. Every
// entry renders to the string the app produced before templates were editable.
func defaultTemplates() map[string]TemplatePair {
	m := make(map[string]TemplatePair, len(template.Contexts()))
	for _, ctx := range template.Contexts() {
		d, s := template.Default(ctx)
		m[string(ctx)] = TemplatePair{Details: d, State: s}
	}
	return m
}

// Bounds for the numeric settings. Shared by Validate and clamp so the
// reject path and the load-time repair path can't drift apart.
const (
	MinUpdateInterval = 500
	MaxUpdateInterval = 10000
)

// Large-image choices during a match. The agent is read from Valorant's own
// log, which can stop resolving if Riot renames a log line, so the card is
// both a preference and the fallback.
const (
	MatchImageAgent = "agent"
	MatchImageCard  = "card"
)

func validMatchImage(m string) bool {
	return m == MatchImageAgent || m == MatchImageCard
}

// Theme values.
const (
	ThemeSystem = "system"
	ThemeLight  = "light"
	ThemeDark   = "dark"
)

func validTheme(t string) bool {
	return t == ThemeSystem || t == ThemeLight || t == ThemeDark
}

// What closing the window does. CloseAsk shows the in-app confirmation; the
// other two skip it once the user has picked "remember my choice".
const (
	CloseAsk  = "ask"
	CloseTray = "tray"
	CloseQuit = "quit"
)

// validLocaleSetting accepts the auto sentinel alongside a real tag. Only
// this field takes the sentinel; the catalogue itself never sees it.
func validLocaleSetting(l string) bool {
	return l == types.LocaleAuto || types.ValidLocale(l)
}

func validCloseAction(a string) bool {
	return a == CloseAsk || a == CloseTray || a == CloseQuit
}

// Validate reports every problem with c without changing it. Store.Apply and
// Save call this and surface the error; the GUI shows it next to the field.
func (c *Config) Validate() error {
	var errs []error

	if c.DiscordAppID == "" {
		errs = append(errs, errors.New("discord_app_id must not be empty"))
	}
	if !validTheme(c.Theme) {
		errs = append(errs, fmt.Errorf("theme must be one of %q, %q, %q", ThemeSystem, ThemeLight, ThemeDark))
	}
	if !validMatchImage(c.Display.Default.MatchImage) {
		errs = append(errs, fmt.Errorf("display.default.match_image must be one of %q, %q", MatchImageAgent, MatchImageCard))
	}
	if !validCloseAction(c.Behavior.CloseAction) {
		errs = append(errs, fmt.Errorf("close_action must be one of %q, %q, %q", CloseAsk, CloseTray, CloseQuit))
	}
	if c.Advanced.UpdateInterval < MinUpdateInterval || c.Advanced.UpdateInterval > MaxUpdateInterval {
		errs = append(errs, fmt.Errorf("update_interval must be between %d and %d ms", MinUpdateInterval, MaxUpdateInterval))
	}
	if !validLocaleSetting(c.Display.Locale) {
		errs = append(errs, fmt.Errorf("locale must be %q or one of the languages valorant-api.com serves, got %q", types.LocaleAuto, c.Display.Locale))
	}

	return errors.Join(errs...)
}

// clamp forces c into valid bounds in place. Only the load path uses it, so a
// hand-broken or older config file on disk still boots with sane values.
func (c *Config) clamp() {
	def := DefaultConfig()

	if c.DiscordAppID == "" {
		c.DiscordAppID = def.DiscordAppID
	}
	if !validTheme(c.Theme) {
		c.Theme = def.Theme
	}
	// Also the upgrade path: a config written before this field existed has it
	// empty and lands on the default rather than failing to load.
	if !validCloseAction(c.Behavior.CloseAction) {
		c.Behavior.CloseAction = def.Behavior.CloseAction
	}
	if c.Advanced.UpdateInterval < MinUpdateInterval || c.Advanced.UpdateInterval > MaxUpdateInterval {
		c.Advanced.UpdateInterval = def.Advanced.UpdateInterval
	}
	// Empty here is a file written before the field existed. Either way the
	// repair is the default, never a blank name in the presence.
	if !validLocaleSetting(c.Display.Locale) {
		c.Display.Locale = def.Display.Locale
	}
	// Empty here is a file written before the field existed, which is every
	// config from before the agent lookup shipped.
	if !validMatchImage(c.Display.Default.MatchImage) {
		c.Display.Default.MatchImage = def.Display.Default.MatchImage
	}
	if c.Presence.Templates == nil {
		c.Presence.Templates = map[string]TemplatePair{}
	}
	// Backfill any context the file is missing so the GUI always has an entry
	// to show and edit; never touch one the user already set.
	for k, v := range defaultTemplates() {
		if _, ok := c.Presence.Templates[k]; !ok {
			c.Presence.Templates[k] = v
		}
	}
}
