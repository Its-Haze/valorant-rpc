package main

import _ "embed"

// trayIcon is the systray-sized app icon, used for light and dark alike.
// Placeholder art carried over from the sibling app.
//
//go:embed tray-icon.png
var trayIcon []byte

// appIcon is the window icon and the dev-mode taskbar fallback. Placeholder
// art carried over from the sibling app.
//
//go:embed app-icon.png
var appIcon []byte
