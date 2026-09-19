package discord

import "testing"

func TestLaunchingPresenceHangsARandomCardOnIt(t *testing.T) {
	cat := testCatalogue(t)
	known := make(map[string]bool)
	for _, card := range cat.PlayerCards() {
		known[card.Icon] = true
	}

	seen := make(map[string]bool)
	for range 200 {
		got := BuildLaunchingPresence(0, cat).LargeImage
		if !known[got] {
			t.Fatalf("LargeImage = %q, which is not any card's art", got)
		}
		seen[got] = true
	}

	if len(seen) < 2 {
		t.Errorf("200 rotations showed %d distinct card(s); the art is not rotating", len(seen))
	}
}

func TestLaunchingPresenceFallsBackToTheAppIconWithoutACatalogue(t *testing.T) {
	if got := BuildLaunchingPresence(0, nil).LargeImage; got != valorantLogoURL {
		t.Errorf("LargeImage = %q, want the app icon %q", got, valorantLogoURL)
	}
}

func TestLaunchingPresenceWearsTheBorderlessMark(t *testing.T) {
	got := BuildLaunchingPresence(0, testCatalogue(t))
	if got.SmallImage != valorantLogoBorderlessURL {
		t.Errorf("SmallImage = %q, want the borderless mark %q", got.SmallImage, valorantLogoBorderlessURL)
	}
	if got.Details != "Launching VALORANT..." {
		t.Errorf("Details = %q", got.Details)
	}
}
