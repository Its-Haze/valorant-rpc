import { useStatus } from "../../hooks/useStatus";
import { FeatureList } from "./home/FeatureList";
import { GithubCta } from "./home/GithubCta";
import { PresencePreview } from "./home/PresencePreview";

// The Home dashboard: the last-sent presence preview, what the app adds over
// Valorant's own status, and a closing GitHub star ask.
export function HomeScreen() {
  const status = useStatus();

  return (
    <div className="flex flex-col gap-6">
      <h1 className="text-xl font-semibold">Home</h1>
      <PresencePreview status={status} />
      <FeatureList />
      <GithubCta />
    </div>
  );
}
