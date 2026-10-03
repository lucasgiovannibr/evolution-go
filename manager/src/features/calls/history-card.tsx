import { useState } from 'react';
import { History, PhoneIncoming, PhoneOutgoing, Search, Trash2, Video } from 'lucide-react';
import type { CallDirection, CallOutcome, CallRecord } from '@/api/calls';
import type { Instance } from '@/api/types';
import { ApiError } from '@/lib/http';
import { formatDateTime, timeAgo } from '@/lib/format';
import { useCallHistory, useDeleteCallHistory, type HistoryFilters } from '@/hooks/use-calls';
import { useDebounced } from '@/hooks/use-now';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardHeader } from '@/components/ui/card';
import { ConfirmDialog } from '@/components/ui/dialog';
import { EmptyState, Skeleton } from '@/components/ui/feedback';
import { Input, Select } from '@/components/ui/form';
import { DIRECTION_LABEL, formatClock, OUTCOME_LABEL, OUTCOME_TONE, peerLabel, reasonLabel } from './format';

const OUTCOMES = Object.keys(OUTCOME_LABEL) as CallOutcome[];

function Row({ r }: { r: CallRecord }) {
  const Icon = r.direction === 'incoming' ? PhoneIncoming : PhoneOutgoing;
  const seconds = r.outcome === 'answered' ? r.talkSeconds : r.ringSeconds;
  return (
    <tr className="border-t border-line align-middle">
      <td className="px-4 py-2.5 whitespace-nowrap" title={formatDateTime(r.startedAt)}>
        <span className="text-[13px]">{timeAgo(r.startedAt)}</span>
        <span className="block text-[11px] text-muted">{formatDateTime(r.startedAt)}</span>
      </td>
      <td className="px-4 py-2.5">
        <span className="flex items-center gap-2 text-[13px] [&_svg]:size-3.5 [&_svg]:text-muted">
          <Icon aria-label={DIRECTION_LABEL[r.direction]} />
          <span className="min-w-0">
            <span className="block truncate font-medium">{peerLabel(r.peer, r.peerPhone)}</span>
            <span className="block truncate font-mono text-[11px] text-muted">{r.peer}</span>
          </span>
          {r.video ? <Video aria-label="Com vídeo" /> : null}
        </span>
      </td>
      <td className="px-4 py-2.5">
        <Badge tone={OUTCOME_TONE[r.outcome]}>{OUTCOME_LABEL[r.outcome] ?? r.outcome}</Badge>
      </td>
      <td className="px-4 py-2.5 font-mono text-[13px] whitespace-nowrap" title={r.outcome === 'answered' ? 'Tempo de conversa' : 'Tempo tocando'}>
        {formatClock(seconds)}
      </td>
      <td className="px-4 py-2.5 text-xs text-muted" title={r.reason}>
        {reasonLabel(r.reason)}
      </td>
    </tr>
  );
}

export function HistoryCard({ instance }: { instance: Instance }) {
  const [direction, setDirection] = useState<CallDirection | ''>('');
  const [outcome, setOutcome] = useState<CallOutcome | ''>('');
  const [peer, setPeer] = useState('');
  const [confirming, setConfirming] = useState(false);
  const filters: HistoryFilters = { direction, outcome, peer: useDebounced(peer.trim()) };

  const history = useCallHistory(instance, filters, true);
  const wipe = useDeleteCallHistory(instance);

  const disabled = history.error instanceof ApiError && history.error.status === 409;
  const records = history.data?.pages.flatMap((p) => p.records) ?? [];
  const filtered = !!(direction || outcome || filters.peer);

  return (
    <Card>
      <CardHeader
        title="Histórico"
        description="Quem ligou, quando e como terminou. Só metadados: o áudio das chamadas nunca é guardado."
        icon={<History />}
        actions={
          !disabled && records.length > 0 ? (
            <Button size="sm" variant="danger-soft" onClick={() => setConfirming(true)}>
              <Trash2 className="size-3.5" />
              Apagar tudo
            </Button>
          ) : null
        }
      />

      {disabled ? (
        <EmptyState
          icon={<History />}
          title="O servidor não guarda o histórico de chamadas"
          description={
            <>
              Ligue com <span className="font-mono">CALL_HISTORY=true</span> nas variáveis do servidor e reinicie. Os registros expiram depois de{' '}
              <span className="font-mono">CALL_HISTORY_RETENTION_DAYS</span> dias (padrão 90).
            </>
          }
        />
      ) : (
        <>
          <div className="flex flex-wrap items-center gap-2 border-b border-line px-4 py-3">
            <Select aria-label="Direção" value={direction} onChange={(e) => setDirection(e.target.value as CallDirection | '')} className="w-36">
              <option value="">Todas</option>
              <option value="incoming">Recebidas</option>
              <option value="outgoing">Discadas</option>
            </Select>
            <Select aria-label="Resultado" value={outcome} onChange={(e) => setOutcome(e.target.value as CallOutcome | '')} className="w-40">
              <option value="">Todos os resultados</option>
              {OUTCOMES.map((o) => (
                <option key={o} value={o}>
                  {OUTCOME_LABEL[o]}
                </option>
              ))}
            </Select>
            <Input aria-label="Contato" className="min-w-44 flex-1" placeholder="Telefone ou JID" value={peer} onChange={(e) => setPeer(e.target.value)} leading={<Search />} />
          </div>

          {history.isPending ? (
            <div className="space-y-2 p-4">
              <Skeleton className="h-10 w-full" />
              <Skeleton className="h-10 w-full" />
              <Skeleton className="h-10 w-full" />
            </div>
          ) : history.isError ? (
            <EmptyState icon={<History />} title="Não foi possível carregar o histórico" description={history.error.message} action={<Button onClick={() => void history.refetch()}>Tentar de novo</Button>} />
          ) : records.length === 0 ? (
            <EmptyState
              className="py-10"
              icon={<History />}
              title={filtered ? 'Nenhuma chamada com esses filtros' : 'Nenhuma chamada registrada ainda'}
              description={filtered ? undefined : 'As chamadas aparecem aqui quando terminam.'}
            />
          ) : (
            <>
              <div className="overflow-x-auto">
                <table className="w-full min-w-[640px] text-left">
                  <thead>
                    <tr className="text-[11px] tracking-wide text-muted uppercase">
                      <th className="px-4 py-2 font-medium">Quando</th>
                      <th className="px-4 py-2 font-medium">Contato</th>
                      <th className="px-4 py-2 font-medium">Resultado</th>
                      <th className="px-4 py-2 font-medium">Duração</th>
                      <th className="px-4 py-2 font-medium">Como terminou</th>
                    </tr>
                  </thead>
                  <tbody>
                    {records.map((r) => (
                      <Row key={r.id} r={r} />
                    ))}
                  </tbody>
                </table>
              </div>
              {history.hasNextPage ? (
                <div className="border-t border-line p-3 text-center">
                  <Button size="sm" loading={history.isFetchingNextPage} onClick={() => void history.fetchNextPage()}>
                    Carregar mais
                  </Button>
                </div>
              ) : null}
            </>
          )}
        </>
      )}

      <ConfirmDialog
        open={confirming}
        onClose={() => setConfirming(false)}
        onConfirm={() => wipe.mutate(undefined, { onSettled: () => setConfirming(false) })}
        loading={wipe.isPending}
        title="Apagar o histórico de chamadas?"
        description="Todos os registros desta instância serão apagados do servidor. Não dá para desfazer."
        confirmLabel="Apagar tudo"
      />
    </Card>
  );
}
