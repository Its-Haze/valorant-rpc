import { Check, Sparkles } from "lucide-react";
import { SettingsCard } from "../../ui";

interface Feature {
  title: string;
  detail: string;
}

// What this app puts on a profile that Valorant's own status does not. Every
// line here is something the app actually does today.
const FEATURES: Feature[] = [
  {
    title: "The agent you're playing",
    detail: "Their portrait fills your status while the match runs, read from Valorant on your own PC.",
  },
  {
    title: "Your player card",
    detail: "The art you picked in game shows between matches, instead of a generic logo.",
  },
  {
    title: "Your rank",
    detail: "The emblem sits beside your status in competitive, with the tier on hover.",
  },
  {
    title: "Map, mode and score",
    detail: "Which map, which queue, and how the rounds are going.",
  },
  {
    title: "Your own wording",
    detail: "Every line is editable per situation, from the lobby to the last round.",
  },
];

// The Home dashboard's pitch: what the app adds, for someone deciding whether
// to keep it running.
export function FeatureList() {
  return (
    <SettingsCard
      icon={Sparkles}
      title="What this adds"
      description="Valorant tells Discord you're playing. This fills in the rest."
    >
      <dl className="flex flex-col gap-3 pt-1">
        {FEATURES.map((feature) => (
          <div key={feature.title} className="flex items-start gap-3">
            <Check className="text-ok mt-0.5 size-4 shrink-0" aria-hidden />
            <div className="flex flex-col gap-0.5">
              <dt className="text-sm font-medium">{feature.title}</dt>
              <dd className="text-muted text-xs leading-relaxed">{feature.detail}</dd>
            </div>
          </div>
        ))}
      </dl>
    </SettingsCard>
  );
}
