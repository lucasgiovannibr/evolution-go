import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import {
  createInstance,
  deleteInstance,
  disconnectInstance,
  getInstance,
  listInstances,
  logoutInstance,
  saveIntegrations,
  updateBehavior,
  type IntegrationsInput,
  type CreateInstanceInput,
} from '@/api/instances';
import type { BehaviorSettings, Instance } from '@/api/types';

export const keys = {
  list: ['instances'] as const,
  one: (id: string) => ['instance', id] as const,
};

export function errMsg(err: unknown, fallback = 'Algo deu errado.'): string {
  return err instanceof Error && err.message ? err.message : fallback;
}

export function useInstances() {
  return useQuery({
    queryKey: keys.list,
    queryFn: listInstances,
    refetchInterval: 15_000,
    staleTime: 5_000,
  });
}

/** One instance, kept fresh. `fast` polls quickly (used while a connection is being established). */
export function useInstance(id: string | undefined, opts: { fast?: boolean } = {}) {
  const qc = useQueryClient();
  return useQuery({
    queryKey: keys.one(id ?? ''),
    queryFn: () => getInstance(id as string),
    enabled: !!id,
    refetchInterval: opts.fast ? 2_000 : 15_000,
    initialData: () => qc.getQueryData<Instance[]>(keys.list)?.find((i) => i.id === id),
    initialDataUpdatedAt: () => qc.getQueryState(keys.list)?.dataUpdatedAt,
  });
}

function useRefresh() {
  const qc = useQueryClient();
  return (id?: string) => {
    void qc.invalidateQueries({ queryKey: keys.list });
    if (id) void qc.invalidateQueries({ queryKey: keys.one(id) });
  };
}

export function useCreateInstance() {
  const refresh = useRefresh();
  return useMutation({
    mutationFn: (input: CreateInstanceInput) => createInstance(input),
    onSuccess: () => refresh(),
  });
}

export function useDeleteInstance() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (i: Instance) => deleteInstance(i.id),
    onSuccess: (_d, i) => {
      qc.removeQueries({ queryKey: keys.one(i.id) });
      void qc.invalidateQueries({ queryKey: keys.list });
    },
  });
}

export function useDisconnectInstance() {
  const refresh = useRefresh();
  return useMutation({
    mutationFn: (i: Instance) => disconnectInstance(i.token),
    onSuccess: (_d, i) => refresh(i.id),
  });
}

export function useLogoutInstance() {
  const refresh = useRefresh();
  return useMutation({
    mutationFn: (i: Instance) => logoutInstance(i.token),
    onSuccess: (_d, i) => refresh(i.id),
  });
}

export function useSaveConnection(i: Instance) {
  const refresh = useRefresh();
  return useMutation({
    mutationFn: (input: IntegrationsInput) => saveIntegrations(i.id, input),
    onSuccess: () => {
      toast.success('Webhook e eventos atualizados');
      refresh(i.id);
    },
    onError: (e) => toast.error('Não foi possível salvar', { description: errMsg(e) }),
  });
}

export function useSaveBehavior(i: Instance) {
  const refresh = useRefresh();
  return useMutation({
    mutationFn: (s: BehaviorSettings) => updateBehavior(i.id, s),
    onSuccess: () => {
      toast.success('Comportamento atualizado');
      refresh(i.id);
    },
    onError: (e) => toast.error('Não foi possível salvar', { description: errMsg(e) }),
  });
}
