/**
 * The video side of the browser phone. The stream carries H.264 access units in Annex-B
 * framing, one picture per message, and neither the server nor WhatsApp converts anything:
 * what the other side's camera produced is what arrives, and what is sent is what the browser
 * encoded. This file holds the parts that need no browser (reading the stream of NAL units,
 * finding the codec in a keyframe, the geometry of the turn a picture needs) and the receiver
 * that decodes with WebCodecs and draws on a canvas.
 */

/** NAL unit types that matter here (H.264, the low five bits of the byte after the start code). */
export const NAL_SLICE_IDR = 5;
export const NAL_SPS = 7;
export const NAL_PPS = 8;

export interface Nal {
  type: number;
  /** Offset of the NAL header byte inside the access unit (just after its start code). */
  start: number;
  /** End of the NAL (the start of the next start code, or the end of the data). */
  end: number;
}

/** Splits an Annex-B access unit (start codes 00 00 01 or 00 00 00 01) into its NAL units. */
export function splitNals(data: Uint8Array): Nal[] {
  const starts: number[] = []; // offsets just after each start code
  const codeAt: number[] = []; // offsets of the first byte of each start code
  for (let i = 0; i + 2 < data.length; i++) {
    if (data[i] === 0 && data[i + 1] === 0 && data[i + 2] === 1) {
      codeAt.push(i > 0 && data[i - 1] === 0 ? i - 1 : i); // the four-byte form owns its leading zero
      starts.push(i + 3);
      i += 2;
    }
  }
  const nals: Nal[] = [];
  starts.forEach((start, n) => {
    if (start >= data.length) return;
    const end = n + 1 < starts.length ? (codeAt[n + 1] ?? data.length) : data.length;
    nals.push({ type: (data[start] ?? 0) & 0x1f, start, end });
  });
  return nals;
}

/** A keyframe is a picture with an IDR slice in it (the SPS and PPS ride in front of it). */
export function isKeyframe(data: Uint8Array): boolean {
  return splitNals(data).some((n) => n.type === NAL_SLICE_IDR);
}

/**
 * The codec string WebCodecs wants ("avc1.42c01e") from the SPS in an access unit: profile,
 * constraint flags and level are the three bytes after the NAL header. Null when there is none.
 */
export function codecOf(data: Uint8Array): string | null {
  const sps = splitNals(data).find((n) => n.type === NAL_SPS);
  if (!sps || sps.end - sps.start < 4) return null;
  const hex = (b: number | undefined) => (b ?? 0).toString(16).padStart(2, '0');
  return `avc1.${hex(data[sps.start + 1])}${hex(data[sps.start + 2])}${hex(data[sps.start + 3])}`;
}

export interface Rotation {
  /** Size of the canvas that holds the turned picture. */
  width: number;
  height: number;
  /** Argument of CanvasRenderingContext2D.setTransform(a, b, c, d, e, f). */
  matrix: [number, number, number, number, number, number];
}

/**
 * How to draw a picture of `w` by `h` turned `turns` quarter turns clockwise (what the server
 * reports per picture: the turns that make it upright). Three-quarter turns are a quarter
 * counter-clockwise; anything beyond is taken modulo four.
 */
export function rotation(w: number, h: number, turns: number): Rotation {
  switch (((turns % 4) + 4) % 4) {
    case 1:
      return { width: h, height: w, matrix: [0, 1, -1, 0, h, 0] };
    case 2:
      return { width: w, height: h, matrix: [-1, 0, 0, -1, w, h] };
    case 3:
      return { width: h, height: w, matrix: [0, -1, 1, 0, 0, w] };
    default:
      return { width: w, height: h, matrix: [1, 0, 0, 1, 0, 0] };
  }
}

/** Whether this browser can decode (and so show) the video of a call. */
export function canDecodeVideo(): boolean {
  return typeof VideoDecoder !== 'undefined' && typeof EncodedVideoChunk !== 'undefined';
}

export interface VideoInfo {
  /** Pictures are arriving. */
  active: boolean;
  width: number;
  height: number;
}

/** No picture for this long means the other side's camera is off or the video stalled. */
const IDLE_MS = 2500;
/** More than this waiting to be decoded means the decoder is behind: skip to the next keyframe. */
const MAX_QUEUE = 8;

/**
 * Decodes the other side's video and draws it. It waits for a keyframe before it decodes
 * anything (a picture without the one before it is garbage), and when the decoder falls behind
 * or fails it starts over at the next keyframe: the browser cannot ask WhatsApp for one, so a
 * loss can freeze the picture for a few seconds.
 */
export class VideoReceiver {
  private canvas: HTMLCanvasElement | null = null;
  private decoder: VideoDecoder | null = null;
  private codec: string | null = null;
  private needKey = true;
  private turns = 0;
  private lastAt = 0;
  private active = false;
  private size = { width: 0, height: 0 };
  private timer: ReturnType<typeof setInterval> | undefined;

  constructor(private readonly onInfo: (info: VideoInfo) => void) {
    this.timer = setInterval(() => {
      if (this.active && Date.now() - this.lastAt > IDLE_MS) this.setActive(false);
    }, 500);
  }

  /** The canvas the pictures are drawn on (null when the page no longer shows it). */
  attach(canvas: HTMLCanvasElement | null) {
    this.canvas = canvas;
  }

  /** One access unit from the stream; `turns` is the quarter turns clockwise that make it upright. */
  push(data: Uint8Array, keyframe: boolean, timestampMs: number, turns: number) {
    if (data.length === 0) return;
    this.turns = turns;
    if (keyframe) {
      const codec = codecOf(data);
      if (!codec) return; // a keyframe the decoder cannot be set up from
      if (codec !== this.codec || !this.decoder || this.decoder.state === 'closed') this.start(codec);
      this.needKey = false;
    } else if (this.needKey || !this.decoder) {
      return;
    }
    const decoder = this.decoder;
    if (!decoder) return;
    if (!keyframe && decoder.decodeQueueSize > MAX_QUEUE) {
      this.needKey = true;
      return;
    }
    try {
      decoder.decode(new EncodedVideoChunk({ type: keyframe ? 'key' : 'delta', timestamp: timestampMs * 1000, data }));
    } catch {
      this.reset();
    }
  }

  close() {
    if (this.timer) clearInterval(this.timer);
    this.timer = undefined;
    this.reset();
    this.canvas = null;
  }

  private start(codec: string) {
    this.reset();
    const decoder = new VideoDecoder({
      output: (frame) => {
        this.draw(frame);
        frame.close();
      },
      error: () => this.reset(),
    });
    try {
      decoder.configure({ codec, optimizeForLatency: true, avc: { format: 'annexb' } } as VideoDecoderConfig);
    } catch {
      return;
    }
    this.decoder = decoder;
    this.codec = codec;
  }

  private reset() {
    try {
      if (this.decoder && this.decoder.state !== 'closed') this.decoder.close();
    } catch {
      /* already closed */
    }
    this.decoder = null;
    this.codec = null;
    this.needKey = true;
  }

  private draw(frame: VideoFrame) {
    this.lastAt = Date.now();
    const canvas = this.canvas;
    const w = frame.displayWidth;
    const h = frame.displayHeight;
    const r = rotation(w, h, this.turns);
    if (canvas) {
      if (canvas.width !== r.width || canvas.height !== r.height) {
        canvas.width = r.width;
        canvas.height = r.height;
      }
      const ctx = canvas.getContext('2d');
      if (ctx) {
        ctx.setTransform(...r.matrix);
        ctx.drawImage(frame, 0, 0);
        ctx.setTransform(1, 0, 0, 1, 0, 0);
      }
    }
    if (!this.active || this.size.width !== r.width || this.size.height !== r.height) {
      this.size = { width: r.width, height: r.height };
      this.setActive(true);
    }
  }

  private setActive(active: boolean) {
    this.active = active;
    this.onInfo({ active, ...this.size });
  }
}
