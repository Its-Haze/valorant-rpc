import { useEffect, useRef, useState } from "react";
import {
  GetDisplayPreview,
  GetTemplateTokens,
} from "../../../../bindings/github.com/its-haze/valorant-rpc/cmd/valorant-rpc-gui/guiservice";
import type { TemplatePair } from "../../../../bindings/github.com/its-haze/valorant-rpc/internal/config/models";
import { useDebouncedValue } from "../../../hooks/useDebouncedValue";
import type { PresenceContext } from "../../../lib/presenceContexts";
import { DiscordPresenceCard } from "../../DiscordPresenceCard";
import { Field } from "../../ui";

// How long to wait after the last keystroke before persisting the template
// and re-running the preview, so typing doesn't save/round-trip every key.
const COMMIT_DELAY_MS = 400;

// The preview as the backend renders it. The images come from Go too, so no
// TypeScript has to mirror the image choices in internal/discord/presence.go.
interface PreviewState {
  details: string;
  state: string;
  warnings: string[];
  largeImage?: string;
  smallImage?: string;
}

export interface TemplateEditorProps {
  ctx: PresenceContext;
  value: TemplatePair;
  onChange: (next: TemplatePair) => void;
  /** Current Display.Default toggles, so the preview honors them the same
   * way a real send would, in both the text and the art. */
  showRank: boolean;
  showStats: boolean;
  /** The built-in details/state pair for ctx, for the per-line reset button.
   * Undefined while defaults haven't loaded yet, which just hides the button. */
  defaultValue?: TemplatePair;
}

// One presence context's editor: details/state text fields, a live preview
// rendered through the real template engine, and a token reference.
export function TemplateEditor({ ctx, value, onChange, showRank, showStats, defaultValue }: TemplateEditorProps) {
  const [tokens, setTokens] = useState<string[]>([]);
  const [preview, setPreview] = useState<PreviewState>({ details: "", state: "", warnings: [] });

  // Local draft so typing feels instant; onChange (which persists to the
  // daemon) only fires once the draft settles, via the debounce below.
  const [draft, setDraft] = useState(value);
  // Tracks the last pair *we* committed, so the round-tripped echo of our
  // own write doesn't clobber whatever the user has typed since.
  const lastSent = useRef(value);
  useEffect(() => {
    if (value.details !== lastSent.current.details || value.state !== lastSent.current.state) {
      setDraft(value);
    }
  }, [value]);
  const debouncedDraft = useDebouncedValue(draft, COMMIT_DELAY_MS);

  // Read from the unmount cleanup, which runs once and would otherwise close
  // over the first render's values.
  const draftRef = useRef(draft);
  draftRef.current = draft;
  const onChangeRef = useRef(onChange);
  onChangeRef.current = onChange;

  useEffect(() => {
    GetTemplateTokens(ctx)
      .then((t) => setTokens(t ?? []))
      .catch(() => {});
  }, [ctx]);

  // Persist only when the text itself settles. Toggling a display switch also
  // re-runs the preview below, and committing from there would write the same
  // template again and race the toggle's own save.
  useEffect(() => {
    if (debouncedDraft.details === lastSent.current.details && debouncedDraft.state === lastSent.current.state) {
      return;
    }
    lastSent.current = debouncedDraft;
    onChange(debouncedDraft);
    // onChange is expected to be stable enough per render.
  }, [debouncedDraft]);

  // Switching tabs unmounts this editor and clears the pending debounce with
  // it, so an edit made in the last COMMIT_DELAY_MS would vanish unsaved.
  useEffect(() => {
    return () => {
      const pending = draftRef.current;
      if (pending.details !== lastSent.current.details || pending.state !== lastSent.current.state) {
        lastSent.current = pending;
        onChangeRef.current(pending);
      }
    };
  }, []);

  useEffect(() => {
    let cancelled = false;
    GetDisplayPreview(ctx, debouncedDraft, showRank, showStats)
      .then((p) => {
        if (!cancelled) {
          setPreview({
            details: p.details,
            state: p.state,
            warnings: p.warnings ?? [],
            largeImage: p.large_image,
            smallImage: p.small_image,
          });
        }
      })
      .catch(() => {});
    return () => {
      cancelled = true;
    };
  }, [ctx, debouncedDraft, showRank, showStats]);

  return (
    <div className="flex flex-col gap-4">
      <div className="text-muted text-xs">
        Available tokens: {tokens.length > 0 ? tokens.map((t) => `{${t}}`).join(" ") : "none"}
      </div>

      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
        <Field
          id={`${ctx}-details`}
          label="Details line"
          stacked
          onReset={defaultValue ? () => setDraft({ ...draft, details: defaultValue.details }) : undefined}
          isDefault={!defaultValue || draft.details === defaultValue.details}
        >
          <input
            id={`${ctx}-details`}
            value={draft.details}
            onChange={(e) => setDraft({ ...draft, details: e.target.value })}
            placeholder="blank uses the built-in default"
            className="border-border bg-surface-raised text-text w-full rounded-sm border px-3 py-1.5 text-sm"
          />
        </Field>
        <Field
          id={`${ctx}-state`}
          label="State line"
          stacked
          onReset={defaultValue ? () => setDraft({ ...draft, state: defaultValue.state }) : undefined}
          isDefault={!defaultValue || draft.state === defaultValue.state}
        >
          <input
            id={`${ctx}-state`}
            value={draft.state}
            onChange={(e) => setDraft({ ...draft, state: e.target.value })}
            placeholder="blank uses the built-in default"
            className="border-border bg-surface-raised text-text w-full rounded-sm border px-3 py-1.5 text-sm"
          />
        </Field>
      </div>

      <div className="border-border flex flex-col gap-2 border-t pt-4">
        <span className="text-muted text-xs font-semibold tracking-wide uppercase">Preview</span>
        <DiscordPresenceCard
          details={preview.details}
          state={preview.state}
          largeImage={preview.largeImage}
          smallImage={preview.smallImage}
        />
        {preview.warnings.length > 0 && (
          <ul className="text-danger text-xs">
            {preview.warnings.map((w) => (
              <li key={w}>{w}</li>
            ))}
          </ul>
        )}
      </div>
    </div>
  );
}
