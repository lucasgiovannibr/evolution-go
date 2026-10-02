import { useCallback, useEffect, useRef, useState } from 'react';
import { toast } from 'sonner';
import { answerCall, callVideo, dialCall, getStreamTicket, hangupCall, type CallInfo } from '@/api/calls';
import type { Instance } from '@/api/types';
import { errMsg } from '@/hooks/use-instances';
import { ApiError } from '@/lib/http';
import { useAuth } from '@/stores/auth';
import { streamOptions } from './audio';
import { streamWsUrl } from './format';
import { Softphone, type PeerVideoState, type PhoneState } from './softphone';
import { canDecodeVideo } from './video';
import { canEncodeVideo } from './video-send';

export interface PhoneVideo {
  /** This browser can decode video (WebCodecs); without it the call goes on with audio only. */
  supported: boolean;
  /** This browser can also encode it, so the camera can be sent. */
  canSend: boolean;
  /** The call has video (a video call, or one that became one). */
  hasVideo: boolean;
  /** Pictures from the other side are arriving. */
  active: boolean;
  width: number;
  height: number;
  /** The last thing the other side reported about its video. */
  peer: PeerVideoState | null;
  /** Our camera is on and being sent. */
  camera: boolean;
  /** The camera, for the preview the person sees of themselves. */
  preview: MediaStream | null;
  /** Why the camera could not be used, when it could not. */
  cameraError: string | null;
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

const noVideo = (): PhoneVideo => ({
  supported: canDecodeVideo(),
  canSend: canDecodeVideo() && canEncodeVideo(),
  hasVideo: false,
  active: false,
  width: 0,
  height: 0,
  peer: null,
  camera: false,
  preview: null,
  cameraError: null,
});
const idle = (): PhoneSession => ({ callId: null, state: 'idle', muted: false, mic: 0, peer: 0, peerSpeaking: false, liveAt: null, video: noVideo() });

/** A video state after which the call carries video in both directions. */
const carriesVideo = (state: string) => state === 'upgrade_accepted' || state === 'enabled';

/**
 * The browser phone of one instance: one call at a time, with the audio (and, in a video call, the
 * video) of the page. It is how a person answers, dials and talks from the panel; anyone else uses
 * the API and a stream of their own. Leaving the page lets go of the audio and the camera, and a
 * call without a stream is hung up by the server after CALL_STREAM_GRACE.
 */
export function usePhone(instance: Instance, onChange: () => void) {
  const [session, setSession] = useState<PhoneSession | null>(null);
  const phone = useRef<Softphone | null>(null);
  const busy = useRef(false);
  const apiUrl = useAuth((s) => s.apiUrl);

  const patch = useCallback((p: Partial<PhoneSession>) => setSession((s) => (s ? { ...s, ...p } : s)), []);
  const patchVideo = useCallback((p: Partial<PhoneVideo>) => setSession((s) => (s ? { ...s, video: { ...s.video, ...p } } : s)), []);

  const open = useCallback(
    (callId: string | null): Softphone => {
      phone.current?.stop();
      // events of a phone that was replaced do not touch the session of the new one
      const mine = (s: PhoneSession | null): s is PhoneSession => !!s && phone.current === p;
      const p = new Softphone({
        onState: (state, detail) => {
          setSession((s) => (mine(s) ? { ...s, state, detail, liveAt: state === 'live' ? Date.now() : s.liveAt } : s));
          if (state === 'ended' || state === 'error') onChange();
        },
        onLevels: (mic, peer) => setSession((s) => (mine(s) ? { ...s, mic, peer } : s)),
        onPeerSpeaking: (peerSpeaking) => setSession((s) => (mine(s) ? { ...s, peerSpeaking } : s)),
        onCall: (hasVideo) => setSession((s) => (mine(s) ? { ...s, video: { ...s.video, hasVideo: s.video.hasVideo || hasVideo } } : s)),
        onVideoInfo: (info) =>
          setSession((s) =>
            mine(s) ? { ...s, video: { ...s.video, hasVideo: s.video.hasVideo || info.active, active: info.active, width: info.width, height: info.height } } : s,
          ),
        onPeerVideo: (peerVideo) =>
          setSession((s) => (mine(s) ? { ...s, video: { ...s.video, peer: peerVideo, hasVideo: s.video.hasVideo || carriesVideo(peerVideo.state) } } : s)),
        onCamera: (camera, preview) =>
          setSession((s) => (mine(s) ? { ...s, video: { ...s.video, camera, preview, cameraError: camera ? null : s.video.cameraError } } : s)),
        onCameraError: (cameraError) => setSession((s) => (mine(s) ? { ...s, video: { ...s.video, cameraError } } : s)),
      });
      phone.current = p;
      setSession({ ...idle(), callId });
      return p;
    },
    [onChange],
  );

  // the audio and the camera belong to this page
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
          // a video call is answered with the camera on; if it cannot be, the call goes on receiving
          if (call.video && canEncodeVideo()) await p.startCamera();
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

  /** Places a call (with video if asked) and connects the audio while the phone rings. */
  const dial = useCallback(
    (number: string, video = false) =>
      guard(async () => {
        const p = open(null);
        if (!(await p.prepare())) return;
        try {
          if (video && canEncodeVideo()) await p.startCamera();
          const call = await dialCall(instance.token, number, video);
          patch({ callId: call.callId });
          // what is queued while the phone rings plays when it is answered
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

  /** Hangs the call up (for both sides) and lets go of the audio and the camera. */
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

  /**
   * Turns our camera on or off. On a call without video, turning it on asks the other side to turn
   * the call into a video call (our pictures go out once it accepts); on one that has video it
   * unmutes our video. Off stops the camera and mutes our video on the call.
   */
  const toggleCamera = useCallback(async () => {
    const p = phone.current;
    const id = session?.callId;
    if (!p || !id) return;
    const hasVideo = session.video.hasVideo;

    if (p.cameraOn) {
      p.stopCamera();
      if (hasVideo) await callVideo(instance.token, id, 'disable').catch(() => undefined);
      return;
    }
    if (!(await p.startCamera())) return; // the reason is shown by the panel
    try {
      if (hasVideo) {
        // "enable" only unmutes video the call already had; on one that never had it the server says 409
        await callVideo(instance.token, id, 'enable').catch(async (e) => {
          if (e instanceof ApiError && e.status === 409) await callVideo(instance.token, id, 'start');
          else throw e;
        });
      } else {
        await callVideo(instance.token, id, 'start');
      }
    } catch (e) {
      p.stopCamera();
      toast.error('Não foi possível ligar o vídeo', { description: errMsg(e) });
    }
  }, [instance.token, session?.callId, session?.video.hasVideo]);

  /** Accepts the other side's request to turn the call into a video call, and sends the camera if it can. */
  const acceptVideo = useCallback(async () => {
    const p = phone.current;
    const id = session?.callId;
    if (!p || !id) return;
    try {
      await callVideo(instance.token, id, 'accept');
    } catch (e) {
      toast.error('Não foi possível aceitar o vídeo', { description: errMsg(e) });
      return;
    }
    p.setVideoReady(true);
    patchVideo({ hasVideo: true });
    if (canEncodeVideo() && !p.cameraOn) await p.startCamera(); // a refusal is shown by the panel; the call goes on receiving
  }, [instance.token, patchVideo, session?.callId]);

  /** The canvas the other side's video is drawn on (null when it is no longer on the page). */
  const attachCanvas = useCallback((canvas: HTMLCanvasElement | null) => phone.current?.attachCanvas(canvas), []);

  const dismiss = useCallback(() => {
    phone.current?.stop();
    phone.current = null;
    setSession(null);
  }, []);

  return { session, join, dial, hangup, toggleMute, toggleCamera, acceptVideo, dismiss, attachCanvas };
}
