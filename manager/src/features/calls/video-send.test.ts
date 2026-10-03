import { describe, expect, it } from 'vitest';
import { cameraErrorMessage, coverCrop, FRAME_VIDEO, KeyframePolicy, videoFrame, withoutDelimiters } from './video-send';
import { splitNals } from './video';

describe('videoFrame', () => {
  it('puts the type byte in front of the access unit', () => {
    const au = Uint8Array.from([0, 0, 0, 1, 0x65, 0x88]);
    const bytes = new Uint8Array(videoFrame(au));
    expect(bytes[0]).toBe(FRAME_VIDEO);
    expect([...bytes.slice(1)]).toEqual([...au]);
  });

  it('copies: the encoder may reuse its buffer', () => {
    const au = Uint8Array.from([1, 2, 3]);
    const frame = videoFrame(au);
    au[0] = 99;
    expect(new Uint8Array(frame)[1]).toBe(1);
  });
});

describe('withoutDelimiters', () => {
  const START4 = [0, 0, 0, 1];
  const AUD = [0x09, 0xf0];
  const SPS = [0x67, 0x42, 0xc0, 0x1e, 0xda];
  const PPS = [0x68, 0xce, 0x3c, 0x80];
  const IDR = [0x65, 0x88, 0x84, 0x00, 0x10];
  const SLICE = [0x41, 0x9a, 0x24, 0x6c];
  const au = (...nals: number[][]) => Uint8Array.from(nals.flatMap((n) => [...START4, ...n]));

  it('takes the delimiter off a keyframe and keeps the rest in order', () => {
    const out = withoutDelimiters(au(AUD, SPS, PPS, IDR));
    expect([...out]).toEqual([...au(SPS, PPS, IDR)]);
    expect(splitNals(out).map((n) => n.type)).toEqual([7, 8, 5]);
  });

  it('takes it off a picture that is only a slice', () => {
    expect([...withoutDelimiters(au(AUD, SLICE))]).toEqual([...au(SLICE)]);
  });

  it('handles three-byte start codes, and writes four-byte ones', () => {
    const mixed = Uint8Array.from([0, 0, 1, ...AUD, 0, 0, 1, ...SLICE]);
    expect([...withoutDelimiters(mixed)]).toEqual([...au(SLICE)]);
  });

  it('gives back what has no delimiter as it is', () => {
    const plain = au(SPS, PPS, IDR);
    expect(withoutDelimiters(plain)).toBe(plain);
    const empty = new Uint8Array(0);
    expect(withoutDelimiters(empty)).toBe(empty);
  });
});

describe('coverCrop', () => {
  const area = (c: { sw: number; sh: number }) => c.sw / c.sh;

  it('cuts the sides of a landscape camera to fill a portrait picture', () => {
    const c = coverCrop(640, 480, 360, 640);
    expect(c.sh).toBe(480); // the whole height is used
    expect(area(c)).toBeCloseTo(360 / 640, 6);
    expect(c.sx + c.sw / 2).toBeCloseTo(320, 6); // centred
    expect(c.sy).toBe(0);
  });

  it('cuts the top and bottom of a camera that is taller than the picture', () => {
    const c = coverCrop(480, 1000, 360, 640);
    expect(c.sw).toBe(480);
    expect(area(c)).toBeCloseTo(360 / 640, 6);
    expect(c.sy + c.sh / 2).toBeCloseTo(500, 6);
    expect(c.sx).toBe(0);
  });

  it('takes the whole picture when the shapes already match', () => {
    expect(coverCrop(720, 1280, 360, 640)).toEqual({ sx: 0, sy: 0, sw: 720, sh: 1280 });
  });

  it('never goes outside the source', () => {
    for (const [w, h] of [[1920, 1080], [640, 480], [480, 640], [100, 1000], [1000, 100]] as const) {
      const c = coverCrop(w, h, 360, 640);
      expect(c.sx).toBeGreaterThanOrEqual(0);
      expect(c.sy).toBeGreaterThanOrEqual(0);
      expect(c.sx + c.sw).toBeLessThanOrEqual(w + 1e-9);
      expect(c.sy + c.sh).toBeLessThanOrEqual(h + 1e-9);
    }
  });
});

describe('KeyframePolicy', () => {
  it('makes the first picture a keyframe', () => {
    expect(new KeyframePolicy(2000).next(0)).toBe(true);
  });

  it('then waits for the interval', () => {
    const p = new KeyframePolicy(2000);
    expect(p.next(1000)).toBe(true);
    expect(p.next(1066)).toBe(false);
    expect(p.next(2900)).toBe(false);
    expect(p.next(3000)).toBe(true); // 2 s after the last one
    expect(p.next(3066)).toBe(false);
  });

  it('makes the next picture a keyframe when one is asked for, once', () => {
    const p = new KeyframePolicy(2000);
    p.next(0);
    expect(p.next(100)).toBe(false);
    p.request();
    expect(p.next(166)).toBe(true);
    expect(p.next(233)).toBe(false);
  });

  it('counts the interval from the keyframe that was asked for', () => {
    const p = new KeyframePolicy(2000);
    p.next(0);
    p.request();
    p.next(500); // asked for
    expect(p.next(2400)).toBe(false); // 1.9 s later: not due yet
    expect(p.next(2500)).toBe(true);
  });
});

describe('cameraErrorMessage', () => {
  it('says what went wrong in words', () => {
    expect(cameraErrorMessage(new DOMException('x', 'NotAllowedError'))).toContain('negado');
    expect(cameraErrorMessage(new DOMException('x', 'NotFoundError'))).toContain('Nenhuma câmera');
    expect(cameraErrorMessage(new DOMException('x', 'NotReadableError'))).toContain('em uso');
    expect(cameraErrorMessage(new Error('boom'))).toBe('boom');
    expect(cameraErrorMessage(null)).toBe('Não foi possível usar a câmera.');
  });
});
