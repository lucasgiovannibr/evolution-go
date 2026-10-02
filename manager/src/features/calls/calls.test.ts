import { describe, expect, it } from 'vitest';
import {
  Chunker,
  Downsampler,
  PCM_RATE,
  PlayoutClock,
  audioFrame,
  floatToPcm16,
  parseServerFrame,
  pcm16ToFloat,
  rms,
} from './audio';
import { elapsedSeconds, formatClock, levelPercent, peerLabel, peerVideoLabel, reasonLabel, streamWsUrl } from './format';

const tone = (freq: number, rate: number, seconds: number, peak = 0.5) =>
  Float32Array.from({ length: Math.round(rate * seconds) }, (_, i) => peak * Math.sin((2 * Math.PI * freq * i) / rate));

describe('PCM conversion', () => {
  it('goes to 16 bits and back within one step', () => {
    const f = Float32Array.from([0, 0.5, -0.5, 1, -1, 0.123]);
    const back = pcm16ToFloat(floatToPcm16(f));
    f.forEach((v, i) => expect(Math.abs(back[i]! - v)).toBeLessThan(1 / 16000));
  });

  it('clamps what is louder than full scale instead of wrapping around', () => {
    const pcm = floatToPcm16(Float32Array.from([2, -2, 1.0001]));
    expect([...pcm]).toEqual([32767, -32768, 32767]);
  });

  it('measures the level of a block', () => {
    expect(rms([])).toBe(0);
    expect(rms(tone(440, 16000, 1, 0.5))).toBeCloseTo(0.5 / Math.SQRT2, 2);
  });
});

describe('Downsampler', () => {
  it.each([
    [48000, 3],
    [44100, 2.75625],
  ])('brings %d Hz to 16 kHz keeping the level of a voice-band tone', (rate, ratio) => {
    const input = tone(1000, rate, 1);
    const out = new Downsampler(rate, PCM_RATE).process(input);
    expect(Math.abs(out.length - input.length / ratio)).toBeLessThan(2);
    expect(rms(out.slice(100))).toBeCloseTo(0.5 / Math.SQRT2, 1);
  });

  it('gives the same audio however the input is cut (the audio thread hands over 128 samples)', () => {
    const input = tone(700, 48000, 0.5);
    const whole = new Downsampler(48000, PCM_RATE).process(input);
    const d = new Downsampler(48000, PCM_RATE);
    const parts: number[] = [];
    for (let i = 0; i < input.length; i += 128) parts.push(...d.process(input.slice(i, i + 128)));
    expect(parts.length).toBe(whole.length);
    whole.forEach((v, i) => expect(parts[i]).toBeCloseTo(v, 6));
  });

  it('weakens what is far above the new Nyquist instead of folding it back at full strength', () => {
    const out = new Downsampler(48000, PCM_RATE).process(tone(20000, 48000, 0.5));
    expect(rms(out.slice(50))).toBeLessThan(0.25 * (0.5 / Math.SQRT2));
  });

  it('passes a rate that is already the target through', () => {
    const input = tone(500, 16000, 0.1);
    expect(new Downsampler(16000, 16000).process(input)).toBe(input);
  });

  it('refuses to raise the rate', () => {
    expect(() => new Downsampler(8000, 16000)).toThrow();
  });
});

describe('Chunker', () => {
  it('cuts into whole frames and keeps the remainder for the next push', () => {
    const c = new Chunker(320);
    expect(c.push(new Int16Array(100))).toHaveLength(0);
    const frames = c.push(new Int16Array(560)); // 660 in all
    expect(frames).toHaveLength(2);
    expect(frames.every((f) => f.length === 320)).toBe(true);
    expect(c.push(new Int16Array(300))).toHaveLength(1); // 20 left over + 300
  });

  it('keeps the samples in order', () => {
    const c = new Chunker(4);
    const out = [...c.push(Int16Array.from([1, 2, 3])), ...c.push(Int16Array.from([4, 5, 6, 7, 8]))];
    expect(out.map((f) => [...f])).toEqual([
      [1, 2, 3, 4],
      [5, 6, 7, 8],
    ]);
  });
});

describe('binary frames', () => {
  it('builds the frame for the peer: 0x01 then little-endian PCM', () => {
    const bytes = new Uint8Array(audioFrame(Int16Array.from([1, -2, 0x1234])));
    expect([...bytes]).toEqual([0x01, 0x01, 0x00, 0xfe, 0xff, 0x34, 0x12]);
  });

  const serverAudio = (seq: number, ts: number, pcm: number[]) => {
    const buf = new ArrayBuffer(9 + pcm.length * 2);
    const v = new DataView(buf);
    v.setUint8(0, 0x01);
    v.setUint32(1, seq); // the header is big-endian...
    v.setUint32(5, ts);
    pcm.forEach((s, i) => v.setInt16(9 + i * 2, s, true)); // ...the audio is not
    return buf;
  };

  it('reads an audio frame from the server', () => {
    const f = parseServerFrame(serverAudio(7, 5311, [100, -100, 32767]));
    expect(f).toEqual({ kind: 'audio', seq: 7, timestamp: 5311, pcm: Int16Array.from([100, -100, 32767]) });
  });

  it('reads a video frame: the flags, the header and the picture after it', () => {
    const buf = new ArrayBuffer(14);
    const v = new DataView(buf);
    v.setUint8(0, 0x02);
    v.setUint8(1, 0b0000_0111); // keyframe, three quarter turns
    v.setUint32(2, 3);
    v.setUint32(6, 900);
    new Uint8Array(buf, 10).set([0, 0, 0, 1]);
    const f = parseServerFrame(buf);
    expect(f).toMatchObject({ kind: 'video', seq: 3, timestamp: 900, keyframe: true, orientation: 3 });
    expect(f?.kind === 'video' && [...f.data]).toEqual([0, 0, 0, 1]);
  });

  it('reads a frame that is not a keyframe and is upright', () => {
    const buf = new ArrayBuffer(11);
    new DataView(buf).setUint8(0, 0x02);
    expect(parseServerFrame(buf)).toMatchObject({ kind: 'video', keyframe: false, orientation: 0 });
  });

  it('refuses what is not a frame', () => {
    expect(parseServerFrame(new ArrayBuffer(0))).toBeNull();
    expect(parseServerFrame(new ArrayBuffer(5))).toBeNull();
    expect(parseServerFrame(new Uint8Array([0x7f, 1, 2, 3, 4, 5, 6, 7, 8, 9]).buffer)).toBeNull();
  });
});

describe('PlayoutClock', () => {
  it('starts the first frame a little in the future', () => {
    const c = new PlayoutClock(0.15);
    expect(c.startAt(1000, 0.06, 10)).toBeCloseTo(10.15, 6);
  });

  it('plays a frame at its own offset, so a pause stays a pause', () => {
    const c = new PlayoutClock(0.15);
    const first = c.startAt(1000, 0.06, 10);
    const later = c.startAt(1400, 0.06, 10.1); // 400 ms after the first, as the peer sent it
    expect(later - first).toBeCloseTo(0.4, 6);
  });

  it('never overlaps: a burst of frames with one stamp is played one after the other', () => {
    const c = new PlayoutClock(0.15);
    const a = c.startAt(2000, 0.06, 10);
    const b = c.startAt(2000, 0.06, 10);
    const d = c.startAt(2010, 0.06, 10);
    expect(b).toBeCloseTo(a + 0.06, 6);
    expect(d).toBeCloseTo(b + 0.06, 6);
  });

  it('starts over when it has fallen far behind (a tab that was in the background)', () => {
    const c = new PlayoutClock(0.15, 0.5);
    c.startAt(1000, 0.06, 10);
    const start = c.startAt(1100, 0.06, 20); // 10 s late
    expect(start).toBeCloseTo(20.15, 6);
  });

  it('plays a frame that is only a little late at once, without moving the clock', () => {
    const c = new PlayoutClock(0.15, 0.5);
    const first = c.startAt(1000, 0.06, 10); // 10.15
    const late = c.startAt(1100, 0.06, 10.4); // due at 10.25, 150 ms late
    expect(late).toBeCloseTo(10.4, 6);
    expect(c.startAt(1500, 0.06, 10.45) - first).toBeCloseTo(0.5, 6); // still on the original timeline
  });

  it('can be reset', () => {
    const c = new PlayoutClock(0.15);
    c.startAt(5000, 0.06, 10);
    c.reset();
    expect(c.startAt(100, 0.06, 50)).toBeCloseTo(50.15, 6);
  });
});

describe('display helpers', () => {
  it('formats a clock', () => {
    expect(formatClock(0)).toBe('00:00');
    expect(formatClock(187.9)).toBe('03:07');
    expect(formatClock(3723)).toBe('1:02:03');
    expect(formatClock(-5)).toBe('00:00');
  });

  it('counts elapsed time without going negative', () => {
    const now = Date.parse('2026-10-02T12:00:30Z');
    expect(elapsedSeconds('2026-10-02T12:00:00Z', now)).toBe(30);
    expect(elapsedSeconds('2026-10-02T12:01:00Z', now)).toBe(0);
    expect(elapsedSeconds('not a date', now)).toBe(0);
  });

  it('names the other side as well as it can', () => {
    expect(peerLabel('273117121392855@lid', '553197157574')).toBe('+55 31 9715-7574');
    expect(peerLabel('553197157574@s.whatsapp.net')).toBe('+55 31 9715-7574');
    expect(peerLabel('273117121392855@lid')).toBe('Contato oculto (…2855)');
    expect(peerLabel('')).toBe('—');
  });

  it('builds the address of the stream from the address of the API', () => {
    expect(streamWsUrl('http://localhost:8100', '/call/stream/ABC?ticket=xyz')).toBe('ws://localhost:8100/call/stream/ABC?ticket=xyz');
    expect(streamWsUrl('https://host.example/evo/', '/call/stream/ABC?ticket=xyz')).toBe('wss://host.example/evo/call/stream/ABC?ticket=xyz');
  });

  it('gives the level bar a range that speech moves through', () => {
    expect(levelPercent(0)).toBe(0);
    expect(levelPercent(0.0005)).toBe(0); // under -60 dBFS
    expect(levelPercent(1)).toBe(100);
    expect(levelPercent(0.05)).toBeGreaterThan(levelPercent(0.01));
    expect(levelPercent(0.05)).toBeGreaterThan(40);
    expect(levelPercent(0.05)).toBeLessThan(90);
  });

  it('says what the other side\'s camera is doing', () => {
    expect(peerVideoLabel('enabled')).toBe('Câmera do outro lado ligada');
    expect(peerVideoLabel('disabled')).toBe(peerVideoLabel('stopped'));
    expect(peerVideoLabel('upgrade_request')).toContain('pede');
    expect(peerVideoLabel('unknown')).toBeNull();
    expect(peerVideoLabel(undefined)).toBeNull();
  });

  it('says why a call ended in words', () => {
    expect(reasonLabel('peer_hangup')).toBe('O outro lado desligou');
    expect(reasonLabel('server:503')).toContain('servidor');
    expect(reasonLabel('something new')).toBe('something new');
    expect(reasonLabel('')).toBe('—');
  });
});
