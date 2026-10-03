import { useInfiniteQuery, useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import {
  answerCall,
  deleteCallHistory,
  getActiveCalls,
  getCallHistory,
  hangupCall,
  type CallDirection,
  type CallOutcome,
} from '@/api/calls';
import type { Instance } from '@/api/types';
import { errMsg } from '@/hooks/use-instances';

export const callKeys = {
  active: (id: string) => ['calls', 'active', id] as const,
  history: (id: string) => ['calls', 'history', id] as const,
};

/** The engine of the instance and its calls, kept fresh every two seconds while the page is open. */
export function useActiveCalls(instance: Instance) {
  return useQuery({
    queryKey: callKeys.active(instance.id),
    queryFn: () => getActiveCalls(instance.token),
    refetchInterval: 2_000,
    staleTime: 1_000,
  });
}

export function useCallActions(instance: Instance) {
  const qc = useQueryClient();
  const refresh = () => {
    void qc.invalidateQueries({ queryKey: callKeys.active(instance.id) });
    void qc.invalidateQueries({ queryKey: callKeys.history(instance.id) });
  };
  const answer = useMutation({
    mutationFn: (callId: string) => answerCall(instance.token, callId),
    onSuccess: refresh,
    onError: (e) => toast.error('Não foi possível atender', { description: errMsg(e) }),
  });
  const hangup = useMutation({
    mutationFn: (callId: string) => hangupCall(instance.token, callId),
    onSuccess: refresh,
    onError: (e) => toast.error('Não foi possível encerrar', { description: errMsg(e) }),
  });
  return { answer, hangup, refresh };
}

export interface HistoryFilters {
  direction: CallDirection | '';
  outcome: CallOutcome | '';
  peer: string;
}

/** The call history, page by page (the server pages by cursor). It answers 409 when the server keeps none. */
export function useCallHistory(instance: Instance, filters: HistoryFilters, enabled: boolean) {
  return useInfiniteQuery({
    queryKey: [...callKeys.history(instance.id), filters] as const,
    queryFn: ({ pageParam }) => getCallHistory(instance.token, { ...filters, limit: 20, cursor: pageParam }),
    initialPageParam: undefined as string | undefined,
    getNextPageParam: (last) => last.next,
    enabled,
    retry: false,
    staleTime: 10_000,
  });
}

export function useDeleteCallHistory(instance: Instance) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: () => deleteCallHistory(instance.token),
    onSuccess: (deleted) => {
      void qc.invalidateQueries({ queryKey: callKeys.history(instance.id) });
      toast.success(deleted === 1 ? '1 registro apagado' : `${deleted} registros apagados`);
    },
    onError: (e) => toast.error('Não foi possível apagar o histórico', { description: errMsg(e) }),
  });
}
