import { useQuery } from '@tanstack/react-query';
import { fetchHealth } from '@/api/session';
import type { HealthStatus } from '@/api/types';

export function useHealth() {
  return useQuery({
    queryKey: ['health'],
    queryFn: ({ signal }) => fetchHealth(signal),
    refetchInterval: 30_000,
    staleTime: 15_000,
  });
}

export const HEALTH_META: Record<HealthStatus, { label: string; detail: string; tone: 'ok' | 'warn' | 'danger' }> = {
  ok: { label: 'Operacional', detail: 'Bancos de dados respondendo', tone: 'ok' },
  degraded: { label: 'Degradado', detail: 'Um banco de dados está lento', tone: 'warn' },
  unavailable: { label: 'Indisponível', detail: 'Falha em um banco de dados', tone: 'danger' },
  unreachable: { label: 'Sem conexão', detail: 'Servidor sem resposta', tone: 'danger' },
};
