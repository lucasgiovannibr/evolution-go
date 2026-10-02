/**
 * The audio side of the browser phone, as pure functions so it can be tested without a
 * browser: converting between the Web Audio floats and the PCM the stream carries, bringing
 * the microphone's rate down to 16 kHz, the binary frames of the stream and the pace at
 * which the peer's audio is played out.
 */

/** The audio of every call: signed 16-bit little-endian mono at 16 kHz. */
export const PCM_RATE = 16_000;

export const FRAME_AUDIO = 0x01;

/** What the panel asks the stream for: PCM 16 kHz, binary frames, and speech events. */
export const STREAM_OPTIONS = { encoding: 'audio/pcm-s16le', sampleRate: PCM_RATE, binary: true, speechEvents: true } as const;

export function floatToPcm16(input: Float32Array): Int16Array {
  const out = new Int16Array(input.length);
  for (let i = 0; i < input.length; i++) {
    const v = Math.max(-1, Math.min(1, input[i] ?? 0));
    out[i] = v < 0 ? Math.round(v * 32768) : Math.round(v * 32767);
  }
  return out;
}

export function pcm16ToFloat(input: Int16Array): Float32Array<ArrayBuffer> {
  const out = new Float32Array(input.length);
  for (let i = 0; i < input.length; i++) out[i] = (input[i] ?? 0) / 32768;
  return out;
}

/** Root mean square of a block, 0 to 1. */
export function rms(block: ArrayLike<number>): number {
  if (block.length === 0) return 0;
  let sum = 0;
  for (let i = 0; i < block.length; i++) sum += (block[i] ?? 0) ** 2;
  return Math.sqrt(sum / block.length);
}

/**
 * Brings audio down to a lower rate by averaging what falls into each output sample (the
 * microphone runs at 44.1 or 48 kHz). The state is kept between calls, so the blocks of
 * the audio thread (128 samples) can be fed one by one. A rate that is already the target
 * passes through.
 */
export class Downsampler {
  private readonly step: number;
  private acc = 0;
  private weight = 0;

  constructor(
    readonly inRate: number,
    readonly outRate: number,
  ) {
    if (inRate < outRate) throw new Error('Downsampler only lowers the rate');
    this.step = inRate / outRate;
  }

  process(input: Float32Array): Float32Array {
    if (this.inRate === this.outRate) return input;
    const out: number[] = [];
    for (let i = 0; i < input.length; i++) {
      let left = 1; // how much of this input sample is still to be spent
      const sample = input[i] ?? 0;
      while (left > 1e-9) {
        const take = Math.min(left, this.step - this.weight);
        this.acc += sample * take;
        this.weight += take;
        left -= take;
        if (this.weight >= this.step - 1e-9) {
          out.push(this.acc / this.step);
          this.acc = 0;
          this.weight = 0;
        }
      }
    }
    return Float32Array.from(out);
  }
}

/** Cuts a stream of samples into frames of a fixed size; what is left over waits for the next call. */
export class Chunker {
  private pending = new Int16Array(0);

  constructor(readonly size: number) {}

  push(samples: Int16Array): Int16Array[] {
    const all = new Int16Array(this.pending.length + samples.length);
    all.set(this.pending);
    all.set(samples, this.pending.length);
    const frames: Int16Array[] = [];
    let at = 0;
    for (; at + this.size <= all.length; at += this.size) frames.push(all.slice(at, at + this.size));
    this.pending = all.slice(at);
    return frames;
  }
}

/** The binary frame that carries audio to the peer: 0x01 and the PCM, little-endian. */
export function audioFrame(pcm: Int16Array): ArrayBuffer {
  const buf = new ArrayBuffer(1 + pcm.length * 2);
  const view = new DataView(buf);
  view.setUint8(0, FRAME_AUDIO);
  for (let i = 0; i < pcm.length; i++) view.setInt16(1 + i * 2, pcm[i] ?? 0, true);
  return buf;
}

export type ServerFrame =
  | { kind: 'audio'; seq: number; timestamp: number; pcm: Int16Array }
  | { kind: 'video'; seq: number; timestamp: number };

/**
 * Reads a binary frame from the server (all integers big-endian):
 * audio is 0x01, seq u32, timestamp u32 and the PCM; video 0x02, flags u8, seq u32, timestamp u32 and the picture.
 * Anything else, or a frame too short to be one, is null.
 */
export function parseServerFrame(buf: ArrayBuffer): ServerFrame | null {
  const view = new DataView(buf);
  if (view.byteLength < 1) return null;
  const type = view.getUint8(0);
  if (type === 0x01 && view.byteLength >= 9) {
    const n = (view.byteLength - 9) >> 1;
    const pcm = new Int16Array(n);
    for (let i = 0; i < n; i++) pcm[i] = view.getInt16(9 + i * 2, true);
    return { kind: 'audio', seq: view.getUint32(1), timestamp: view.getUint32(5), pcm };
  }
  if (type === 0x02 && view.byteLength >= 10) {
    return { kind: 'video', seq: view.getUint32(2), timestamp: view.getUint32(6) };
  }
  return null;
}

/**
 * Decides when each piece of the peer's audio plays. The server stamps every frame with the
 * moment it reached it, and a peer that is quiet sends only two or three frames a second, so
 * playing frames back to back would turn every pause into a gap and a burst into a pile-up.
 * Instead the first frame starts a little in the future and each later one plays at its own
 * offset from it, never before the previous one has finished. A frame that is already late by
 * more than `rebaseAfter` (the tab was in the background, say) starts the clock over.
 */
export class PlayoutClock {
  private base: number | null = null; // audio-context time at which stream time 0 plays
  private nextFree = 0;

  constructor(
    private readonly lead = 0.15,
    private readonly rebaseAfter = 0.5,
  ) {}

  /** Seconds, on the audio context's clock, at which a frame stamped `timestampMs` should start. */
  startAt(timestampMs: number, durationSec: number, now: number): number {
    const at = timestampMs / 1000;
    if (this.base === null || this.base + at < now - this.rebaseAfter) {
      this.base = now + this.lead - at;
      this.nextFree = 0;
    }
    const start = Math.max(this.base + at, this.nextFree, now);
    this.nextFree = start + durationSec;
    return start;
  }

  reset() {
    this.base = null;
    this.nextFree = 0;
  }
}
