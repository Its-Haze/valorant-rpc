package template

import (
	"slices"
	"strings"
	"testing"
)

// valorantContexts is the six the app ships, in phase order.
var valorantContexts = []Context{
	ContextInClient, ContextInLobby, ContextInQueue, ContextCustomGame, ContextAgentSelect, ContextInMatch,
}

func TestEveryValorantContextIsInstalled(t *testing.T) {
	for _, ctx := range valorantContexts {
		if !IsContext(ctx) {
			t.Errorf("IsContext(%q) = false", ctx)
		}
		if len(KnownTokens(ctx)) == 0 {
			t.Errorf("KnownTokens(%q) is empty", ctx)
		}
		details, state := Default(ctx)
		if details == "" && state == "" {
			t.Errorf("%q has no default template", ctx)
		}
	}
}

// The order drives the settings screen, so it follows the phases a player
// actually moves through rather than map order.
func TestContextsAreInPhaseOrder(t *testing.T) {
	got := Contexts()

	if len(got) < len(valorantContexts) {
		t.Fatalf("Contexts() = %v, want at least the six Valorant contexts", got)
	}
	if !slices.Equal(got[:len(valorantContexts)], valorantContexts) {
		t.Errorf("Contexts() starts %v, want %v", got[:len(valorantContexts)], valorantContexts)
	}
}

// Every token the ticket names has to exist somewhere, or a template author
// has no way to reach that piece of state.
func TestEveryRequiredTokenIsReachable(t *testing.T) {
	required := []string{
		"map", "mode", "score", "score_ally", "score_enemy", "rank",
		"party", "party_size", "max_party_size", "account_level", "riot_id", "idle", "agent",
	}

	reachable := map[string]bool{}
	for _, ctx := range valorantContexts {
		for _, tok := range KnownTokens(ctx) {
			reachable[tok] = true
		}
	}

	for _, tok := range required {
		if !reachable[tok] {
			t.Errorf("token %q is not valid in any context", tok)
		}
	}
}

// idle is orthogonal to phase, so it is a token everywhere rather than a
// context of its own. See the spec.
func TestIdleIsAvailableInEveryContext(t *testing.T) {
	for _, ctx := range valorantContexts {
		if !slices.Contains(KnownTokens(ctx), "idle") {
			t.Errorf("%q cannot render {idle}", ctx)
		}
	}
}

// A default that names a token the context does not know would render as a
// literal brace in a real presence.
func TestDefaultsOnlyUseKnownTokens(t *testing.T) {
	for _, ctx := range valorantContexts {
		details, state := Default(ctx)

		for _, tmpl := range []string{details, state} {
			_, unknown := Render(ctx, tmpl, SampleData(ctx))
			if len(unknown) > 0 {
				t.Errorf("%q default %q uses unknown tokens %v", ctx, tmpl, unknown)
			}
		}
	}
}

// Previews render the defaults against sample data, so a token with no sample
// would silently collapse and show the author a misleading preview.
func TestSampleDataCoversEveryToken(t *testing.T) {
	for _, ctx := range valorantContexts {
		sample := SampleData(ctx)

		for _, tok := range KnownTokens(ctx) {
			// agent stays empty until v0.2 wires the lookup.
			if tok == "agent" {
				continue
			}
			if sample[tok] == "" {
				t.Errorf("%q has no sample value for {%s}", ctx, tok)
			}
		}
	}
}

// The preview is what the settings screen shows, so a default rendered
// against its own sample data must come out as readable text.
func TestDefaultsRenderAgainstSampleData(t *testing.T) {
	for _, ctx := range valorantContexts {
		details, state, warnings := RenderPair(ctx, "", "", SampleData(ctx))

		if len(warnings) > 0 {
			t.Errorf("%q previewed with warnings %v", ctx, warnings)
		}
		if strings.TrimSpace(details) == "" && strings.TrimSpace(state) == "" {
			t.Errorf("%q previews as blank", ctx)
		}
		for _, line := range []string{details, state} {
			if strings.Contains(line, "{") || strings.Contains(line, "}") {
				t.Errorf("%q previewed with an unsubstituted token: %q", ctx, line)
			}
		}
	}
}

// v0.1 has no agent lookup, so every default must still read correctly with
// {agent} empty rather than leaving a dangling separator.
func TestDefaultsSurviveAnEmptyAgent(t *testing.T) {
	for _, ctx := range valorantContexts {
		sample := SampleData(ctx)
		sample["agent"] = ""

		details, state, _ := RenderPair(ctx, "", "", sample)
		for _, line := range []string{details, state} {
			if line != strings.TrimSpace(line) {
				t.Errorf("%q left untrimmed whitespace: %q", ctx, line)
			}
			if strings.Contains(line, "· ·") || strings.HasSuffix(line, "·") {
				t.Errorf("%q left a dangling separator: %q", ctx, line)
			}
		}
	}
}
