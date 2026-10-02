import { useState } from 'react';
import { Link } from 'react-router-dom';
import { ArrowDown, ArrowUp, Phone, PhoneCall, PhoneIncoming, PhoneOff, PhoneOutgoing, Video } from 'lucide-react';
import type { ActiveCalls, CallInfo } from '@/api/calls';
import type { Instance } from '@/api/types';
import { useNow } from '@/hooks/use-now';
import { Alert, EmptyState } from '@/components/ui/feedback';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardBody, CardHeader } from '@/components/ui/card';
import { Field, Input } from '@/components/ui/form';
import { elapsedSeconds, formatClock, peerLabel, PHASE_LABEL, PHASE_TONE } from './format';

/** Whether the call engine works on this instance, and what to do when it does not. */
export function EngineCard({ instance, calls }: { instance: Instance; calls: ActiveCalls | undefined }) {
  const settings = `/manager/instances/${instance.id}/behavior`;
  if (!calls) return null;

  if (calls.enabled) return null; // nothing to say while it works: the status is the list below

  if (calls.state === 'hook_failed') {
    return (
      <Alert tone="danger" title="O motor de chamadas não está funcionando">
        Ele não conseguiu se acoplar ao WhatsApp (em geral depois de uma atualização da biblioteca). As chamadas ficam sem áudio.
        {calls.error ? <span className="mt-1 block font-mono text-xs">{calls.error}</span> : null}
      </Alert>
    );
  }
  if (calls.state === 'blocked_by_proxy') {
    return (
      <Alert tone="warn" title="Esta instância usa proxy, então não faz chamadas">
        O áudio da chamada sai direto por UDP e ignoraria o proxy, o que mostraria o endereço real do servidor. Retire o proxy da instância para usar chamadas.
      </Alert>
    );
  }
  return (
    <Alert
      tone="info"
      title="As chamadas estão desligadas nesta instância"
      action={
        <Link to={settings} className="text-[13px] font-medium text-brand-text hover:underline">
          Abrir Comportamento
        </Link>
      }
    >
      {instance.behavior.callsEnabled
        ? 'Já estão ligadas na configuração, mas valem só na próxima conexão: desconecte e conecte a instância.'
        : 'Ligue "Chamadas" em Comportamento e reconecte a instância. Enquanto isso o painel só mostra o histórico, se o servidor o guarda.'}
    </Alert>
  );
}

function digitsOf(s: string) {
  return s.replace(/\D/g, '');
}

export function DialCard({ disabled, busy, onDial }: { disabled: boolean; busy: boolean; onDial: (number: string) => void }) {
  const [number, setNumber] = useState('');
  const digits = digitsOf(number);
  const valid = digits.length >= 10 && digits.length <= 15;

  return (
    <Card>
      <CardHeader title="Ligar" description="O telefone do número toca e o áudio é o desta página." icon={<PhoneOutgoing />} />
      <CardBody>
        <form
          className="flex flex-wrap items-end gap-2"
          onSubmit={(e) => {
            e.preventDefault();
            if (valid && !disabled) onDial(digits);
          }}
        >
          <Field label="Número com DDI e DDD" className="min-w-56 flex-1" hint={digits && !valid ? 'Digite o número inteiro, por exemplo 5511999990000.' : undefined}>
            {(id) => (
              <Input
                id={id}
                inputMode="tel"
                autoComplete="off"
                placeholder="5511999990000"
                value={number}
                onChange={(e) => setNumber(e.target.value)}
                leading={<Phone />}
                disabled={disabled}
              />
            )}
          </Field>
          <Button type="submit" variant="primary" disabled={disabled || !valid} loading={busy}>
            <PhoneCall className="size-4" />
            Ligar
          </Button>
        </form>
      </CardBody>
    </Card>
  );
}

function CallRow({
  call,
  now,
  busy,
  inBrowser,
  onJoin,
  onHangup,
}: {
  call: CallInfo;
  now: number;
  busy: boolean;
  inBrowser: boolean;
  onJoin: (c: CallInfo) => void;
  onHangup: (c: CallInfo) => void;
}) {
  const incomingRinging = call.direction === 'incoming' && call.phase === 'ringing';
  const Icon = call.direction === 'incoming' ? PhoneIncoming : PhoneOutgoing;
  const s = call.stream;

  return (
    <li className="flex flex-wrap items-center gap-x-4 gap-y-2 px-4 py-3">
      <span className="flex size-9 shrink-0 items-center justify-center rounded-full bg-surface-2 text-muted ring-1 ring-line [&_svg]:size-4">
        <Icon />
      </span>
      <div className="min-w-0 flex-1 basis-48">
        <p className="truncate text-[13px] font-medium">{peerLabel(call.peer)}</p>
        <p className="mt-0.5 flex flex-wrap items-center gap-1.5 text-xs text-muted">
          <Badge tone={PHASE_TONE[call.phase]} dot pulse={call.phase === 'ringing' || call.phase === 'calling'}>
            {PHASE_LABEL[call.phase]}
          </Badge>
          <span className="font-mono">{formatClock(elapsedSeconds(call.startedAt, now))}</span>
          {call.video ? (
            <Badge tone="info" icon={<Video />}>
              Vídeo
            </Badge>
          ) : null}
          {call.mediaStalled ? <Badge tone="warn">Sem áudio do outro lado</Badge> : null}
          {inBrowser ? <Badge tone="brand">No navegador</Badge> : null}
        </p>
      </div>
      {s?.attached ? (
        <p className="flex items-center gap-3 font-mono text-[11px] text-muted" title="Quadros de áudio de 60 ms: recebidos, enviados e descartados por lentidão">
          <span className="flex items-center gap-0.5">
            <ArrowDown className="size-3" />
            {s.toClient}
          </span>
          <span className="flex items-center gap-0.5">
            <ArrowUp className="size-3" />
            {s.fromClient}
          </span>
          {s.droppedToClient + s.droppedFromClient > 0 ? <span className="text-warn">−{s.droppedToClient + s.droppedFromClient}</span> : null}
        </p>
      ) : null}
      <div className="flex items-center gap-2">
        {!inBrowser ? (
          <Button size="sm" variant={incomingRinging ? 'primary' : 'secondary'} disabled={busy} onClick={() => onJoin(call)}>
            <PhoneCall className="size-3.5" />
            {incomingRinging ? 'Atender no navegador' : 'Falar pelo navegador'}
          </Button>
        ) : null}
        <Button size="sm" variant={incomingRinging ? 'secondary' : 'danger-soft'} onClick={() => onHangup(call)}>
          <PhoneOff className="size-3.5" />
          {incomingRinging ? 'Rejeitar' : 'Desligar'}
        </Button>
      </div>
    </li>
  );
}

interface LiveCallsProps {
  calls: ActiveCalls | undefined;
  loading: boolean;
  /** The call the audio of this page is on, if any. */
  browserCallId: string | null;
  /** The browser phone is in use: it takes one call at a time. */
  phoneBusy: boolean;
  onJoin: (c: CallInfo) => void;
  onHangup: (c: CallInfo) => void;
}

export function LiveCalls({ calls, loading, browserCallId, phoneBusy, onJoin, onHangup }: LiveCallsProps) {
  const now = useNow();
  const list = calls?.calls ?? [];

  return (
    <Card>
      <CardHeader
        title="Chamadas agora"
        description="As chamadas que o servidor acompanha nesta instância, atualizadas a cada 2 segundos."
        icon={<PhoneCall />}
        actions={list.length > 0 ? <Badge tone="brand">{list.length}</Badge> : null}
      />
      {list.length === 0 ? (
        <EmptyState
          className="py-10"
          icon={<Phone />}
          title={loading ? 'Carregando…' : 'Nenhuma chamada no momento'}
          description={calls?.enabled ? 'Quando alguém ligar para este número, a chamada aparece aqui para você atender.' : undefined}
        />
      ) : (
        <ul className="divide-y divide-line">
          {list.map((c) => (
            <CallRow key={c.callId} call={c} now={now} busy={phoneBusy} inBrowser={c.callId === browserCallId} onJoin={onJoin} onHangup={onHangup} />
          ))}
        </ul>
      )}
    </Card>
  );
}
