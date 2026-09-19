import { Eye, Languages, MessageSquareText } from "lucide-react";
import type { TemplatePair } from "../../../bindings/github.com/its-haze/valorant-rpc/internal/config/models";
import { useDefaultConfig } from "../../hooks/useDefaultConfig";
import { useLocales } from "../../hooks/useLocales";
import { useSettings } from "../../hooks/useSettings";
import { useStatus } from "../../hooks/useStatus";
import { withLocale, withMatchImage, withShowInClient, withShowRank, withShowStats } from "../../lib/displayPatch";
import { MATCH_IMAGE_OPTIONS } from "../../lib/matchImage";
import { LOCALE_AUTO, autoLocaleLabel } from "../../lib/locales";
import { PRESENCE_CONTEXT_LABELS, PRESENCE_CONTEXTS } from "../../lib/presenceContexts";
import { Field, Select, SettingsCard, Tabs, Toggle } from "../ui";
import { TemplateEditor } from "./display/TemplateEditor";

// The Display section: global toggles, the name language, and one tab per
// presence context so the five text editors don't all show at once.
export function DisplayScreen() {
  const { cfg, error, applyPatch } = useSettings();
  const defaults = useDefaultConfig();
  const locales = useLocales();
  const status = useStatus();

  if (!cfg) {
    return <p className="text-muted text-sm">Loading settings…</p>;
  }

  function setTemplate(ctx: string, next: TemplatePair) {
    void applyPatch({
      presence: { ...cfg!.presence, templates: { ...cfg!.presence.templates, [ctx]: next } },
    });
  }

  const localeOptions = [
    { value: LOCALE_AUTO, label: autoLocaleLabel(status?.auto_locale ?? "", locales) },
    ...locales.map((l) => ({ value: l.tag, label: l.name })),
  ];

  return (
    <div className="flex flex-col gap-6">
      <h1 className="text-xl font-semibold">Display</h1>
      {error && <p className="text-danger text-sm">{error}</p>}

      <SettingsCard
        icon={Eye}
        title="What your status shows"
        description="The extras Valorant RPC adds on top of your agent and map."
      >
        <Field
          id="show-rank"
          label="Show rank"
          hint="Rank emblem and tier name"
          onReset={defaults ? () => void applyPatch(withShowRank(cfg, defaults.display.default.show_rank)) : undefined}
          isDefault={!defaults || cfg.display.default.show_rank === defaults.display.default.show_rank}
        >
          <Toggle
            id="show-rank"
            checked={cfg.display.default.show_rank}
            onCheckedChange={(v) => void applyPatch(withShowRank(cfg, v))}
            label="Show rank"
          />
        </Field>
        <Field
          id="show-stats"
          label="Show match detail"
          hint="The round score while you're in a match"
          onReset={defaults ? () => void applyPatch(withShowStats(cfg, defaults.display.default.show_stats)) : undefined}
          isDefault={!defaults || cfg.display.default.show_stats === defaults.display.default.show_stats}
        >
          <Toggle
            id="show-stats"
            checked={cfg.display.default.show_stats}
            onCheckedChange={(v) => void applyPatch(withShowStats(cfg, v))}
            label="Show match detail"
          />
        </Field>
        <Field
          id="match-image"
          label="Picture during a match"
          hint="Your card is used anyway if the agent can't be read"
          onReset={
            defaults
              ? () => void applyPatch(withMatchImage(cfg, defaults.display.default.match_image))
              : undefined
          }
          isDefault={!defaults || cfg.display.default.match_image === defaults.display.default.match_image}
        >
          <Select
            value={cfg.display.default.match_image}
            onValueChange={(v) => void applyPatch(withMatchImage(cfg, v))}
            options={MATCH_IMAGE_OPTIONS}
            aria-label="Picture during a match"
          />
        </Field>
        <Field
          id="show-in-client"
          label="Show presence while in client"
          hint="Keeps your status up between matches, not only during one"
          onReset={defaults ? () => void applyPatch(withShowInClient(cfg, defaults.presence.show_in_client)) : undefined}
          isDefault={!defaults || cfg.presence.show_in_client === defaults.presence.show_in_client}
        >
          <Toggle
            id="show-in-client"
            checked={cfg.presence.show_in_client}
            onCheckedChange={(v) => void applyPatch(withShowInClient(cfg, v))}
            label="Show presence while in client"
          />
        </Field>
      </SettingsCard>

      <SettingsCard
        icon={Languages}
        title="Language"
        description="What language agent, map and rank names appear in. Changing it takes effect on your next status update."
      >
        <Field
          id="locale"
          label="Name language"
          hint="Automatic follows whatever language your Riot Client is running in"
          onReset={defaults ? () => void applyPatch(withLocale(cfg, defaults.display.locale)) : undefined}
          isDefault={!defaults || cfg.display.locale === defaults.display.locale}
        >
          <Select
            value={cfg.display.locale}
            onValueChange={(v) => void applyPatch(withLocale(cfg, v))}
            options={localeOptions}
            aria-label="Name language"
          />
        </Field>
      </SettingsCard>

      <SettingsCard
        icon={MessageSquareText}
        title="Presence text"
        description="Write your own wording for each situation, or keep the defaults."
      >
        <Tabs
          defaultValue={PRESENCE_CONTEXTS[0]}
          items={PRESENCE_CONTEXTS.map((ctx) => {
            const pair = cfg.presence.templates?.[ctx] ?? { details: "", state: "" };
            return {
              value: ctx,
              label: PRESENCE_CONTEXT_LABELS[ctx],
              content: (
                <TemplateEditor
                  ctx={ctx}
                  value={pair}
                  onChange={(next) => setTemplate(ctx, next)}
                  showRank={cfg.display.default.show_rank}
                  showStats={cfg.display.default.show_stats}
                  defaultValue={defaults?.presence.templates?.[ctx]}
                />
              ),
            };
          })}
        />
      </SettingsCard>
    </div>
  );
}
