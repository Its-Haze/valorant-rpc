package riotclient

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// maxRegionLocaleBytes caps the read. The real payload is three short fields.
const maxRegionLocaleBytes = 4 << 10

// RegionLocale is what the Riot Client reports about itself: the language it
// is running in and the region it is pointed at.
type RegionLocale struct {
	Locale    string `json:"locale"`
	Region    string `json:"region"`
	WebRegion string `json:"webRegion"`
}

// RegionLocale reads the Riot Client's own language and region. The locale
// is a client tag such as en-GB, which is not always one valorant-api serves.
func (c *Client) RegionLocale(ctx context.Context) (RegionLocale, error) {
	ctx, cancel := context.WithTimeout(ctx, c.opts.RequestTimeout)
	defer cancel()

	resp, err := c.Get(ctx, healthEndpoint)
	if err != nil {
		return RegionLocale{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, maxRegionLocaleBytes))
		return RegionLocale{}, fmt.Errorf("riotclient: %s returned status %d", healthEndpoint, resp.StatusCode)
	}

	var out RegionLocale
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxRegionLocaleBytes)).Decode(&out); err != nil {
		return RegionLocale{}, fmt.Errorf("riotclient: decoding %s: %w", healthEndpoint, err)
	}
	return out, nil
}
