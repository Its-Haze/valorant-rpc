import { useEffect, useState } from "react";
import { GetDisplayPreview } from "../../../bindings/github.com/its-haze/valorant-rpc/cmd/valorant-rpc-gui/guiservice";
import type { Config, TemplatePair } from "../../../bindings/github.com/its-haze/valorant-rpc/internal/config/models";
import { withMatchImage, withShowKills, withShowRank, withShowStats } from "../../lib/displayPatch";
import { MATCH_IMAGE_OPTIONS } from "../../lib/matchImage";
import { DiscordPresenceCard } from "../DiscordPresenceCard";
import { Field, Select, Toggle } from "../ui";

export interface SettingsStepProps {
  cfg: Config;
  applyPatch: (patch: Partial<Config>) => Promise<void>;
}

interface Preview {
  details: string;
  state: string;
  largeImage?: string;
  smallImage?: string;
}

const EMPTY_PREVIEW: Preview = { details: "", state: "" };

// Onboarding's settings screen. Two preview cards, not one: the art and the
// match detail read differently in a match than they do in the client.
export function SettingsStep({ cfg, applyPatch }: SettingsStepProps) {
  const showRank = cfg.display.default.show_rank;
  const showStats = cfg.display.default.show_stats;
  const inMatchTemplate = cfg.presence.templates?.["in-match"] ?? { details: "", state: "" };
  const inClientTemplate = cfg.presence.templates?.["in-client"] ?? { details: "", state: "" };

  const matchImage = cfg.display.default.match_image;

  const inMatch = usePreview("in-match", inMatchTemplate, showRank, showStats, matchImage);
  const inClient = usePreview("in-client", inClientTemplate, showRank, showStats, matchImage);

  return (
    <div className="flex flex-col gap-4">
      <h1 className="text-2xl font-semibold">Pick what shows up</h1>
      <p className="text-muted text-base">
        These are the same toggles you'll find later under Display, where you can always change
        them again.
      </p>

      <section className="border-border bg-surface flex flex-col gap-1 rounded-lg border p-6">
        <Field id="onboarding-show-rank" label="Show rank" hint="Rank emblem and tier name">
          <Toggle
            id="onboarding-show-rank"
            checked={showRank}
            onCheckedChange={(v) => void applyPatch(withShowRank(cfg, v))}
            label="Show rank"
          />
        </Field>
        <Field
          id="onboarding-show-stats"
          label="Show match detail"
          hint="The round score while you're in a match"
        >
          <Toggle
            id="onboarding-show-stats"
            checked={showStats}
            onCheckedChange={(v) => void applyPatch(withShowStats(cfg, v))}
            label="Show match detail"
          />
        </Field>
        <Field
          id="onboarding-show-kills"
          label="Show kills in deathmatch"
          hint="Riot only refreshes the count every minute or so, so it runs behind the scoreboard"
        >
          <Toggle
            id="onboarding-show-kills"
            checked={cfg.display.default.show_kills}
            onCheckedChange={(v) => void applyPatch(withShowKills(cfg, v))}
            label="Show kills in deathmatch"
          />
        </Field>
        <Field
          id="onboarding-match-image"
          label="Picture during a match"
          hint="Your card is used anyway if the agent can't be read"
        >
          <Select
            value={cfg.display.default.match_image}
            onValueChange={(v) => void applyPatch(withMatchImage(cfg, v))}
            options={MATCH_IMAGE_OPTIONS}
            aria-label="Picture during a match"
          />
        </Field>
      </section>

      <div className="border-border flex flex-col gap-3 border-t pt-4">
        <div className="flex flex-col gap-2">
          <span className="text-muted text-xs font-semibold tracking-wide uppercase">In a match</span>
          <DiscordPresenceCard
            details={inMatch.details}
            state={inMatch.state}
            largeImage={inMatch.largeImage}
            smallImage={inMatch.smallImage}
          />
        </div>
        <div className="flex flex-col gap-2">
          <span className="text-muted text-xs font-semibold tracking-wide uppercase">In client</span>
          <DiscordPresenceCard
            details={inClient.details}
            state={inClient.state}
            largeImage={inClient.largeImage}
            smallImage={inClient.smallImage}
          />
        </div>
      </div>
    </div>
  );
}

// Renders one context through the same backend call the Display screen uses,
// so this walkthrough shows a real presence rather than a mock-up of one.
function usePreview(
  ctx: string,
  tmpl: TemplatePair,
  showRank: boolean,
  showStats: boolean,
  matchImage: string,
): Preview {
  const [preview, setPreview] = useState<Preview>(EMPTY_PREVIEW);
  // Depend on the two lines, not the pair: a context missing from the config
  // yields a fresh fallback object per render, which would refetch forever.
  const { details, state } = tmpl;

  useEffect(() => {
    let cancelled = false;
    GetDisplayPreview(ctx, { details, state }, showRank, showStats, matchImage)
      .then((p) => {
        if (!cancelled) {
          setPreview({
            details: p.details,
            state: p.state,
            largeImage: p.large_image,
            smallImage: p.small_image,
          });
        }
      })
      .catch(() => {});
    return () => {
      cancelled = true;
    };
  }, [ctx, details, state, showRank, showStats, matchImage]);

  return preview;
}
