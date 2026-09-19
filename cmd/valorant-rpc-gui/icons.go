package main

import _ "embed"

// trayIcon is the systray-sized mark, the same borderless art the presence
// small icon uses. One icon for light and dark alike.
//
//go:embed tray-icon.png
var trayIcon []byte

// appIcon is the window icon and the dev-mode taskbar fallback, the same
// borderless mark at full size.
//
//go:embed app-icon.png
var appIcon []byte
