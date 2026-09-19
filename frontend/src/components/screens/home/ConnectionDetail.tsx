import { Plug } from "lucide-react";
import type { StatusSnapshot } from "../../../../bindings/github.com/its-haze/valorant-rpc/internal/app/models";
import { SettingsCard } from "../../ui";

export interface ConnectionDetailProps {
  status: StatusSnapshot | null;
}

interface Row {
  label: string;
  on: boolean;
  /** Shown instead of the plain off state, for a fault worth naming. */
  fault?: string;
}

// The three things that have to be true before a presence can go out, broken
// out so a missing status points at the one that isn't, not just "not working".
export function ConnectionDetail({ status }: ConnectionDetailProps) {
  const rows: Row[] = [
    { label: "Valorant", on: status?.game_running ?? false },
    {
      label: "Riot Client",
      on: (status?.riot_connected ?? false) && !status?.presence_stalled,
      fault: status?.presence_stalled ? "Connected, but no presence yet" : undefined,
    },
    { label: "Discord", on: status?.discord_connected ?? false },
  ];

  return (
    <SettingsCard
      icon={Plug}
      title="Connections"
      description="All three have to be up before anything reaches your Discord profile."
    >
      <dl className="flex flex-col gap-2 text-sm">
        {rows.map((row) => (
          <div key={row.label} className="flex items-center justify-between gap-4">
            <dt>{row.label}</dt>
            <dd className="flex items-center gap-2">
              <span className={`text-xs ${row.on ? "text-ok" : "text-muted"}`}>
                {row.fault ?? (row.on ? "Connected" : "Not connected")}
              </span>
              <span
                aria-hidden
                className={`size-2 rounded-full ${row.on ? "bg-ok" : row.fault ? "bg-warn" : "bg-muted"}`}
              />
            </dd>
          </div>
        ))}
      </dl>
    </SettingsCard>
  );
}
