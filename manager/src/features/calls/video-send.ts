/**
 * Sending the camera: it is cropped to the shape the phone shows a call in (portrait), encoded
 * to H.264 with WebCodecs and handed to the stream as Annex-B access units. WhatsApp's call
 * screen on a phone is portrait whatever the phone's position, so a landscape picture is shown
 * small with bars on the sides; a portrait one fills it (tried live with an iPhone).
 */

import { splitNals } from './video';

/** What is sent: portrait, modest, and small enough that a phone on a mobile network keeps up. */
export const SEND_WIDTH = 360;
export const SEND_HEIGHT = 640;
export const SEND_FPS = 15;
export const SEND_BITRATE = 500_000;
/** Constrained baseline, level 3.1: the profile that every phone decodes. */
export const SEND_CODEC = 'avc1.42001f';
/** The decoder on the other side needs a way in at least this often; it also repairs a loss. */
export const KEYFRAME_EVERY_MS = 2000;

export const FRAME_VIDEO = 0x02;

/** NAL type of the access unit delimiter (H.264). */
const NAL_AUD = 9;

/**
 * The access unit without its delimiters. Chrome's encoder starts every picture with one (NAL type 9):
 * it is optional, and nothing tells that WhatsApp's decoder takes it, while every phone takes a picture
 * that is only its SPS and PPS (on a keyframe) and its slice. The start codes come out four bytes long.
 */
export function withoutDelimiters(accessUnit: Uint8Array): Uint8Array {
  const nals = splitNals(accessUnit);
  if (!nals.some((n) => n.type === NAL_AUD)) return accessUnit;
  const kept = nals.filter((n) => n.type !== NAL_AUD);
  const out = new Uint8Array(kept.reduce((total, n) => total + 4 + (n.end - n.start), 0));
  let at = 0;
  for (const n of kept) {
    out.set([0, 0, 0, 1], at);
    out.set(accessUnit.subarray(n.start, n.end), at + 4);
    at += 4 + (n.end - n.start);
  }
  return out;
}

/** The binary frame that carries one H.264 access unit to the peer: 0x02 and the picture. */
export function videoFrame(accessUnit: Uint8Array): ArrayBuffer {
  const buf = new ArrayBuffer(1 + accessUnit.length);
  const bytes = new Uint8Array(buf);
  bytes[0] = FRAME_VIDEO;
  bytes.set(accessUnit, 1);
  return buf;
}

export interface Crop {
  sx: number;
  sy: number;
  sw: number;
  sh: number;
}

/** The centred part of a source picture that fills a destination of another shape without stretching it. */
export function coverCrop(srcW: number, srcH: number, dstW: number, dstH: number): Crop {
  const srcRatio = srcW / srcH;
  const dstRatio = dstW / dstH;
  if (srcRatio > dstRatio) {
    const sw = srcH * dstRatio; // too wide: cut the sides
    return { sx: (srcW - sw) / 2, sy: 0, sw, sh: srcH };
  }
  const sh = srcW / dstRatio; // too tall: cut the top and bottom
  return { sx: 0, sy: (srcH - sh) / 2, sw: srcW, sh };
}

/** Decides which pictures are keyframes: the first, one every `intervalMs`, and the ones that were asked for. */
export class KeyframePolicy {
  private last: number | null = null;
  private asked = true; // the first picture is always a keyframe

  constructor(private readonly intervalMs = KEYFRAME_EVERY_MS) {}

  /** The other side (or the server) wants a keyframe now. */
  request() {
    this.asked = true;
  }

  /** Whether the picture taken at `now` (milliseconds) should be a keyframe. */
  next(now: number): boolean {
    const due = this.asked || this.last === null || now - this.last >= this.intervalMs;
    if (due) {
      this.last = now;
      this.asked = false;
    }
    return due;
  }
}

export function canEncodeVideo(): boolean {
  return typeof VideoEncoder !== 'undefined' && typeof VideoFrame !== 'undefined';
}

export interface SenderEvents {
  /** An access unit to put on the wire. */
  send(accessUnit: Uint8Array): void;
  /** The camera or the encoder cannot be used; the sender has stopped. */
  onError(message: string): void;
}

/** What to tell the person when the browser refuses the camera. */
export function cameraErrorMessage(err: unknown): string {
  const name = err instanceof DOMException ? err.name : '';
  if (name === 'NotAllowedError' || name === 'SecurityError') return 'O acesso à câmera foi negado. Libere-o nas permissões do site e tente de novo.';
  if (name === 'NotFoundError' || name === 'OverconstrainedError') return 'Nenhuma câmera foi encontrada neste aparelho.';
  if (name === 'NotReadableError') return 'A câmera está em uso por outro programa.';
  return err instanceof Error && err.message ? err.message : 'Não foi possível usar a câmera.';
}

/**
 * Takes the camera, crops it to portrait, encodes it and gives the access units to `send`. Nothing
 * is encoded until `setReady(true)`: the call must be able to carry video (a video call, or an
 * upgrade the other side accepted), or the server would only drop the pictures.
 */
export class VideoSender {
  private stream: MediaStream | null = null;
  private video: HTMLVideoElement | null = null;
  private canvas: HTMLCanvasElement | null = null;
  private encoder: VideoEncoder | null = null;
  private timer: ReturnType<typeof setInterval> | undefined;
  private ready = false;
  private readonly keyframes = new KeyframePolicy();

  constructor(private readonly events: SenderEvents) {}

  /** The camera, for the preview the person sees of themselves. */
  get preview(): MediaStream | null {
    return this.stream;
  }

  /** Opens the camera and the encoder. Resolves false (after reporting why) when it cannot. */
  async start(): Promise<boolean> {
    try {
      if (!navigator.mediaDevices?.getUserMedia) throw new Error('O navegador só libera a câmera em páginas seguras: abra o painel por https ou por localhost.');
      const config: VideoEncoderConfig = {
        codec: SEND_CODEC,
        width: SEND_WIDTH,
        height: SEND_HEIGHT,
        bitrate: SEND_BITRATE,
        framerate: SEND_FPS,
        latencyMode: 'realtime',
        avc: { format: 'annexb' },
      };
      const support = await VideoEncoder.isConfigSupported(config);
      if (!support.supported) throw new Error('Este navegador não codifica H.264, que o WhatsApp exige para o vídeo.');

      this.stream = await navigator.mediaDevices.getUserMedia({
        video: { width: { ideal: 640 }, height: { ideal: 480 }, frameRate: { ideal: SEND_FPS }, facingMode: 'user' },
      });
      const video = document.createElement('video');
      video.muted = true;
      video.playsInline = true;
      video.srcObject = this.stream;
      await video.play();
      this.video = video;

      this.canvas = document.createElement('canvas');
      this.canvas.width = SEND_WIDTH;
      this.canvas.height = SEND_HEIGHT;

      const encoder = new VideoEncoder({
        output: (chunk) => {
          const au = new Uint8Array(chunk.byteLength);
          chunk.copyTo(au);
          this.events.send(withoutDelimiters(au));
        },
        error: (e) => this.fail(e.message || 'O codificador de vídeo falhou.'),
      });
      encoder.configure(config);
      this.encoder = encoder;

      this.timer = setInterval(() => this.tick(), 1000 / SEND_FPS);
      return true;
    } catch (err) {
      this.fail(cameraErrorMessage(err));
      return false;
    }
  }

  /** Whether the call can carry our video yet; turning it on makes the next picture a keyframe. */
  setReady(ready: boolean) {
    if (ready && !this.ready) this.keyframes.request();
    this.ready = ready;
  }

  /** The other side lost part of our video: the next picture is a keyframe. */
  requestKeyframe() {
    this.keyframes.request();
  }

  stop() {
    if (this.timer) clearInterval(this.timer);
    this.timer = undefined;
    try {
      if (this.encoder && this.encoder.state !== 'closed') this.encoder.close();
    } catch {
      /* already closed */
    }
    this.encoder = null;
    this.stream?.getTracks().forEach((t) => t.stop());
    this.stream = null;
    if (this.video) this.video.srcObject = null;
    this.video = null;
    this.canvas = null;
  }

  private fail(message: string) {
    this.stop();
    this.events.onError(message);
  }

  private tick() {
    const { video, canvas, encoder } = this;
    if (!this.ready || !video || !canvas || !encoder || encoder.state !== 'configured') return;
    if (video.readyState < 2 || video.videoWidth === 0) return;
    if (encoder.encodeQueueSize > 3) return; // the encoder is behind: skip a picture rather than add delay
    const ctx = canvas.getContext('2d');
    if (!ctx) return;
    const c = coverCrop(video.videoWidth, video.videoHeight, SEND_WIDTH, SEND_HEIGHT);
    ctx.drawImage(video, c.sx, c.sy, c.sw, c.sh, 0, 0, SEND_WIDTH, SEND_HEIGHT);
    const frame = new VideoFrame(canvas, { timestamp: Math.round(performance.now() * 1000) });
    try {
      encoder.encode(frame, { keyFrame: this.keyframes.next(performance.now()) });
    } finally {
      frame.close();
    }
  }
}
