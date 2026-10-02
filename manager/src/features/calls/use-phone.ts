import { useCallback, useEffect, useRef, useState } from 'react';
import { toast } from 'sonner';
import { answerCall, dialCall, getStreamTicket, hangupCall, type CallInfo } from '@/api/calls';
import type { Instance } from '@/api/types';
import { errMsg } from '@/hooks/use-instances';
import { useAuth } from '@/stores/auth';
import { STREAM_OPTIONS } from './audio';
import { streamWsUrl } from './format';
import { Softphone, type PhoneState } from './softphone';

export interface PhoneSession {
  callId: string | null;
  state: PhoneState;
  detail?: string;
  muted: boolean;
  mic: number;
  peer: number;
  peerSpeaking: boolean;
  /** When the audio went live, for the timer. */
  liveAt: number | null;
}

const idle: PhoneSession = { callId: null, state: 'idle', muted: false, mic: 0, peer: 0, peerSpeaking: false, liveAt: null };

/**
 * The browser phone of one instance: one call at a time, with the audio of the page. It is how
 * a person answers, dials and talks from the panel; anyone else uses the API and a stream of
 * their own. Leaving the page lets go of the audio, and a call without a stream is hung up
 * by the server after CALL_STREAM_GRACE.
 */
export function usePhone(instance: Instance, onChange: () => void) {
  const [session, setSession] = useState<PhoneSession | null>(null);
  const phone = useRef<Softphone | null>(null);
  const busy = useRef(false);
  const apiUrl = useAuth((s) => s.apiUrl);

  const patch = useCallback((p: Partial<PhoneSession>) => setSession((s) => (s ? { ...s, ...p } : s)), []);

  const open = useCallback(
    (callId: string | null): Softphone => {
      phone.current?.stop();
      const p = new Softphone({
        onState: (state, detail) => {
          setSession((s) => (s && phone.current === p ? { ...s, state, detail, liveAt: state === 'live' ? Date.now() : s.liveAt } : s));
          if (state === 'ended' || state === 'error') onChange();
        },
        onLevels: (mic, peer) => setSession((s) => (s && phone.current === p ? { ...s, mic, peer } : s)),
        onPeerSpeaking: (peerSpeaking) => setSession((s) => (s && phone.current === p ? { ...s, peerSpeaking } : s)),
      });
      phone.current = p;
      setSession({ ...idle, callId });
      return p;
    },
    [onChange],
  );

  // the audio belongs to this page
  useEffect(
    () => () => {
      phone.current?.stop();
      phone.current = null;
    },
    [],
  );

  const guard = useCallback(async (work: () => Promise<void>) => {
    if (busy.current) return;
    busy.current = true;
    try {
      await work();
    } finally {
      busy.current = false;
    }
  }, []);

  /** Connects the audio of the page to a call that is already there, answering it first when it rings for us. */
  const join = useCallback(
    (call: CallInfo) =>
      guard(async () => {
        const p = open(call.callId);
        if (!(await p.prepare())) return;
        try {
          const ticket = await getStreamTicket(instance.token, call.callId, STREAM_OPTIONS);
          // the stream is opened before answering: the audio of a call that is answered with none is lost
          if (!(await p.connect(streamWsUrl(apiUrl, ticket.path)))) return;
          if (call.direction === 'incoming' && call.phase === 'ringing') await answerCall(instance.token, call.callId);
          onChange();
        } catch (e) {
          p.stop();
          toast.error('Não foi possível entrar na chamada', { description: errMsg(e) });
        }
      }),
    [apiUrl, guard, instance.token, onChange, open],
  );

  /** Places a call and connects the audio before the phone is picked up. */
  const dial = useCallback(
    (number: string) =>
      guard(async () => {
        const p = open(null);
        if (!(await p.prepare())) return;
        try {
          const result = await dialCall(instance.token, number, STREAM_OPTIONS);
          patch({ callId: result.callId });
          if (!result.streamTicket) throw new Error('O servidor não devolveu o bilhete do áudio.');
          if (!(await p.connect(streamWsUrl(apiUrl, result.streamTicket.path)))) {
            await hangupCall(instance.token, result.callId).catch(() => undefined);
            return;
          }
          onChange();
        } catch (e) {
          p.stop();
          toast.error('Não foi possível ligar', { description: errMsg(e) });
        }
      }),
    [apiUrl, guard, instance.token, onChange, open, patch],
  );

  /** Hangs the call up (for both sides) and lets go of the audio. */
  const hangup = useCallback(async () => {
    const id = session?.callId;
    if (id) await hangupCall(instance.token, id).catch((e) => toast.error('Não foi possível encerrar', { description: errMsg(e) }));
    phone.current?.stop();
    onChange();
  }, [instance.token, onChange, session?.callId]);

  const toggleMute = useCallback(() => {
    const p = phone.current;
    if (!p) return;
    p.setMuted(!p.isMuted);
    patch({ muted: p.isMuted });
  }, [patch]);

  const dismiss = useCallback(() => {
    phone.current?.stop();
    phone.current = null;
    setSession(null);
  }, []);

  return { session, join, dial, hangup, toggleMute, dismiss };
}
