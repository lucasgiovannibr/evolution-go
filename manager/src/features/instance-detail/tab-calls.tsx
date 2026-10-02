import { useCallback } from 'react';
import { useActiveCalls, useCallActions } from '@/hooks/use-calls';
import { DialCard, EngineCard, LiveCalls } from '@/features/calls/live-calls';
import { HistoryCard } from '@/features/calls/history-card';
import { PhonePanel } from '@/features/calls/phone-panel';
import { usePhone } from '@/features/calls/use-phone';
import { useInstanceContext } from './instance-page';

/** Calls of one instance: what rings now, answering and dialling from the browser, and the history. */
export function TabCalls() {
  const { instance } = useInstanceContext();
  const calls = useActiveCalls(instance);
  const { hangup, refresh } = useCallActions(instance);
  const onChange = useCallback(() => refresh(), [refresh]);
  const phone = usePhone(instance, onChange);

  const session = phone.session;
  const phoneBusy = !!session && session.state !== 'ended' && session.state !== 'error';
  const engineOn = calls.data?.enabled === true;

  return (
    <div className="space-y-4">
      <EngineCard instance={instance} calls={calls.data} />

      {session ? <PhonePanel session={session} onHangup={() => void phone.hangup()} onToggleMute={phone.toggleMute} onDismiss={phone.dismiss} /> : null}

      <div className="grid gap-4 lg:grid-cols-[2fr_1fr]">
        <LiveCalls
          calls={calls.data}
          loading={calls.isPending}
          browserCallId={phoneBusy ? (session?.callId ?? null) : null}
          phoneBusy={phoneBusy}
          onJoin={(c) => void phone.join(c)}
          onHangup={(c) => hangup.mutate(c.callId)}
        />
        <DialCard disabled={!engineOn || phoneBusy} busy={phoneBusy && !session?.callId} onDial={(n) => void phone.dial(n)} />
      </div>

      <HistoryCard instance={instance} />
    </div>
  );
}
