package discord

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"

	"github.com/its-haze/valorant-rpc/internal/content"
	"github.com/its-haze/valorant-rpc/pkg/types"
)

// These tests hit the live network. They exist because a presence image that
// stops resolving fails silently: Discord just shows no picture.

const (
	// liveProbes is the concurrency against valorant-api.com. Small on
	// purpose: this is a free community API, not a load target.
	liveProbes = 6

	liveTimeout = 30 * time.Second
)

// asset is one URL the builders can emit, labelled so a failure names the
// thing that broke rather than just a URL.
type asset struct {
	label string
	url   string
}

func liveClient() *http.Client { return &http.Client{Timeout: liveTimeout} }

// liveCatalogue fetches the real catalogue, so the test covers every UUID
// the app knows about today rather than the ones frozen into fixtures.
func liveCatalogue(t *testing.T) *content.Catalogue {
	t.Helper()

	cache := content.New(content.Options{Doer: liveClient(), Logger: zerolog.Nop()})
	if err := cache.Refresh(context.Background()); err != nil {
		t.Fatalf("fetching the live catalogue: %v", err)
	}
	return cache.Snapshot()
}

// checkAll probes every asset and reports each failure by label.
func checkAll(t *testing.T, assets []asset) {
	t.Helper()

	client := liveClient()
	var (
		wg   sync.WaitGroup
		mu   sync.Mutex
		bad  []string
		gate = make(chan struct{}, liveProbes)
	)

	for _, a := range assets {
		wg.Add(1)
		go func() {
			defer wg.Done()
			gate <- struct{}{}
			defer func() { <-gate }()

			if err := probeImage(client, a.url); err != nil {
				mu.Lock()
				bad = append(bad, fmt.Sprintf("%s: %v", a.label, err))
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	sort.Strings(bad)
	for _, line := range bad {
		t.Error(line)
	}
	t.Logf("checked %d assets, %d broken", len(assets), len(bad))
}

// probeImage HEADs a URL and insists the answer is actually an image.
// Checking the status alone is not enough: GitHub answers a missing file
// under blob/...?raw=true with 200 and an HTML page, so a broken hotlink
// would sail through a status-only check.
func probeImage(client *http.Client, url string) error {
	resp, err := head(client, url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s returned %d", url, resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "image/") {
		return fmt.Errorf("%s returned %d but served %q, not an image", url, resp.StatusCode, ct)
	}
	return nil
}

func head(client *http.Client, url string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodHead, url, nil)
	if err != nil {
		return nil, err
	}
	return client.Do(req)
}

// TestPresenceImagesResolve checks every valorant-api image a builder can
// emit. A retired asset breaks presence silently, so CI has to catch it.
func TestPresenceImagesResolve(t *testing.T) {
	if testing.Short() {
		t.Skip("network test")
	}

	cat := liveCatalogue(t)
	var assets []asset

	// Only the variants the builders actually use. Adding a variant to a
	// builder means adding it here.
	for _, agent := range cat.Agents(types.DefaultLocale) {
		assets = append(assets, asset{"agent " + agent.Name + " displayIcon", agent.Icon})
	}
	for _, world := range cat.Maps(types.DefaultLocale) {
		assets = append(assets, asset{"map " + world.URL + " splash", world.Splash})
	}
	for _, tier := range cat.Tiers(types.DefaultLocale) {
		if tier.LargeIcon == "" {
			continue
		}
		assets = append(assets, asset{fmt.Sprintf("tier %d (%s) largeIcon", tier.Tier, tier.Name), tier.LargeIcon})
	}

	if len(assets) < 50 {
		t.Fatalf("only %d assets collected; the catalogue looks wrong", len(assets))
	}
	checkAll(t, assets)
}

// TestPlayerCardImagesResolve checks one card, because any card a player has
// equipped becomes their large image in the lobby contexts. Every card shares
// one URL shape, so one proves the shape and ~1000 only prove valorant-api is
// up.
func TestPlayerCardImagesResolve(t *testing.T) {
	if testing.Short() {
		t.Skip("network test")
	}

	cards := liveCatalogue(t).PlayerCards()
	if len(cards) == 0 {
		t.Fatal("no player cards in the catalogue")
	}

	// PlayerCards sorts by UUID, so this picks the same card every run.
	card := cards[0]
	checkAll(t, []asset{{"card " + card.UUID + " displayIcon", card.Icon}})
}

// TestRepoHostedImagesResolve checks this repository's own images, which the
// builders hotlink and Discord's proxy fetches anonymously.
func TestRepoHostedImagesResolve(t *testing.T) {
	if testing.Short() {
		t.Skip("network test")
	}

	checkAll(t, []asset{
		{"valorantLogoURL", valorantLogoURL},
		{"valorantLogoBorderlessURL", valorantLogoBorderlessURL},
		{"valorantLogoIdleURL", valorantLogoIdleURL},
	})
}
