package updates

import (
	_ "embed"
	"time"

	wupdater "github.com/wailsapp/wails/v3/pkg/updater"
	"github.com/wailsapp/wails/v3/pkg/updater/providers/github"
)

// RepoSlug is the GitHub repository the App Update flow pulls releases from.
const RepoSlug = "Its-Haze/valorant-rpc"

// ReleaseAsset is the published binary the updater downloads. Named .bin
const ReleaseAsset = "valorant-rpc-windows-amd64.bin"

// ChecksumAsset is the sidecar every release publishes alongside the binary.
// Its ed25519 signature is verified against publicKeyPEM before any swap.
const ChecksumAsset = "SHA256SUMS"

// CheckInterval is how often the background check re-hits GitHub Releases
// after the launch check.
const CheckInterval = 30 * time.Minute

// publicKeyPEM is the release-signature trust root; its private half exists
// only as the UPDATE_SIGNING_KEY secret. See docs/release-signing.md.
//
//go:embed keys/update-public.pem
var publicKeyPEM []byte

// BuildConfig assembles the Wails updater configuration for currentVersion:
// GitHub Releases as the only provider, stable channel, no built-in window.
func BuildConfig(currentVersion string) (wupdater.Config, error) {
	provider, err := github.New(github.Config{
		Repository:    RepoSlug,
		Prerelease:    false,
		ChecksumAsset: ChecksumAsset,
	})
	if err != nil {
		return wupdater.Config{}, err
	}
	signed := newSignedGithubProvider(provider, RepoSlug, NewProductionHTTPDoer())
	return wupdater.Config{
		CurrentVersion: currentVersion,
		Providers:      []wupdater.Provider{signed},
		PublicKey:      publicKeyPEM,
		Window:         wupdater.WindowNone,
	}, nil
}
