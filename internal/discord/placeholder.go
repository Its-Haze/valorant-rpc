package discord

import "github.com/its-haze/valorant-rpc/pkg/constants"

// BuildLaunchingPresence is the presence shown while Valorant is running but
// no presence has been read from it yet. Ticket 10 owns its final form.
func BuildLaunchingPresence(start int64) *RPCData {
	return &RPCData{
		LargeText: constants.AppName,
		SmallText: constants.SmallText,
		Details:   "Launching VALORANT...",
		State:     constants.AppName,
		Start:     start,
	}
}
