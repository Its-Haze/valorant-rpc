package updates

import (
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"os"
	"regexp"
	"strings"
	"testing"
)

const signingDoc = "../../docs/release-signing.md"

var (
	errNoPEM      = errors.New("no PEM block")
	errNotEd25519 = errors.New("not an ed25519 public key")
)

// The app only ever looks at /releases/latest, which GitHub computes by
// skipping prereleases. That is what makes a hyphen tag safe to dry-run with.
func TestHyphenTagPublishesAsPrerelease(t *testing.T) {
	body := readFile(t, releaseWorkflow)
	if !strings.Contains(body, `if [[ "$GITHUB_REF_NAME" == *-* ]]; then`) {
		t.Fatalf("%s no longer branches on a hyphen in the tag name", releaseWorkflow)
	}
	if !strings.Contains(body, `prerelease_flag="--prerelease"`) {
		t.Fatalf("%s does not pass --prerelease for a hyphen tag", releaseWorkflow)
	}
}

// The private key is an environment secret so only this job can read it.
func TestSigningKeyStaysInsideTheProtectedEnvironment(t *testing.T) {
	body := readFile(t, releaseWorkflow)
	if !strings.Contains(body, "environment: release-signing") {
		t.Fatalf("%s no longer pins the signing job to the release-signing environment", releaseWorkflow)
	}
	if strings.Count(body, "secrets.UPDATE_SIGNING_KEY") != 1 {
		t.Fatalf("%s must read secrets.UPDATE_SIGNING_KEY in exactly one job", releaseWorkflow)
	}
}

// A tag with no curated notes fails the release either way. Failing it first
// is what keeps a doomed tag from burning a build and a use of the signing key.
func TestReleaseNotesGuardRunsBeforeTheBuild(t *testing.T) {
	body := readFile(t, releaseWorkflow)
	if !strings.Contains(body, "run: bash .github/scripts/require-release-notes.sh") {
		t.Fatalf("%s no longer runs the release-notes guard as its own step", releaseWorkflow)
	}

	guard := strings.Index(body, "preflight:")
	build := strings.Index(body, "build:")
	if guard < 0 || build < 0 || guard > build {
		t.Fatalf("%s must declare the preflight job before build", releaseWorkflow)
	}
	if !strings.Contains(body, "needs: preflight") {
		t.Fatalf("%s no longer gates the build on preflight", releaseWorkflow)
	}
}

func TestEmbeddedPublicKeyIsAUsableEd25519Key(t *testing.T) {
	if _, err := parseEmbeddedKey(); err != nil {
		t.Fatalf("keys/update-public.pem: %v", err)
	}
}

// The doc tells a person how to verify a release by hand, so a stale
// fingerprint in it is worse than no fingerprint at all.
func TestSigningDocQuotesTheEmbeddedKeyFingerprint(t *testing.T) {
	key, err := parseEmbeddedKey()
	if err != nil {
		t.Fatalf("keys/update-public.pem: %v", err)
	}
	digest := sha256.Sum256(key)
	want := hex.EncodeToString(digest[:])

	doc := readFile(t, signingDoc)
	found := regexp.MustCompile(`\b[0-9a-f]{64}\b`).FindAllString(doc, -1)
	for _, got := range found {
		if got == want {
			return
		}
	}
	t.Fatalf("%s quotes %v, want the embedded key's fingerprint %s", signingDoc, found, want)
}

func parseEmbeddedKey() (ed25519.PublicKey, error) {
	block, _ := pem.Decode(publicKeyPEM)
	if block == nil {
		return nil, errNoPEM
	}
	parsed, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	key, ok := parsed.(ed25519.PublicKey)
	if !ok {
		return nil, errNotEd25519
	}
	return key, nil
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(body)
}
