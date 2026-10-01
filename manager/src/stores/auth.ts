import { create } from 'zustand';
import { createJSONStorage, persist } from 'zustand/middleware';

export type LicenseState = 'unchecked' | 'licensed' | 'unlicensed';

interface AuthState {
  apiUrl: string;
  apiKey: string;
  isAuthenticated: boolean;
  licenseState: LicenseState;
  setSession: (s: Partial<Pick<AuthState, 'apiUrl' | 'apiKey' | 'isAuthenticated' | 'licenseState'>>) => void;
  clear: () => void;
}

const defaultUrl = () => (typeof window !== 'undefined' ? window.location.origin : 'http://localhost:8080');

/**
 * The session lives in sessionStorage: it ends with the tab. It used to be kept in
 * localStorage, so the API key (the GLOBAL key, when an administrator logs in) stayed in the
 * browser for good and was readable by any script that ever ran on this origin. Whatever the
 * old versions left behind is removed.
 */
try {
  localStorage.removeItem('evolution-auth');
} catch {
  /* storage blocked: nothing to clean */
}

export const useAuth = create<AuthState>()(
  persist(
    (set) => ({
      apiUrl: defaultUrl(),
      apiKey: '',
      isAuthenticated: false,
      licenseState: 'unchecked',
      setSession: (s) => set(s),
      clear: () => set({ apiUrl: defaultUrl(), apiKey: '', isAuthenticated: false, licenseState: 'unchecked' }),
    }),
    {
      name: 'evolution-auth',
      storage: createJSONStorage(() => sessionStorage),
      partialize: (s) => ({
        apiUrl: s.apiUrl,
        apiKey: s.apiKey,
        isAuthenticated: s.isAuthenticated,
        licenseState: s.licenseState,
      }),
    },
  ),
);

export const isSignedIn = (s: Pick<AuthState, 'isAuthenticated' | 'licenseState'>) =>
  s.isAuthenticated && s.licenseState === 'licensed';
