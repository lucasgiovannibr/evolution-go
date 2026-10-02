import { useCallback, useEffect, useRef, useState } from 'react';
import { toast } from 'sonner';
import { answerCall, dialCall, getStreamTicket, hangupCall, type CallInfo } from '@/api/calls';
import type { Instance } from '@/api/types';
import { errMsg } from '@/hooks/use-instances';
import { useAuth } from '@/stores/auth';
import { streamOptions } from './audio';
import { streamWsUrl } from './format';
import { Softphone, type PeerVideoState, type PhoneState } from './softphone';
import { canDecodeVideo } from './video';

export interface PhoneVideo {
  /** This browser can decode video (WebCodecs); without it the call goes on with audio only. */
  supported: boolean;
  /** The call has video (a video call, or one that became one). */
  hasVideo: boolean;
  /** Pictures from the other side are arriving. */
  active: boolean;
  width: number;
  height: number;
  /** The last thing the other side reported about its video. */
  peer: PeerVideoState | null;
}

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
  video: PhoneVideo;
}

const noVideo = (): PhoneVideo => ({ supported: canDecodeVideo(), hasVideo: false, active: false, width: 0, height: 0, peer: null });
const idle = (): PhoneSession => ({ callId: null, state: 'idle', muted: false, mic: 0, peer: 0, peerSpeaking: false, liveAt: null, video: noVideo() });

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
        onCall: (hasVideo) => setSession((s) => (s && phone.current === p ? { ...s, video: { ...s.video, hasVideo: s.video.hasVideo || hasVideo } } : s)),
        onVideoInfo: (info) =>
          setSession((s) =>
            s && phone.current === p ? { ...s, video: { ...s.video, hasVideo: s.video.hasVideo || info.active, active: info.active, width: info.width, height: info.height } } : s,
          ),
        onPeerVideo: (peerVideo) => setSession((s) => (s && phone.current === p ? { ...s, video: { ...s.video, peer: peerVideo } } : s)),
      });
      phone.current = p;
      setSession({ ...idle(), callId });
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
          const ticket = await getStreamTicket(instance.token, call.callId, streamOptions(canDecodeVideo()));
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
          const call = await dialCall(instance.token, number);
          patch({ callId: call.callId });
          // the audio is connected while the phone rings; what is queued meanwhile plays when it is answered
          const ticket = await getStreamTicket(instance.token, call.callId, streamOptions(canDecodeVideo()));
          if (!(await p.connect(streamWsUrl(apiUrl, ticket.path)))) {
            await hangupCall(instance.token, call.callId).catch(() => undefined);
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

  /** The canvas the other side's video is drawn on (null when it is no longer on the page). */
  const attachCanvas = useCallback((canvas: HTMLCanvasElement | null) => phone.current?.attachCanvas(canvas), []);

  const dismiss = useCallback(() => {
    phone.current?.stop();
    phone.current = null;
    setSession(null);
  }, []);

  return { session, join, dial, hangup, toggleMute, dismiss, attachCanvas };
}
