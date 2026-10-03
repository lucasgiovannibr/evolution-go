import { Chunker, Downsampler, PCM_RATE, PlayoutClock, audioFrame, floatToPcm16, parseServerFrame, pcm16ToFloat, rms } from './audio';
import captureWorkletUrl from './capture-worklet.ts?worker&url';
import { micErrorMessage } from './format';
import { canDecodeVideo, VideoReceiver, type VideoInfo } from './video';
import { canEncodeVideo, VideoSender, videoFrame } from './video-send';

export type PhoneState = 'idle' | 'ready' | 'connecting' | 'live' | 'ended' | 'error';

/** What the other side reported about its video (the stream's `video_state`). */
export interface PeerVideoState {
  /** enabled, disabled, stopped, upgrade_request, upgrade_accepted, upgrade_rejected, upgrade_cancelled or unknown. */
  state: string;
  active: boolean;
  /** The other side asks to turn the call into a video call. */
  upgrade: boolean;
}

export interface PhoneEvents {
  /** The phone changed state; `detail` says why when it ended or failed. */
  onState(state: PhoneState, detail?: string): void;
  /** Levels of the microphone and of the peer, 0 to 1, about ten times a second. */
  onLevels(mic: number, peer: number): void;
  /** The peer began or stopped talking (the stream's speech events). */
  onPeerSpeaking(speaking: boolean): void;
  /** The stream started: whether the call has video. */
  onCall?(hasVideo: boolean): void;
  /** Pictures from the peer started or stopped arriving, or changed size. */
  onVideoInfo?(info: VideoInfo): void;
  /** The peer's camera or an upgrade request changed. */
  onPeerVideo?(state: PeerVideoState): void;
  /** Our camera was turned on (with its picture for a preview) or off. */
  onCamera?(on: boolean, preview: MediaStream | null): void;
  /** Our camera could not be used, in words. */
  onCameraError?(message: string): void;
  /** How many pictures of our camera have been put on the wire so far (a way to see that video is going out). */
  onVideoSent?(frames: number): void;
}

/** 20 ms of audio per frame sent to the peer: small enough for low delay, large enough not to flood the socket. */
const SEND_SAMPLES = PCM_RATE / 50;
/** What may wait in the socket before frames are dropped instead (the connection is slower than real time). */
const MAX_BUFFERED = 128 * 1024;

/**
 * A phone in the browser for one call: the microphone goes to the stream and what the peer
 * says comes out of the speakers. The stream is the one of GET /call/stream/{callId}, with
 * binary frames, PCM 16 kHz and speech events (see docs/wiki/guias-api/api-call.md).
 *
 * It is used in two steps so that the microphone can be asked for (which needs a click and
 * may show a permission prompt) before the call exists: `prepare`, then `connect` with the
 * address the ticket gave.
 */
export class Softphone {
  state: PhoneState = 'idle';

  private ctx?: AudioContext;
  private mic?: MediaStream;
  private node?: AudioWorkletNode;
  private out?: GainNode;
  private ws?: WebSocket;
  private downsampler?: Downsampler;
  private readonly chunker = new Chunker(SEND_SAMPLES);
  private readonly clock = new PlayoutClock();
  /** Decodes the peer's video; null in a browser that cannot (the call then goes on with audio only). */
  private readonly receiver: VideoReceiver | null;
  /** Sends our camera; null while it is off. */
  private sender: VideoSender | null = null;
  /** The call can carry our video (a video call, or an upgrade that went through). */
  private videoReady = false;
  private videoSent = 0;
  private videoSentReported = 0;
  private muted = false;
  private micLevel = 0;
  private peerLevel = 0;
  private levelTimer?: ReturnType<typeof setInterval>;
  private opened?: (ok: boolean) => void;

  constructor(private readonly events: PhoneEvents) {
    this.receiver = canDecodeVideo() ? new VideoReceiver((info) => events.onVideoInfo?.(info)) : null;
  }

  /** The canvas the peer's video is drawn on; null when the page stops showing it. */
  attachCanvas(canvas: HTMLCanvasElement | null) {
    this.receiver?.attach(canvas);
  }

  private set(state: PhoneState, detail?: string) {
    this.state = state;
    this.events.onState(state, detail);
  }

  /** Opens the audio context and the microphone. Resolves false (and reports why) when it cannot. */
  async prepare(): Promise<boolean> {
    if (this.state !== 'idle') return this.state === 'ready';
    try {
      if (!navigator.mediaDevices?.getUserMedia) throw new Error(micErrorMessage(null));
      const ctx = new AudioContext();
      this.ctx = ctx;
      if (ctx.state === 'suspended') await ctx.resume();
      if (ctx.sampleRate < PCM_RATE) throw new Error(`O áudio deste navegador roda a ${ctx.sampleRate} Hz, abaixo dos ${PCM_RATE} Hz da chamada.`);

      this.mic = await navigator.mediaDevices.getUserMedia({
        audio: { echoCancellation: true, noiseSuppression: true, autoGainControl: true, channelCount: 1 },
      });
      await ctx.audioWorklet.addModule(captureWorkletUrl);

      this.downsampler = new Downsampler(ctx.sampleRate, PCM_RATE);
      const node = new AudioWorkletNode(ctx, 'pcm-capture', { numberOfInputs: 1, numberOfOutputs: 1, channelCount: 1 });
      node.port.onmessage = (e: MessageEvent<Float32Array>) => this.onMic(e.data);
      // a node nobody listens to may not be run by the browser: send it to a muted output
      const silent = ctx.createGain();
      silent.gain.value = 0;
      ctx.createMediaStreamSource(this.mic).connect(node);
      node.connect(silent).connect(ctx.destination);
      this.node = node;

      this.out = ctx.createGain();
      this.out.connect(ctx.destination);

      this.levelTimer = setInterval(() => {
        this.events.onLevels(this.micLevel, this.peerLevel);
        if (this.videoSent !== this.videoSentReported) {
          this.videoSentReported = this.videoSent;
          this.events.onVideoSent?.(this.videoSent);
        }
        this.peerLevel *= 0.6; // the bar falls when the peer's audio stops arriving
      }, 100);
      this.set('ready');
      return true;
    } catch (err) {
      this.fail(err instanceof DOMException ? micErrorMessage(err) : err instanceof Error ? err.message : micErrorMessage(err));
      return false;
    }
  }

  /** Opens the stream. Resolves true when the server says it has started, false if it could not. */
  connect(url: string): Promise<boolean> {
    if (this.state !== 'ready') return Promise.resolve(false);
    this.set('connecting');
    return new Promise<boolean>((resolve) => {
      this.opened = resolve;
      let ws: WebSocket;
      try {
        ws = new WebSocket(url);
      } catch {
        this.fail('Endereço do stream inválido.');
        return;
      }
      ws.binaryType = 'arraybuffer';
      this.ws = ws;
      ws.onmessage = (e) => this.onMessage(e.data as string | ArrayBuffer);
      ws.onerror = () => {
        if (this.state === 'connecting') this.fail('Não foi possível abrir o stream de áudio. Se o painel está em outro endereço que o servidor, a origem precisa estar em CALL_STREAM_ORIGINS.');
      };
      ws.onclose = () => {
        if (this.state === 'live') this.end('A conexão de áudio foi fechada.');
        else if (this.state === 'connecting') this.fail('O servidor fechou o stream antes de começar.');
      };
    });
  }

  get cameraOn(): boolean {
    return this.sender !== null;
  }

  /** Turns the camera on. Resolves false (after saying why through onCameraError) when it cannot. */
  async startCamera(): Promise<boolean> {
    if (this.sender) return true;
    if (!canEncodeVideo()) {
      this.events.onCameraError?.('Este navegador não codifica vídeo (WebCodecs): use um Chrome, Edge ou Safari recente para enviar a câmera.');
      return false;
    }
    const sender = new VideoSender({
      send: (au) => this.sendVideo(au),
      onError: (message) => {
        if (this.sender === sender) this.sender = null;
        this.events.onCamera?.(false, null);
        this.events.onCameraError?.(message);
      },
    });
    this.sender = sender;
    if (!(await sender.start())) return false;
    sender.setReady(this.videoReady);
    this.events.onCamera?.(true, sender.preview);
    return true;
  }

  stopCamera() {
    this.sender?.stop();
    this.sender = null;
    this.events.onCamera?.(false, null);
  }

  /** The call can (or can no longer) carry our video; the camera starts sending when it can. */
  setVideoReady(ready: boolean) {
    this.videoReady = ready;
    this.sender?.setReady(ready);
  }

  setMuted(muted: boolean) {
    this.muted = muted;
  }

  get isMuted() {
    return this.muted;
  }

  /** Lets the call go on without the browser: closes the stream, the microphone and the audio. Safe to call twice. */
  stop() {
    if (this.state === 'ended' || this.state === 'error') return;
    try {
      if (this.ws?.readyState === WebSocket.OPEN) this.ws.send(JSON.stringify({ event: 'stop' }));
    } catch {
      /* closing anyway */
    }
    this.end('Você saiu do áudio da chamada.');
  }

  // ---- internals

  private onMic(block: Float32Array) {
    const level = rms(block);
    this.micLevel = level > this.micLevel ? level : this.micLevel * 0.9 + level * 0.1;
    if (this.state !== 'live' || !this.downsampler) return;
    const frames = this.chunker.push(floatToPcm16(this.downsampler.process(block)));
    const ws = this.ws;
    if (this.muted || !ws || ws.readyState !== WebSocket.OPEN) return;
    for (const frame of frames) {
      if (ws.bufferedAmount > MAX_BUFFERED) return; // better to lose a moment than to fall seconds behind
      ws.send(audioFrame(frame));
    }
  }

  private sendVideo(accessUnit: Uint8Array) {
    const ws = this.ws;
    if (this.state !== 'live' || !ws || ws.readyState !== WebSocket.OPEN || ws.bufferedAmount > MAX_BUFFERED * 2) return;
    ws.send(videoFrame(accessUnit));
    this.videoSent++;
  }

  private onMessage(data: string | ArrayBuffer) {
    if (typeof data !== 'string') {
      const frame = parseServerFrame(data);
      if (frame?.kind === 'audio') this.play(frame.pcm, frame.timestamp);
      else if (frame?.kind === 'video') this.receiver?.push(frame.data, frame.keyframe, frame.timestamp, frame.orientation);
      return;
    }
    let msg: {
      event?: string;
      reason?: string;
      code?: string;
      message?: string;
      binary?: boolean;
      video?: boolean;
      state?: string;
      active?: boolean;
      upgrade?: boolean;
    };
    try {
      msg = JSON.parse(data);
    } catch {
      return;
    }
    switch (msg.event) {
      case 'start':
        if (this.state === 'connecting') {
          this.set('live');
          if (msg.video) this.setVideoReady(true); // a video call carries our video from the start
          // the camera may have been on before the stream was: its first keyframe went nowhere
          this.sender?.requestKeyframe();
          this.events.onCall?.(!!msg.video);
          this.opened?.(true);
          this.opened = undefined;
        }
        break;
      case 'video_state':
        // the other side accepted our request, or turned its camera on: video goes both ways now
        if (msg.state === 'upgrade_accepted' || msg.state === 'enabled') this.setVideoReady(true);
        this.events.onPeerVideo?.({ state: msg.state ?? 'unknown', active: !!msg.active, upgrade: !!msg.upgrade });
        break;
      case 'keyframe_request':
        this.sender?.requestKeyframe(); // WhatsApp lost part of our video
        break;
      case 'speech_start':
        this.events.onPeerSpeaking(true);
        break;
      case 'speech_end':
        this.events.onPeerSpeaking(false);
        break;
      case 'stop':
        this.end(msg.reason ? `A chamada terminou (${msg.reason}).` : 'A chamada terminou.');
        break;
      case 'error':
        // not fatal: the server tells about a problem once and goes on (a slow reader, a bad frame)
        if (this.state === 'connecting' && msg.code) this.fail(msg.message || msg.code);
        // what it says about our video is what the person needs to see when the picture does not arrive
        else if (msg.code && /video|binary/.test(msg.code)) this.events.onCameraError?.(`O servidor recusou o vídeo (${msg.code}): ${msg.message ?? ''}`);
        break;
    }
  }

  private play(pcm: Int16Array, timestampMs: number) {
    const ctx = this.ctx;
    if (!ctx || !this.out || pcm.length === 0) return;
    const samples = pcm16ToFloat(pcm);
    this.peerLevel = Math.max(this.peerLevel, rms(samples));
    const buffer = ctx.createBuffer(1, samples.length, PCM_RATE); // the browser resamples to its own rate
    buffer.copyToChannel(samples, 0);
    const source = ctx.createBufferSource();
    source.buffer = buffer;
    source.connect(this.out);
    source.start(this.clock.startAt(timestampMs, samples.length / PCM_RATE, ctx.currentTime));
  }

  private fail(detail: string) {
    this.opened?.(false);
    this.opened = undefined;
    this.release();
    this.set('error', detail);
  }

  private end(detail: string) {
    this.release();
    this.opened?.(false);
    this.opened = undefined;
    this.set('ended', detail);
  }

  private release() {
    if (this.levelTimer) clearInterval(this.levelTimer);
    this.levelTimer = undefined;
    try {
      this.ws?.close();
    } catch {
      /* already closed */
    }
    this.receiver?.close();
    this.stopCamera();
    this.node?.disconnect();
    this.mic?.getTracks().forEach((t) => t.stop());
    this.ctx?.close().catch(() => undefined);
    this.events.onPeerSpeaking(false);
    this.events.onLevels(0, 0);
  }
}
