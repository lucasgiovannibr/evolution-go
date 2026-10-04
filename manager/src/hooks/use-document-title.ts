import { useEffect } from 'react';

/** Sets the browser tab title ("Instâncias · WhatyGo"); restores the default on unmount. */
export function useDocumentTitle(title: string | undefined) {
  useEffect(() => {
    if (!title) return;
    document.title = `${title} · WhatyGo`;
    return () => {
      document.title = 'WhatyGo Manager';
    };
  }, [title]);
}
