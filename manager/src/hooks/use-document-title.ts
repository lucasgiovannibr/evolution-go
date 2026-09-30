import { useEffect } from 'react';

/** Sets the browser tab title ("Instâncias · Evolution GO"); restores the default on unmount. */
export function useDocumentTitle(title: string | undefined) {
  useEffect(() => {
    if (!title) return;
    document.title = `${title} · Evolution GO`;
    return () => {
      document.title = 'Evolution GO Manager';
    };
  }, [title]);
}
