import { useEffect, useRef, useState, type InputHTMLAttributes } from "react";
import { useDebouncedValue } from "../../hooks/useDebouncedValue";

export interface DebouncedTextFieldProps
  extends Omit<InputHTMLAttributes<HTMLInputElement>, "value" | "onChange" | "onCommit"> {
  value: string;
  onCommit: (value: string) => void;
  delayMs?: number;
}

// A text input that only calls onCommit after typing pauses for delayMs, so
// a disk-writing/IPC-calling commit doesn't fire on every keystroke.
export function DebouncedTextField({ value, onCommit, delayMs = 400, ...inputProps }: DebouncedTextFieldProps) {
  const [draft, setDraft] = useState(value);
  // Tracks the last value *we* committed, so the round-tripped echo of our
  // own write doesn't clobber whatever the user has typed since.
  const lastSent = useRef(value);
  useEffect(() => {
    if (value !== lastSent.current) setDraft(value);
  }, [value]);

  const debounced = useDebouncedValue(draft, delayMs);
  const mounted = useRef(false);
  useEffect(() => {
    if (!mounted.current) {
      mounted.current = true;
      return;
    }
    lastSent.current = debounced;
    onCommit(debounced);
    // onCommit is expected to be stable enough per render; only the debounced
  }, [debounced]);

  // Read from the unmount cleanup, which runs once and would otherwise close
  // over the first render's values.
  const draftRef = useRef(draft);
  draftRef.current = draft;
  const onCommitRef = useRef(onCommit);
  onCommitRef.current = onCommit;

  // Navigating away unmounts this field and clears the pending debounce with
  // it, so an edit made in the last delayMs would vanish unsaved.
  useEffect(() => {
    return () => {
      if (draftRef.current !== lastSent.current) {
        lastSent.current = draftRef.current;
        onCommitRef.current(draftRef.current);
      }
    };
  }, []);

  return <input {...inputProps} value={draft} onChange={(e) => setDraft(e.target.value)} />;
}
