import { AppWindow, Bug, Gauge, RotateCcw } from "lucide-react";
import { useEffect, useState } from "react";
import { useConfigBounds } from "../../hooks/useConfigBounds";
import { useDefaultConfig } from "../../hooks/useDefaultConfig";
import { useDiscordAppName } from "../../hooks/useDiscordAppName";
import { useSettings } from "../../hooks/useSettings";
import { formatIntervalSeconds, type Bounds } from "../../lib/advancedBounds";
import { DISCORD_DEVELOPER_PORTAL_URL, openExternal } from "../../lib/links";
import { DebouncedTextField, Field, Select, SettingsCard, Toggle } from "../ui";

// The two ways the presence can appear on a profile. Custom is a UI mode
// rather than a stored value; the config only ever holds an application ID.
const APP_ID_DEFAULT = "default";
const APP_ID_CUSTOM = "custom";

const APP_ID_OPTIONS = [
  { value: APP_ID_DEFAULT, label: "Valorant" },
  { value: APP_ID_CUSTOM, label: "My own Discord app" },
];

// The Advanced section: the Discord Application ID, the update interval
// clamped to the config package's bounds, and debug logging.
export function AdvancedScreen() {
  const { cfg, error, applyPatch } = useSettings();
  const defaults = useDefaultConfig();
  const bounds = useConfigBounds();
  // null means "untouched this session"; once the user types, even an empty
  // string sticks, so clearing the field to retype it doesn't snap back.
  const [draftAppId, setDraftAppId] = useState<string | null>(null);
  // Sticky once chosen, so selecting Custom does not snap back to Valorant
  // while the id still equals the default.
  const [customAppId, setCustomAppId] = useState(false);

  // Every hook must run before the "still loading" early return below, so
  // this falls back to a blank value until cfg resolves.
  const appId = draftAppId ?? cfg?.discord_app_id ?? "";
  const resolvedName = useDiscordAppName(appId.trim());

  if (!cfg) {
    return <p className="text-muted text-sm">Loading settings…</p>;
  }

  const appIdInvalid = appId.trim() === "";
  // Custom is a UI mode, not a stored setting: an id that is not the built-in
  // default is custom by definition, and picking Custom keeps the field open
  // while the user types even before it differs.
  const isCustomAppId = customAppId || (!!defaults && cfg.discord_app_id !== defaults.discord_app_id);

  function handleAppIdMode(mode: string) {
    if (mode === APP_ID_CUSTOM) {
      setCustomAppId(true);
      return;
    }
    setCustomAppId(false);
    handleAppIdReset();
  }

  function handleAppIdCommit(value: string) {
    setDraftAppId(value);
    if (value.trim() !== "") void applyPatch({ discord_app_id: value.trim() });
  }

  function handleAppIdReset() {
    if (!defaults) return;
    setDraftAppId(defaults.discord_app_id);
    if (defaults.discord_app_id !== cfg!.discord_app_id) void applyPatch({ discord_app_id: defaults.discord_app_id });
  }

  return (
    <div className="flex flex-col gap-6">
      <h1 className="text-xl font-semibold">Advanced</h1>
      {error && <p className="text-danger text-sm">{error}</p>}

      <SettingsCard
        icon={AppWindow}
        title="Discord application"
        description="Which app your presence appears under, including its name and icon on your profile."
      >
        <Field
          id="app-id-mode"
          label="Appears as"
          hint="Leave this on Valorant unless you want your presence to appear under your own Discord app"
          onReset={defaults && isCustomAppId ? () => handleAppIdMode(APP_ID_DEFAULT) : undefined}
          isDefault={!isCustomAppId}
        >
          <Select
            value={isCustomAppId ? APP_ID_CUSTOM : APP_ID_DEFAULT}
            onValueChange={handleAppIdMode}
            options={APP_ID_OPTIONS}
            aria-label="Appears as"
          />
        </Field>
        {isCustomAppId && (
          <>
            <Field id="app-id" label="Application ID" hint="The ID from your own Discord application">
              <DebouncedTextField
                id="app-id"
                value={appId}
                onCommit={handleAppIdCommit}
                className="border-border bg-surface-raised text-text rounded-sm border px-3 py-1.5 text-sm"
                aria-invalid={appIdInvalid}
              />
            </Field>
            {resolvedName && (
              <p className="text-muted text-right text-xs">
                Resolves to <span className="text-text font-medium">{resolvedName}</span>
              </p>
            )}
            {appIdInvalid && <p className="text-danger text-xs">Discord Application ID must not be empty.</p>}
            <CustomAppIdTutorial />
          </>
        )}
      </SettingsCard>

      <SettingsCard
        icon={Gauge}
        title="Update speed"
        description="How often Valorant RPC refreshes what Discord shows. Faster feels more live, slower is lighter on your PC."
      >
        <div className="flex flex-col gap-4 pt-1">
          <IntervalSlider
            id="update-interval"
            label="Discord status"
            description="How quickly your status updates when your queue, agent, or match changes."
            bounds={bounds.updateInterval}
            value={cfg.advanced.update_interval}
            defaultValue={defaults?.advanced.update_interval}
            onCommit={(v) => void applyPatch({ advanced: { ...cfg.advanced, update_interval: v } })}
          />
        </div>
      </SettingsCard>

      <SettingsCard
        icon={Bug}
        title="Debug logging"
        description="Writes far more detail to the log file, for when you're chasing a problem. Takes effect immediately."
        onReset={
          defaults ? () => void applyPatch({ advanced: { ...cfg.advanced, debug_mode: defaults.advanced.debug_mode } }) : undefined
        }
        isDefault={!defaults || cfg.advanced.debug_mode === defaults.advanced.debug_mode}
        action={
          <Toggle
            id="debug-mode"
            checked={cfg.advanced.debug_mode}
            onCheckedChange={(v) => void applyPatch({ advanced: { ...cfg.advanced, debug_mode: v } })}
            label="Debug logging"
          />
        }
      />
    </div>
  );
}

// A collapsed-by-default walkthrough for getting a custom Discord
// Application ID, for users who don't already have one lying around.
function CustomAppIdTutorial() {
  return (
    <details className="text-muted text-xs">
      <summary className="text-muted hover:text-text select-none font-medium">
        How do I get a custom Application ID?
      </summary>
      <ol className="mt-2 list-decimal space-y-1 pl-4">
        <li>
          Open the{" "}
          <a
            href={DISCORD_DEVELOPER_PORTAL_URL}
            onClick={(e) => {
              e.preventDefault();
              openExternal(DISCORD_DEVELOPER_PORTAL_URL);
            }}
            className="text-accent underline"
          >
            Discord Developer Portal
          </a>{" "}
          and sign in with your Discord account.
        </li>
        <li>Click "New Application", give it a name, and create it.</li>
        <li>On the app's "General Information" page, copy the "Application ID".</li>
        <li>Paste that ID into the field above.</li>
      </ol>
    </details>
  );
}

interface IntervalSliderProps {
  id: string;
  label: string;
  description: string;
  bounds: Bounds;
  value: number;
  /** The built-in default in ms, for the reset button. Undefined while
   * defaults haven't loaded yet, which just hides the button. */
  defaultValue?: number;
  onCommit: (ms: number) => void;
}

// A plain-language slider for a millisecond interval: shows seconds instead
// of raw milliseconds, and only persists once the drag or keypress ends.
function IntervalSlider({ id, label, description, bounds, value, defaultValue, onCommit }: IntervalSliderProps) {
  const [draft, setDraft] = useState(value);

  useEffect(() => setDraft(value), [value]);

  function commit() {
    if (draft !== value) onCommit(draft);
  }

  function reset() {
    if (defaultValue === undefined) return;
    setDraft(defaultValue);
    if (defaultValue !== value) onCommit(defaultValue);
  }

  const isDefault = defaultValue === undefined || draft === defaultValue;

  return (
    <div className="flex flex-col gap-1">
      <div className="flex items-center justify-between gap-4">
        <label htmlFor={id} className="text-sm">
          {label}
        </label>
        <div className="flex items-center gap-2">
          <span className="text-accent text-sm font-medium">{formatIntervalSeconds(draft)}</span>
          <button
            type="button"
            onClick={reset}
            title="Reset to default"
            aria-label={`Reset ${label} to default`}
            disabled={isDefault}
            className="text-muted hover:text-text shrink-0 rounded-sm p-1 disabled:pointer-events-none disabled:opacity-0"
          >
            <RotateCcw className="size-3.5" />
          </button>
        </div>
      </div>
      <p className="text-muted text-xs">{description}</p>
      <input
        id={id}
        type="range"
        min={bounds.min}
        max={bounds.max}
        step={100}
        value={draft}
        onChange={(e) => setDraft(Number(e.target.value))}
        onPointerUp={commit}
        onKeyUp={commit}
        className="accent-accent w-full"
      />
      <div className="text-muted flex justify-between text-[11px]">
        <span>Faster</span>
        <span>Slower</span>
      </div>
    </div>
  );
}
