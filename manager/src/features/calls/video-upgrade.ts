import type { PeerVideoState } from './softphone';

/**
 * The other side asks to turn a voice call into a video call and then waits for the answer: the
 * iPhone shows its camera while it waits and gives up after a few seconds, going back to voice.
 * Tried live: accepted half a second after the request, the video came; accepted six seconds after,
 * the phone had already withdrawn it (its video state turned to "disabled") and no picture arrived.
 * A person reading a notice and clicking takes longer than that, so by default the panel accepts at once.
 */
export const AUTO_ACCEPT_KEY = 'evolution-calls-auto-accept-video';

/** Whether requests for video are accepted without waiting for a click (on unless the person turned it off). */
export function readAutoAccept(): boolean {
  try {
    return localStorage.getItem(AUTO_ACCEPT_KEY) !== 'off';
  } catch {
    return true; // storage blocked: the default, which is the one that works
  }
}

export function saveAutoAccept(on: boolean) {
  try {
    localStorage.setItem(AUTO_ACCEPT_KEY, on ? 'on' : 'off');
  } catch {
    /* the choice is just not remembered */
  }
}

/**
 * Whether a request for video that just arrived should be accepted now. `handled` is the last
 * request already dealt with, so that one request is accepted once and not at every render.
 */
export function shouldAutoAccept(peer: PeerVideoState | null, handled: PeerVideoState | null, live: boolean, enabled: boolean): boolean {
  return enabled && live && peer?.state === 'upgrade_request' && peer !== handled;
}
