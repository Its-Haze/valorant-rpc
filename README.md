<div align="center">

<img src="assets/valorant-logo.png" width="15%" height="auto" alt="Valorant RPC" />

<h1>Valorant RPC</h1>

<p>A better Valorant Rich Presence for Discord.</p>

<p>
<a href="https://github.com/Its-Haze/valorant-rpc/releases/latest"><img alt="Downloads" src="https://img.shields.io/github/downloads/Its-Haze/valorant-rpc/total.svg?style=for-the-badge&color=A6E3A1&labelColor=11111B"></a>
<a href="https://github.com/Its-Haze/valorant-rpc/stargazers"><img alt="Stargazers" src="https://img.shields.io/github/stars/Its-Haze/valorant-rpc.svg?style=for-the-badge&color=F9E2AF&labelColor=11111B"></a>
<a href="https://github.com/Its-Haze/valorant-rpc/releases/latest"><img alt="Latest release" src="https://img.shields.io/github/v/release/Its-Haze/valorant-rpc?style=for-the-badge&color=CBA6F7&labelColor=11111B"></a>
<a href="https://github.com/Its-Haze/valorant-rpc/blob/main/LICENSE"><img alt="MIT License" src="https://img.shields.io/badge/license-MIT-7F849C?style=for-the-badge&labelColor=11111B"></a>
</p>

<h3><a href="https://github.com/Its-Haze/valorant-rpc/releases/latest"><strong>Download for Windows &raquo;</strong></a></h3>

<p>
<a href="#showcase">Showcase</a>
&middot;
<a href="#installation">Installation</a>
&middot;
<a href="#-faq">FAQ</a>
</p>

</div>

Your map, mode, rank and round score on your Discord profile, and every line of it is yours to
rewrite. It reads what the Riot Client already publishes on your own PC, so there's nothing to
configure and nothing for Vanguard to object to.

---

## Showcase

### In a Match

Your map as the artwork, the round score as it happens, and a timer that runs from the first round.

![in-game](images/in_match_showcase.png)

### Ranked Games

Your rank emblem, right there on your presence. Competitive only, so an unrated game never advertises your rank, and **Show rank** turns it off entirely.

![ranked-1](images/in_queue_show_rank_1.png) ![ranked-2](images/in_match_show_rank_1.png)

### In the Client

Your equipped player card shows up between matches, the same art the game puts next to your name.

![player-card-1](images/in_client_card_1.png) ![player-card-2](images/in_client_card_2.png)

There's an idle marker too, for when you've stepped away.

![Online](images/in_client_online_status.png) ![Idle](images/in_client_idle_status.png)

### Write Your Own

Every line Discord shows is a template. Rewrite it, drop in your map, mode, party or score, and the preview updates as you type. Each situation has its own: client, queue, custom game, agent select and in a match.

![presence-text-editor](images/presence-text.gif)

---

## Installation

### 📥 Getting Started
1. Head over to the [Releases Page](https://github.com/Its-Haze/valorant-rpc/releases)
2. Download `valorant-rpc-<version>-setup.exe` from the latest release (it's under Assets)
3. Run it and accept the Windows security popup if it shows up
4. Start Valorant and Discord, in whatever order you like
5. That's it! ✨

Closing the window keeps Valorant RPC running in your system tray. Quit from there when you want it to stop.

Already running [League RPC](https://github.com/Its-Haze/league-rpc)? The two run side by side without stepping on each other.

### 🔄 Updating
You can update directly from the app. If you have **update notifications** enabled, you will see a notification that a new version is available.

Otherwise go to **About** → **Check for updates** and install from there.

---

## ❓ FAQ

### 🚫 Will this get my account banned?
Nope! It only reads what the Riot Client already publishes on your own computer. It changes nothing, injects nothing, never writes anything back to Riot, and gives you no advantage in game, so Vanguard has no reason to care. It's an independent open-source project, not affiliated with Riot Games.

### 🛡️ Is this a virus? Why is Windows warning me?
No, and because it isn't code-signed. A certificate costs $100 a year, which is hard to justify for a free project, so Windows distrusts an installer it hasn't seen before. Click **More info**, then **Run anyway**, and whitelist it if Defender gets loud. The entire source code is public on GitHub, so review it or build it yourself.

### 🎭 Why doesn't it show my agent?
Riot doesn't publish that one on your own machine, so there's nothing to read yet. It's the next thing on the list.

There's a longer FAQ inside the app, under **Help**, for the questions you only run into once it's running.

---

## 🏗️ Build from Source
For the cool kids who want to build it themselves:

```powershell
# Clone and navigate
git clone https://github.com/Its-Haze/valorant-rpc.git
cd valorant-rpc

# Build
task build
```

You'll need Go, Node and [Task](https://taskfile.dev/). [CONTRIBUTING.md](CONTRIBUTING.md) has the rest.

---

## 📞 Contact and Support
Got questions? Join the [Discord Server](https://discord.haze.sh)
Feel free to open up Help tickets, or contact me directly on Discord (@haze.dev).

For issues related to the code, or project as a whole, please open an [issue on GitHub](https://github.com/Its-Haze/valorant-rpc/issues). Before you do, hit **Copy diagnostics** on the app's Help screen and paste the result in. It gathers most of what I'd otherwise have to ask you for.

⭐ If you enjoy it, don't forget to star this project! ⭐

## Star History

<a href="https://star-history.com/#Its-Haze/valorant-rpc&Date">
 <picture>
   <source media="(prefers-color-scheme: dark)" srcset="https://api.star-history.com/svg?repos=Its-Haze/valorant-rpc&type=Date&theme=dark" />
   <source media="(prefers-color-scheme: light)" srcset="https://api.star-history.com/svg?repos=Its-Haze/valorant-rpc&type=Date" />
   <img alt="Star History Chart" src="https://api.star-history.com/svg?repos=Its-Haze/valorant-rpc&type=Date" />
 </picture>
</a>
