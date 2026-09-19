import { AppWindow, EyeOff, Gamepad2, ShieldCheck, type LucideIcon } from "lucide-react";
import { DISCORD_COMMUNITY_URL, DISCORD_DEVELOPER_PORTAL_URL, GITHUB_REPO_URL } from "./links";

export interface FaqLink {
  label: string;
  href: string;
}

export interface FaqEntry {
  question: string;
  answer: string;
  links?: FaqLink[];
}

export interface FaqGroup {
  id: string;
  title: string;
  icon: LucideIcon;
  entries: FaqEntry[];
}

// Plain data, no JSX: the screen owns rendering, including routing links
// through openExternal so the webview doesn't navigate itself.
export const FAQ_GROUPS: FaqGroup[] = [
  {
    id: "safety",
    title: "Is this safe?",
    icon: ShieldCheck,
    entries: [
      {
        question: "Will this get my account banned?",
        answer:
          "No. It reads endpoints the Riot Client already runs on your own machine, injects nothing and touches no game files. It never writes anything back to Riot. Vanguard has no reason to care.",
      },
      {
        question: "What does it actually read?",
        answer:
          "Your own presence: your Riot ID, account level, rank, party size, map, mode and round score. Nothing about anyone else in your match, and nothing about the game server.",
        links: [{ label: "Read the source", href: GITHUB_REPO_URL }],
      },
      {
        question: "Why does Windows say it's dangerous?",
        answer:
          "It isn't code-signed, so SmartScreen gets twitchy about an .exe it hasn't seen before. That's the whole reason.",
      },
      {
        question: "Is this made by Riot?",
        answer: "No. Independent open-source project, nothing to do with Riot.",
      },
    ],
  },
  {
    id: "not-showing",
    title: "Presence isn't showing",
    icon: EyeOff,
    entries: [
      {
        question: "Discord shows nothing at all",
        answer:
          "It only talks to Discord while Valorant is running, so your status clears on purpose when you close the game. Check you're on the Discord desktop app too, the browser has no Rich Presence.",
        links: [{ label: "Ask on Discord", href: DISCORD_COMMUNITY_URL }],
      },
      {
        question: "It says it can't reach the Riot Client",
        answer:
          "That means it connected but Riot never sent a presence. Restarting the Riot Client usually sorts it. If it keeps happening, that's a bug worth reporting.",
      },
      {
        question: "Did I leave it paused?",
        answer:
          "Pause is on the Behavior page and it clears your status until you switch it back off. Worth a glance before assuming something's broken.",
      },
    ],
  },
  {
    id: "app",
    title: "Using the app",
    icon: AppWindow,
    entries: [
      {
        question: "I opened Valorant RPC but Valorant didn't start",
        answer:
          "It doesn't launch Valorant, and that's deliberate: it's built to start with Windows, and nobody wants Valorant opening the moment they boot. Start Valorant however you normally do and Valorant RPC picks it up, even mid-match.",
      },
      {
        question: "I closed the window and it's still running",
        answer:
          "On purpose. It hides to the tray so your presence keeps updating. Click the tray icon to bring the window back, right-click it for pause and quit, or make the X really mean quit under Behavior.",
      },
      {
        question: "Where are the logs?",
        answer:
          "Help page, Open logs folder. Copy diagnostics is faster and grabs everything I'd ask you for anyway.",
      },
      {
        question: "Can I change the app name Discord shows?",
        answer:
          "Advanced page, Application ID. Point it at your own Discord application and the bold name and icon come from that instead. The lines under it still come from Display.",
        links: [{ label: "Discord developer portal", href: DISCORD_DEVELOPER_PORTAL_URL }],
      },
    ],
  },
  {
    id: "game-data",
    title: "Game data",
    icon: Gamepad2,
    entries: [
      {
        question: "Why doesn't it show which agent I picked?",
        answer:
          "Riot's local presence doesn't carry it, so there's nothing to read yet. Agent lookup is planned; until then the agent token renders empty and the line closes up around it.",
      },
      {
        question: "Can I see names in my own language?",
        answer:
          "Yes. Display page, Name language. Automatic follows whatever language your Riot Client is in, and switching takes effect on your next status update.",
      },
      {
        question: "Why is the round score missing early in a match?",
        answer: "It only appears once somebody has won a round. Riot reports 0-0 all through the menus, and that isn't a score.",
      },
    ],
  },
];
