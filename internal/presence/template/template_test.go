package template

import (
	"reflect"
	"testing"
)

// The context tables are empty until the presence builders fill them, so these
// tests install their own and exercise the engine against that.
const (
	ctxOne Context = "ctx-one"
	ctxTwo Context = "ctx-two"
)

// install registers ctx with tokens and a default pair for the duration of t.
func install(t *testing.T, ctx Context, tokens []string, details, state string) {
	t.Helper()
	knownTokens[ctx] = tokens
	defaults[ctx] = [2]string{details, state}
	sampleData[ctx] = map[string]string{}
	order = append(order, ctx)
	t.Cleanup(func() {
		delete(knownTokens, ctx)
		delete(defaults, ctx)
		delete(sampleData, ctx)
		order = order[:len(order)-1]
	})
}

func TestRender_Substitution(t *testing.T) {
	install(t, ctxOne, []string{"emoji", "mode", "score", "agent", "map"}, "{mode}", "{score}")

	tests := []struct {
		name string
		tmpl string
		data map[string]string
		want string
	}{
		{
			name: "single token",
			tmpl: "{mode}",
			data: map[string]string{"mode": "Competitive"},
			want: "Competitive",
		},
		{
			name: "token among literal text",
			tmpl: "In Match · {score}",
			data: map[string]string{"score": "7-5"},
			want: "In Match · 7-5",
		},
		{
			name: "repeated token substituted every time",
			tmpl: "{map} — playing {map}",
			data: map[string]string{"map": "Ascent"},
			want: "Ascent — playing Ascent",
		},
		{
			name: "literal two spaces between tokens are preserved",
			tmpl: "{emoji}  {mode}",
			data: map[string]string{"emoji": "🟢", "mode": "Competitive"},
			want: "🟢  Competitive",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, warnings := Render(ctxOne, tt.tmpl, tt.data)
			if got != tt.want {
				t.Errorf("Render() = %q, want %q", got, tt.want)
			}
			if len(warnings) != 0 {
				t.Errorf("warnings = %v, want none", warnings)
			}
		})
	}
}

func TestRender_UnknownTokenLeftLiteralAndReported(t *testing.T) {
	install(t, ctxOne, []string{"mode"}, "{mode}", "In Match")

	got, warnings := Render(ctxOne, "hi {mode} {foo} {bar} {foo}", map[string]string{
		"mode": "Competitive",
	})
	if want := "hi Competitive {foo} {bar} {foo}"; got != want {
		t.Errorf("Render() = %q, want %q", got, want)
	}
	if want := []string{"foo", "bar"}; !reflect.DeepEqual(warnings, want) {
		t.Errorf("warnings = %v, want %v (first appearance, deduped)", warnings, want)
	}
}

func TestRender_EmptyTokenCollapse(t *testing.T) {
	install(t, ctxOne, []string{"emoji", "mode", "score", "agent", "map"}, "{mode}", "{score}")

	tests := []struct {
		name string
		tmpl string
		data map[string]string
		want string
	}{
		{
			name: "leading empty token and its whitespace vanish",
			tmpl: "{emoji}  {mode}",
			data: map[string]string{"mode": "Competitive"},
			want: "Competitive",
		},
		{
			name: "trailing empty token leaves no space",
			tmpl: "In Match {score}",
			data: map[string]string{},
			want: "In Match",
		},
		{
			name: "trailing empty token with dangling middot",
			tmpl: "In Match · {score}",
			data: map[string]string{},
			want: "In Match",
		},
		{
			name: "middle empty token collapses to one space",
			tmpl: "{agent} {score} {map}",
			data: map[string]string{"agent": "Jett", "map": "Ascent"},
			want: "Jett Ascent",
		},
		{
			name: "middle empty token between separators folds the run",
			tmpl: "{agent} • {score} • {map}",
			data: map[string]string{"agent": "Jett", "map": "Ascent"},
			want: "Jett • Ascent",
		},
		{
			name: "everything empty renders empty",
			tmpl: "{map}",
			data: map[string]string{},
			want: "",
		},
		{
			name: "non-empty middot separators inside a value are untouched",
			tmpl: "In Match · {score}",
			data: map[string]string{"score": "7-5 · Ascent · Competitive"},
			want: "In Match · 7-5 · Ascent · Competitive",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _ := Render(ctxOne, tt.tmpl, tt.data)
			if got != tt.want {
				t.Errorf("Render() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRenderPair_BlankOverrideFallsBackToDefault(t *testing.T) {
	install(t, ctxOne, []string{"mode", "score"}, "{mode}", "In Match · {score}")
	data := map[string]string{"mode": "Competitive", "score": "7-5"}

	// Both lines blank: identical to rendering the two defaults.
	d, s, warn := RenderPair(ctxOne, "", "", data)
	if d != "Competitive" || s != "In Match · 7-5" {
		t.Fatalf("blank override = %q / %q, want the defaults", d, s)
	}
	if warn != nil {
		t.Fatalf("warnings = %v, want none", warn)
	}

	// One line overridden, the other blank and still defaulted.
	d, s, _ = RenderPair(ctxOne, "{mode} ranked", "", data)
	if d != "Competitive ranked" || s != "In Match · 7-5" {
		t.Fatalf("mixed override = %q / %q", d, s)
	}
}

func TestRenderPair_DedupesUnknownAcrossLines(t *testing.T) {
	install(t, ctxOne, []string{"mode"}, "{mode}", "In Match")

	_, _, warn := RenderPair(ctxOne, "{foo} {mode}", "{foo} {bar}", map[string]string{"mode": "Competitive"})
	if want := []string{"foo", "bar"}; !reflect.DeepEqual(warn, want) {
		t.Fatalf("warnings = %v, want %v", warn, want)
	}
}

func TestContextsAndKnownTokens(t *testing.T) {
	install(t, ctxOne, []string{"mode"}, "{mode}", "In Match")
	install(t, ctxTwo, []string{"score"}, "{score}", "In Match")

	// The app's own contexts are already registered, so the two installed
	// here land at the end, in the order they were installed.
	got := Contexts()
	if tail := got[len(got)-2:]; !reflect.DeepEqual(tail, []Context{ctxOne, ctxTwo}) {
		t.Fatalf("Contexts() ends %v, want the two installed, in order", tail)
	}
	for _, ctx := range Contexts() {
		if !IsContext(ctx) {
			t.Errorf("IsContext(%q) = false", ctx)
		}
		if len(KnownTokens(ctx)) == 0 {
			t.Errorf("KnownTokens(%q) is empty", ctx)
		}
	}
	if IsContext("nonsense") {
		t.Error("IsContext(nonsense) = true")
	}
	if KnownTokens("nonsense") != nil {
		t.Error("KnownTokens(nonsense) should be nil")
	}
}

// Contexts() must hand back a copy: a caller mutating it cannot reorder the
// package's own chronology.
func TestContexts_ReturnsACopy(t *testing.T) {
	install(t, ctxOne, []string{"mode"}, "{mode}", "In Match")

	before := Contexts()
	clobbered := Contexts()
	clobbered[0] = "clobbered"

	if !reflect.DeepEqual(Contexts(), before) {
		t.Fatal("Contexts() handed out the backing slice")
	}
}
