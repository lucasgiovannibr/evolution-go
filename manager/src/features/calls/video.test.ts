import { describe, expect, it } from 'vitest';
import { codecOf, isKeyframe, NAL_PPS, NAL_SLICE_IDR, NAL_SPS, rotation, splitNals } from './video';

const bytes = (...parts: number[][]) => Uint8Array.from(parts.flat());

const START4 = [0, 0, 0, 1];
const START3 = [0, 0, 1];
// a baseline SPS: profile 0x42, constraint flags 0xc0, level 3.0 (0x1e), then some payload
const SPS = [0x67, 0x42, 0xc0, 0x1e, 0xda, 0x05, 0x07, 0xe8];
const PPS = [0x68, 0xce, 0x3c, 0x80];
const IDR = [0x65, 0x88, 0x84, 0x00, 0x10];
const SLICE = [0x41, 0x9a, 0x24, 0x6c];

describe('splitNals', () => {
  it('splits an access unit made of four-byte start codes', () => {
    const au = bytes(START4, SPS, START4, PPS, START4, IDR);
    expect(splitNals(au).map((n) => n.type)).toEqual([NAL_SPS, NAL_PPS, NAL_SLICE_IDR]);
  });

  it('splits one made of three-byte start codes, and a mix of both', () => {
    const au = bytes(START3, SPS, START4, PPS, START3, IDR);
    const nals = splitNals(au);
    expect(nals.map((n) => n.type)).toEqual([NAL_SPS, NAL_PPS, NAL_SLICE_IDR]);
    // a NAL ends where the next start code begins, the leading zero of a four-byte code included
    expect([...au.slice(nals[0]!.start, nals[0]!.end)]).toEqual(SPS);
    expect([...au.slice(nals[1]!.start, nals[1]!.end)]).toEqual(PPS);
    expect([...au.slice(nals[2]!.start, nals[2]!.end)]).toEqual(IDR);
  });

  it('finds nothing in data without a start code', () => {
    expect(splitNals(bytes(SPS))).toEqual([]);
    expect(splitNals(new Uint8Array(0))).toEqual([]);
    expect(splitNals(bytes(START4))).toEqual([]); // a start code with nothing after it
  });
});

describe('isKeyframe', () => {
  it('is true for a picture with an IDR slice', () => {
    expect(isKeyframe(bytes(START4, SPS, START4, PPS, START4, IDR))).toBe(true);
    expect(isKeyframe(bytes(START4, IDR))).toBe(true);
  });

  it('is false for a picture that depends on the ones before it', () => {
    expect(isKeyframe(bytes(START4, SLICE))).toBe(false);
    expect(isKeyframe(new Uint8Array(0))).toBe(false);
  });
});

describe('codecOf', () => {
  it('writes the codec string from profile, constraints and level of the SPS', () => {
    expect(codecOf(bytes(START4, SPS, START4, PPS, START4, IDR))).toBe('avc1.42c01e');
  });

  it('pads small values to two digits', () => {
    const sps = [0x67, 0x4d, 0x00, 0x0a, 0x00];
    expect(codecOf(bytes(START4, sps, START4, IDR))).toBe('avc1.4d000a');
  });

  it('is null without an SPS, or with one too short to read', () => {
    expect(codecOf(bytes(START4, IDR))).toBeNull();
    expect(codecOf(bytes(START4, [0x67, 0x42]))).toBeNull();
  });
});

describe('rotation', () => {
  const apply = (m: number[], x: number, y: number) => [m[0]! * x + m[2]! * y + m[4]!, m[1]! * x + m[3]! * y + m[5]!];

  it('leaves an upright picture alone', () => {
    expect(rotation(320, 212, 0)).toEqual({ width: 320, height: 212, matrix: [1, 0, 0, 1, 0, 0] });
  });

  it('swaps the canvas for a quarter turn either way, and not for a half', () => {
    expect(rotation(320, 212, 1)).toMatchObject({ width: 212, height: 320 });
    expect(rotation(320, 212, 3)).toMatchObject({ width: 212, height: 320 });
    expect(rotation(320, 212, 2)).toMatchObject({ width: 320, height: 212 });
  });

  it('puts the corners where a clockwise turn puts them', () => {
    const w = 320;
    const h = 212;
    // a quarter turn clockwise: the top-left corner goes to the top-right, the top-right to the bottom-right
    const q1 = rotation(w, h, 1).matrix;
    expect(apply(q1, 0, 0)).toEqual([h, 0]);
    expect(apply(q1, w, 0)).toEqual([h, w]);
    // a half turn: top-left to bottom-right
    expect(apply(rotation(w, h, 2).matrix, 0, 0)).toEqual([w, h]);
    // three quarters clockwise (one counter-clockwise): the top-left goes to the bottom-left
    const q3 = rotation(w, h, 3).matrix;
    expect(apply(q3, 0, 0)).toEqual([0, w]);
    expect(apply(q3, w, h)).toEqual([h, 0]);
  });

  it('keeps every corner of the picture inside the canvas', () => {
    for (const turns of [0, 1, 2, 3]) {
      const r = rotation(320, 212, turns);
      for (const [x, y] of [[0, 0], [320, 0], [0, 212], [320, 212]] as const) {
        const [px, py] = apply(r.matrix, x, y);
        expect(px).toBeGreaterThanOrEqual(0);
        expect(px).toBeLessThanOrEqual(r.width);
        expect(py).toBeGreaterThanOrEqual(0);
        expect(py).toBeLessThanOrEqual(r.height);
      }
    }
  });

  it('takes any number of turns modulo four', () => {
    expect(rotation(10, 20, 5)).toEqual(rotation(10, 20, 1));
    expect(rotation(10, 20, -1)).toEqual(rotation(10, 20, 3));
  });
});
