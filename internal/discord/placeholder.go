package discord

import (
	"github.com/its-haze/valorant-rpc/internal/content"
	"github.com/its-haze/valorant-rpc/pkg/constants"
)

// BuildLaunchingPresence is the presence shown while Valorant is running but
// no presence has been read from it yet. cat may be nil.
func BuildLaunchingPresence(start int64, cat *content.Catalogue) *RPCData {
	// A random card on every rotation, the way league-rpc rotates skins.
	// Nothing about the player is known yet, so any face will do.
	largeImage := valorantLogoURL
	if card, ok := cat.RandomPlayerCard(); ok && card.Icon != "" {
		largeImage = card.Icon
	}

	return &RPCData{
		LargeImage: largeImage,
		LargeText:  constants.AppName,
		SmallImage: valorantLogoBorderlessURL,
		SmallText:  constants.SmallText,
		Details:    "Launching VALORANT...",
		State:      constants.AppName,
		Start:      start,
	}
}
