import { api, ApiError, rawRequest } from '@/lib/http';
import type { HealthStatus } from './types';

/** Session + license calls. They run before the store is populated, so they pass url/key explicitly. */

export interface LicenseStatus {
  status: 'active' | 'inactive';
  instance_id?: string;
  api_key?: string;
}

export async function fetchLicenseStatus(baseUrl: string, apikey: string): Promise<LicenseStatus> {
  return api<LicenseStatus>('/license/status', { baseUrl, apikey });
}

export async function startLicenseRegistration(baseUrl: string, apikey: string, redirectUri: string) {
  return api<{ register_url?: string; message?: string }>('/license/register', {
    baseUrl,
    apikey,
    query: { redirect_uri: redirectUri },
  });
}

export async function activateLicense(baseUrl: string, apikey: string, code: string) {
  return api<{ status?: string; message?: string }>('/license/activate', { baseUrl, apikey, query: { code } });
}

/** Validates the key against an authenticated endpoint. */
export async function verifyApiKey(baseUrl: string, apikey: string): Promise<void> {
  try {
    await api('/instance/all', { baseUrl, apikey, query: { t: Date.now() } });
  } catch (err) {
    if (err instanceof ApiError && (err.status === 401 || err.status === 403)) {
      throw new Error('API Key inválida. Verifique a chave informada.');
    }
    throw new Error('Não foi possível conectar. Verifique a URL e a API Key.');
  }
}

export async function fetchHealth(signal?: AbortSignal): Promise<HealthStatus> {
  try {
    const res = await rawRequest('/health', { apikey: null, signal, timeoutMs: 6000 });
    const status = (res.data as { status?: string } | null)?.status;
    if (status === 'ok' || status === 'degraded' || status === 'unavailable') return status;
    return res.status >= 500 ? 'unavailable' : 'ok';
  } catch {
    return 'unreachable';
  }
}
