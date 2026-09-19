package discord

// valorantLogoURL is the app's own mark, hotlinked from this repository the
// way league-rpc hotlinks its own. Ticket 08's test guards it.
const valorantLogoURL = "https://github.com/Its-Haze/valorant-rpc/blob/main/assets/valorant-logo.png?raw=true"

// valorantLogoBorderlessURL is the same mark without its frame. Discord
// renders the small icon at about 32px, where the frame eats the V.
const valorantLogoBorderlessURL = "https://github.com/Its-Haze/valorant-rpc/blob/main/assets/valorant-logo-borderless.png?raw=true"

// valorantLogoIdleURL is the borderless mark desaturated and dimmed. It is
// the small icon whenever Riot reports the player as idle.
const valorantLogoIdleURL = "https://github.com/Its-Haze/valorant-rpc/blob/main/assets/valorant-logo-idle.png?raw=true"

// ValorantLogoURL is the app's own icon, used wherever no Riot art applies.
func ValorantLogoURL() string { return valorantLogoURL }

// ValorantLogoSmallURL is the mark as it appears in the small-icon slot.
func ValorantLogoSmallURL() string { return valorantLogoBorderlessURL }
