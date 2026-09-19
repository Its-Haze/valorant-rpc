package discord

// valorantLogoURL is the fallback small icon, hotlinked from this repository
// the way league-rpc hotlinks its own. Ticket 08's test guards it.
const valorantLogoURL = "https://github.com/Its-Haze/valorant-rpc/blob/master/assets/valorant-logo.png?raw=true"

// ValorantLogoURL is the app's own icon, used wherever no Riot art applies.
func ValorantLogoURL() string { return valorantLogoURL }
