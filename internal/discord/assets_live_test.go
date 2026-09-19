package discord

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"sort"
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
	// repoURL is checked anonymously, the way Discord's image proxy sees it.
	repoURL = "https://github.com/Its-Haze/valorant-rpc"

	// liveProbes is the concurrency against valorant-api.com. Small on
	// purpose: this is a free community API, not a load target.
	liveProbes = 6

	liveTimeout = 30 * time.Second

	// cardSample caps how many of the ~1000 cards a normal run checks. They
	// share one URL shape, and fullCardScanEnv checks every one in ~7s.
	cardSample      = 25
	fullCardScanEnv = "VALORANT_RPC_FULL_ASSET_SCAN"
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

			if err := probe(client, a.url); err != nil {
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

// probe HEADs a URL. The CDN answers HEAD, so nothing downloads an image.
func probe(client *http.Client, url string) error {
	req, err := http.NewRequest(http.MethodHead, url, nil)
	if err != nil {
		return err
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s returned %d", url, resp.StatusCode)
	}
	return nil
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

// TestPlayerCardImagesResolve samples the card art, because any card a
// player has equipped becomes their large image in the lobby contexts.
func TestPlayerCardImagesResolve(t *testing.T) {
	if testing.Short() {
		t.Skip("network test")
	}

	cards := liveCatalogue(t).PlayerCards()
	if len(cards) == 0 {
		t.Fatal("no player cards in the catalogue")
	}

	_, full := os.LookupEnv(fullCardScanEnv)
	step := 1
	if !full && len(cards) > cardSample {
		step = len(cards) / cardSample
	}

	var assets []asset
	for i := 0; i < len(cards); i += step {
		assets = append(assets, asset{"card " + cards[i].UUID + " displayIcon", cards[i].Icon})
	}

	if full {
		t.Logf("checking all %d cards", len(cards))
	} else {
		t.Logf("sampling %d of %d cards; set %s=1 to check every one", len(assets), len(cards), fullCardScanEnv)
	}
	checkAll(t, assets)
}

// TestRepoHostedImagesResolve checks this repository's own images. A private
// repo 404s for Discord's proxy, so the check starts working when it opens.
func TestRepoHostedImagesResolve(t *testing.T) {
	if testing.Short() {
		t.Skip("network test")
	}

	if err := probe(liveClient(), repoURL); err != nil {
		t.Skipf("%s is not readable anonymously, so its hotlinked images cannot resolve for anyone yet: %v", repoURL, err)
	}

	checkAll(t, []asset{{"valorantLogoURL", valorantLogoURL}})
}
