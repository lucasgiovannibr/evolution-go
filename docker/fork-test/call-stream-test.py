#!/usr/bin/env python3
"""Live test of the call audio stream (GET /call/stream/{callId}).

Needs `pip install websockets`. The instance must have callsEnabled turned on and
must have been (re)connected after that. Call the instance's number from another
phone: the script waits for the ringing call, opens the audio stream, answers, and

  * records what the caller says into a WAV file (16 kHz mono), and
  * echoes it back (--echo) and/or plays a 440 Hz tone (--tone SECONDS),

so you can hear both directions work. Ctrl+C hangs the call up.

    python call-stream-test.py --base http://localhost:4000 --apikey INSTANCE_TOKEN --echo
"""
import argparse
import asyncio
import base64
import json
import math
import struct
import sys
import time
import urllib.error
import urllib.request
import wave

import websockets

SAMPLE_RATE = 16000
FRAME_MS = 60
FRAME_SAMPLES = SAMPLE_RATE * FRAME_MS // 1000


def api(base, apikey, method, path, body=None):
    data = json.dumps(body).encode() if body is not None else None
    req = urllib.request.Request(base + path, data=data, method=method)
    req.add_header("apikey", apikey)
    req.add_header("Content-Type", "application/json")
    try:
        with urllib.request.urlopen(req, timeout=10) as resp:
            return resp.status, json.loads(resp.read() or b"{}")
    except urllib.error.HTTPError as e:
        try:
            return e.code, json.loads(e.read() or b"{}")
        except ValueError:
            return e.code, {}


def wait_for_ringing_call(base, apikey):
    print("Waiting for an incoming call... (call the instance's number now)")
    shown = False
    while True:
        status, body = api(base, apikey, "GET", "/call/active")
        if status != 200:
            sys.exit(f"GET /call/active failed: {status} {body}")
        if not body.get("enabled"):
            sys.exit(
                "The instance has no working call engine: "
                f"state={body.get('state')!r} error={body.get('error')!r}. "
                "Turn on callsEnabled and reconnect the instance."
            )
        for call in body.get("calls", []):
            if call["direction"] == "incoming" and call["phase"] == "ringing":
                return call
        if not shown:
            print("  (engine is active, nobody is calling yet)")
            shown = True
        time.sleep(1)


def tone_frames(seconds):
    total = int(seconds * SAMPLE_RATE)
    pcm = b"".join(
        struct.pack("<h", int(12000 * math.sin(2 * math.pi * 440 * i / SAMPLE_RATE)))
        for i in range(total)
    )
    step = FRAME_SAMPLES * 2
    return [pcm[i : i + step] for i in range(0, len(pcm), step)]


async def main(args):
    call = {"callId": args.call_id} if args.call_id else wait_for_ringing_call(args.base, args.apikey)
    call_id = call["callId"]
    print(f"Call {call_id} from {call.get('peer')} (video={call.get('video')})")

    status, ticket = api(args.base, args.apikey, "POST", "/call/stream-ticket", {"callId": call_id})
    if status != 200:
        sys.exit(f"stream-ticket failed: {status} {ticket}")
    ws_url = "ws" + args.base[4:] + ticket["path"]

    wav_path = args.record or f"call-{call_id}.wav"
    received = 0
    async with websockets.connect(ws_url, max_size=1 << 20) as ws, _wav(wav_path) as wav:
        start = json.loads(await ws.recv())
        print("start:", {k: v for k, v in start.items() if k != "event"})

        # The stream is attached while the call still rings, so no audio is lost.
        status, body = api(args.base, args.apikey, "POST", "/call/answer", {"callId": call_id})
        if status != 200:
            sys.exit(f"answer failed: {status} {body}")
        print("answered; phase:", body.get("phase"))

        async def send_tone():
            for frame in tone_frames(args.tone):
                await ws.send(json.dumps({"event": "media", "payload": base64.b64encode(frame).decode()}))
                await asyncio.sleep(FRAME_MS / 1000)

        tone_task = asyncio.create_task(send_tone()) if args.tone > 0 else None
        try:
            async for raw in ws:
                msg = json.loads(raw)
                if msg["event"] == "media":
                    pcm = base64.b64decode(msg["payload"])
                    wav.writeframes(pcm)
                    received += len(pcm) // 2
                    if args.echo:
                        await ws.send(json.dumps({"event": "media", "payload": msg["payload"]}))
                elif msg["event"] == "stop":
                    print("call ended:", msg.get("reason"))
                    break
                elif msg["event"] == "error":
                    print("stream error:", msg.get("code"), msg.get("message"))
        except asyncio.CancelledError:
            api(args.base, args.apikey, "POST", "/call/hangup", {"callId": call_id})
            print("hung up")
            raise
        finally:
            if tone_task:
                tone_task.cancel()
    print(f"received {received / SAMPLE_RATE:.1f} s of audio -> {wav_path}")


class _wav:
    def __init__(self, path):
        self.path = path

    async def __aenter__(self):
        self.w = wave.open(self.path, "wb")
        self.w.setnchannels(1)
        self.w.setsampwidth(2)
        self.w.setframerate(SAMPLE_RATE)
        return self.w

    async def __aexit__(self, *exc):
        self.w.close()


if __name__ == "__main__":
    p = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    p.add_argument("--base", default="http://localhost:4000", help="server address")
    p.add_argument("--apikey", required=True, help="the instance token")
    p.add_argument("--call-id", help="use this call instead of waiting for a ringing one")
    p.add_argument("--record", help="WAV file for the caller's audio (default call-<id>.wav)")
    p.add_argument("--echo", action="store_true", help="send the caller's audio back")
    p.add_argument("--tone", type=float, default=0, metavar="SECONDS", help="play a 440 Hz tone")
    try:
        asyncio.run(main(p.parse_args()))
    except KeyboardInterrupt:
        pass
