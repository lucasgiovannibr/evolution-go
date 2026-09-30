import { useState } from 'react';

/**
 * Editable copy of server data. Background refetches replace the draft only while the user has
 * no unsaved edits, so polling never clobbers what is being typed.
 */
export function useDraft<T>(server: T) {
  const serverKey = JSON.stringify(server);
  const [draft, setDraft] = useState<T>(server);
  const [baseKey, setBaseKey] = useState(serverKey);

  if (serverKey !== baseKey) {
    if (JSON.stringify(draft) === baseKey) setDraft(server);
    setBaseKey(serverKey);
  }

  return {
    draft,
    setDraft,
    patch: (p: Partial<T>) => setDraft((d) => ({ ...d, ...p })),
    dirty: JSON.stringify(draft) !== serverKey,
    reset: () => setDraft(server),
  };
}
