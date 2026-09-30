import { create } from 'zustand';
import { persist } from 'zustand/middleware';

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
 * Persisted under the same key and shape the previous manager used, so an existing
 * browser session keeps working after the upgrade.
 */
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
