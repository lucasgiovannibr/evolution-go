import { afterEach, describe, expect, it } from 'vitest';
import type { PeerVideoState } from './softphone';
import { AUTO_ACCEPT_KEY, readAutoAccept, saveAutoAccept, shouldAutoAccept } from './video-upgrade';

const request: PeerVideoState = { state: 'upgrade_request', active: false, upgrade: true };
const enabled: PeerVideoState = { state: 'enabled', active: true, upgrade: false };

describe('shouldAutoAccept', () => {
  it('accepts a request for video on a live call when it is on', () => {
    expect(shouldAutoAccept(request, null, true, true)).toBe(true);
  });

  it('does nothing when the person turned it off', () => {
    expect(shouldAutoAccept(request, null, true, false)).toBe(false);
  });

  it('waits for the audio to be live: answering comes first', () => {
    expect(shouldAutoAccept(request, null, false, true)).toBe(false);
  });

  it('only answers a request, not the other states', () => {
    expect(shouldAutoAccept(enabled, null, true, true)).toBe(false);
    expect(shouldAutoAccept({ state: 'upgrade_cancelled', active: false, upgrade: false }, null, true, true)).toBe(false);
    expect(shouldAutoAccept(null, null, true, true)).toBe(false);
  });

  it('accepts one request once', () => {
    expect(shouldAutoAccept(request, request, true, true)).toBe(false);
    // a new request is a new object
    expect(shouldAutoAccept({ ...request }, request, true, true)).toBe(true);
  });
});

describe('the choice is remembered', () => {
  const store = new Map<string, string>();
  const original = Object.getOwnPropertyDescriptor(globalThis, 'localStorage');

  const install = () =>
    Object.defineProperty(globalThis, 'localStorage', {
      configurable: true,
      value: { getItem: (k: string) => store.get(k) ?? null, setItem: (k: string, v: string) => void store.set(k, v) },
    });

  afterEach(() => {
    store.clear();
    if (original) Object.defineProperty(globalThis, 'localStorage', original);
    else Reflect.deleteProperty(globalThis, 'localStorage');
  });

  it('is on by default', () => {
    install();
    expect(readAutoAccept()).toBe(true);
  });

  it('keeps what was chosen', () => {
    install();
    saveAutoAccept(false);
    expect(store.get(AUTO_ACCEPT_KEY)).toBe('off');
    expect(readAutoAccept()).toBe(false);
    saveAutoAccept(true);
    expect(readAutoAccept()).toBe(true);
  });

  it('falls back to on, and does not throw, when storage is not there', () => {
    Reflect.deleteProperty(globalThis, 'localStorage');
    expect(readAutoAccept()).toBe(true);
    expect(() => saveAutoAccept(false)).not.toThrow();
  });
});
