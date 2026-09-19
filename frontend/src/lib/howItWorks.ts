// Copy for the onboarding "How this works" screen. Kept out of the component so
// all four toggle combinations stay readable and testable in one place.
export interface HowItWorksInputs {
  launchAtStartup: boolean;
  notifyUpdates: boolean;
}

export interface TimelineStep {
  title: string;
  body: string;
}

export interface HowItWorksCopy {
  steps: TimelineStep[];
  updates: string;
}

export function howItWorksCopy({ launchAtStartup, notifyUpdates }: HowItWorksInputs): HowItWorksCopy {
  return {
    steps: [
      {
        title: "It starts",
        body: launchAtStartup
          ? "Valorant RPC starts with Windows, minimized to the tray. You don't have to do anything."
          : "You start Valorant RPC yourself. Any time works: before Valorant, after Valorant, or mid-match. It catches up either way.",
      },
      {
        title: "It waits",
        body: "Until Valorant is running, it sits idle in the tray. No Discord status, no connection, nothing.",
      },
      {
        title: "Valorant opens, it connects",
        body: "It reads your presence from the Riot Client running on your own PC, connects to Discord, and your status goes live.",
      },
      {
        title: "Valorant closes, it sleeps",
        body: "Back to idle, waiting for the next time.",
      },
    ],
    updates: notifyUpdates
      ? "Update notifications are on. You'll get a toast when a new version is ready."
      : "Update notifications are off. Check the Discord server for new releases, or turn them back on under Behavior.",
  };
}
