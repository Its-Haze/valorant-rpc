import { Gamepad2, Globe, Info, Lightbulb, ShieldCheck, UserRound } from "lucide-react";
import type { ReactNode } from "react";
import { useEffect, useState } from "react";
import { GetVersion } from "../../../bindings/github.com/its-haze/valorant-rpc/cmd/valorant-rpc-gui/guiservice";
import { useCheckForUpdates } from "../../hooks/useCheckForUpdates";
import {
  AUTHOR_WEBSITE_URL,
  DISCORD_COMMUNITY_URL,
  GITHUB_PROFILE_URL,
  openExternal,
} from "../../lib/links";
import { DiscordIcon, GitHubIcon } from "../icons";
import { Button, SettingsCard } from "../ui";
import UpdateBanner from "../UpdateBanner";

// The About section: what the app is, who wrote it, and the running version.
export function AboutScreen() {
  const [version, setVersion] = useState("");
  const { checking, result: checkResult, check: handleCheck } = useCheckForUpdates();

  useEffect(() => {
    GetVersion().then(setVersion).catch(() => {});
  }, []);

  return (
    <div className="flex flex-col gap-8">
      <h1 className="text-xl font-semibold">About</h1>

      <SettingsCard
        icon={Info}
        title="Valorant RPC"
        description={version ? `You're running v${version}.` : "Loading version…"}
        action={
          <>
            {checkResult && <span className="text-muted text-sm">{checkResult}</span>}
            <Button variant="secondary" onClick={handleCheck} disabled={checking}>
              {checking ? "Checking…" : "Check for updates"}
            </Button>
          </>
        }
      />

      <UpdateBanner />

      <div className="grid gap-8 lg:grid-cols-2">
        <SettingsCard
          icon={Gamepad2}
          title="What it does"
          description="Turns what you're doing in Valorant into your Discord status."
        >
          <p className="pt-1 text-sm leading-relaxed">
            Your queue, map, rank, party and round score go out as you play, and follow along as
            the session moves from the client to agent select to the match itself.
          </p>
        </SettingsCard>

        <SettingsCard
          icon={Lightbulb}
          title="Why it exists"
          description="Discord knows you're playing Valorant. It doesn't know much else."
        >
          <p className="pt-1 text-sm leading-relaxed">
            Built-in detection gives you the game's name and little more: no map, no rank, no
            score, no way to word it yourself. Valorant RPC fills in the rest.
          </p>
        </SettingsCard>

        <SettingsCard
          icon={ShieldCheck}
          title="Vanguard safe"
          description="No game files are touched, and nothing here helps you win."
        >
          <p className="pt-1 text-sm leading-relaxed">
            Valorant RPC reads the Riot Client's own local API on your PC and turns what it finds
            into a Discord status. It never writes anything back to Riot, nothing is injected, no
            files are modified, and everything it knows is already on your own screen.
          </p>
        </SettingsCard>

        <SettingsCard
          icon={UserRound}
          title="Made by haze"
          description="Built and maintained in my spare time, with help from everyone who files issues."
        >
          <AboutLink href={AUTHOR_WEBSITE_URL} icon={<Globe className="size-4" />}>
            My personal website
          </AboutLink>
          <AboutLink href={GITHUB_PROFILE_URL} icon={<GitHubIcon className="size-4" />}>
            github.com/its-haze
          </AboutLink>
          <AboutLink href={DISCORD_COMMUNITY_URL} icon={<DiscordIcon className="size-4" />}>
            Join the Discord community
          </AboutLink>
        </SettingsCard>
      </div>
    </div>
  );
}

// Same icon + label row the Help section uses for its external links.
function AboutLink({ href, icon, children }: { href: string; icon: ReactNode; children: ReactNode }) {
  return (
    <a
      href={href}
      onClick={(e) => {
        e.preventDefault();
        openExternal(href);
      }}
      className="text-muted hover:bg-surface-raised hover:text-text -mx-3 flex items-center gap-2 rounded-sm px-3 py-2 text-sm font-medium transition-colors"
    >
      {icon}
      {children}
    </a>
  );
}
