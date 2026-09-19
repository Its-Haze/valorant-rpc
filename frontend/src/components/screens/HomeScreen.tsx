import { useStatus } from "../../hooks/useStatus";
import { ConnectionDetail } from "./home/ConnectionDetail";
import { GithubCta } from "./home/GithubCta";
import { PresencePreview } from "./home/PresencePreview";

// The Home dashboard: the last-sent presence preview, what is and isn't
// connected behind it, and a closing GitHub star ask.
export function HomeScreen() {
  const status = useStatus();

  return (
    <div className="flex flex-col gap-6">
      <h1 className="text-xl font-semibold">Home</h1>
      <PresencePreview status={status} />
      <ConnectionDetail status={status} />
      <GithubCta />
    </div>
  );
}
