import { useEffect, useRef, useState } from "react";
import { Events } from "@wailsio/runtime";
import { GetRecentLogs } from "../../../../bindings/github.com/its-haze/valorant-rpc/cmd/valorant-rpc-gui/guiservice";
import {
  MAX_TAIL_LINES,
  appendLines,
  dropHistoryOverlap,
  isScrolledToBottom,
} from "../../../lib/logTail";

const LOG_LINE_EVENT = "log:line";
// Flushes buffered incoming lines on this cadence, so a burst of log:line
// events costs one re-render instead of one per line.
const FLUSH_INTERVAL_MS = 100;

// Live log viewer: fills from GetRecentLogs, then tails log:line events.
// Auto-scrolls only while the user is at (or near) the bottom already.
export function LogViewer() {
  const [lines, setLines] = useState<string[]>([]);
  const containerRef = useRef<HTMLDivElement>(null);
  const atBottomRef = useRef(true);
  const pendingRef = useRef<string[]>([]);
  const historyRef = useRef(false);

  useEffect(() => {
    // Lines keep buffering while this is in flight and the flush below holds
    // them back, so nothing written mid-fetch is lost or shown twice.
    GetRecentLogs()
      .then((l) => {
        const history = l ?? [];
        pendingRef.current = dropHistoryOverlap(history, pendingRef.current);
        setLines(history);
      })
      .catch(() => {})
      .finally(() => (historyRef.current = true));

    const off = Events.On(LOG_LINE_EVENT, (ev: { data: string }) => {
      // Capped, so a history call that never settles cannot grow this forever.
      if (pendingRef.current.length >= MAX_TAIL_LINES) pendingRef.current.shift();
      pendingRef.current.push(ev.data);
    });

    const flush = setInterval(() => {
      if (!historyRef.current || pendingRef.current.length === 0) return;
      const batch = pendingRef.current;
      pendingRef.current = [];
      setLines((prev) => appendLines(prev, batch));
    }, FLUSH_INTERVAL_MS);

    return () => {
      off();
      clearInterval(flush);
    };
  }, []);

  useEffect(() => {
    const el = containerRef.current;
    if (el && atBottomRef.current) {
      el.scrollTop = el.scrollHeight;
    }
  }, [lines]);

  function handleScroll() {
    const el = containerRef.current;
    if (!el) return;
    atBottomRef.current = isScrolledToBottom(el.scrollTop, el.clientHeight, el.scrollHeight);
  }

  return (
    <div
      ref={containerRef}
      onScroll={handleScroll}
      className="border-border bg-surface-raised h-72 overflow-y-auto rounded-md border p-3 font-mono text-xs"
    >
      {lines.length === 0 ? (
        <p className="text-muted">No log output yet.</p>
      ) : (
        lines.map((line, i) => <div key={i}>{line}</div>)
      )}
    </div>
  );
}
