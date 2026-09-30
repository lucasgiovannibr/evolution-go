#!/usr/bin/env python3
"""Live test of the call stream (GET /call/stream/{callId}): audio, and optionally video.

Needs `pip install websockets`. The instance must have callsEnabled turned on and
must have been (re)connected after that.

Incoming (default): call the instance's number from another phone. The script waits
for the ringing call, opens the stream, answers, and

  * records what the caller says into a WAV file (16 kHz mono), and
  * echoes it back (--echo) and/or plays a 440 Hz tone (--tone SECONDS),

so you can hear both directions work. Ctrl+C hangs the call up.

Outgoing: --dial NUMBER places the call, connects the stream before the other phone
rings, and does the same once it is picked up.

Video: --video also carries the call's video. What the peer sends is written to
<name>.h264 (play it with `ffplay file.h264`). --video-in FILE sends that H.264 file
(Annex-B, see below) to the peer at --fps; it starts over from its first keyframe
whenever WhatsApp asks for one. With --dial, --video places a video call; on an audio
call, --video-in turns the video on after the call is up: --video-mode start (default)
sends the upgrade request of older WhatsApp versions, --video-mode enable just turns
the camera on, which is what current WhatsApp does. A peer's upgrade request is accepted
automatically; a peer that simply turns its camera on needs nothing.

    python call-stream-test.py --apikey TOKEN --echo
    python call-stream-test.py --apikey TOKEN --dial 5511999990000 --tone 3
    python call-stream-test.py --apikey TOKEN --dial 5511999990000 --video --video-in test.h264

A test file (constrained baseline, one slice per picture, headers repeated on every
keyframe, which is what the stream expects):

    ffmpeg -f lavfi -i testsrc=size=640x360:rate=15 -t 10 -c:v libx264 -profile:v baseline \\
      -x264-params keyint=30:repeat-headers=1:slices=1:bframes=0 -f h264 test.h264
"""
import argparse
import asyncio
import base64
import json
import math
import re
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


START_CODE = re.compile(b"\x00\x00\x01")


def access_units(data):
    """Split an Annex-B H.264 file into access units, one per picture.

    A picture ends at its slice NAL (type 1 or 5); the SPS, PPS, SEI and delimiter NALs in
    front of it belong to it. Right for streams with one slice per picture.
    """
    starts = [m.start() for m in START_CODE.finditer(data)]
    units, pending = [], b""
    for i, at in enumerate(starts):
        end = starts[i + 1] if i + 1 < len(starts) else len(data)
        nal = data[at:end].rstrip(b"\x00")
        nal = b"\x00\x00\x00\x01" + nal[3:]
        pending += nal
        if nal[4] & 0x1F in (1, 5):
            units.append(pending)
            pending = b""
    return units


def is_keyframe(au):
    return any(m.end() < len(au) and au[m.end()] & 0x1F == 5 for m in START_CODE.finditer(au))


async def main(args):
    outgoing = bool(args.dial)
    want_video = args.video or bool(args.video_in)
    if outgoing:
        status, call = api(
            args.base, args.apikey, "POST", "/call/dial",
            {"number": args.dial, "stream": True, "video": want_video},
        )
        if status != 200:
            sys.exit(f"dial failed: {status} {call}")
        ticket = call["streamTicket"]
        print(f"Calling {args.dial}... (call {call['callId']}, phase {call['phase']}, video {call['video']})")
    else:
        call = {"callId": args.call_id} if args.call_id else wait_for_ringing_call(args.base, args.apikey)
        print(f"Call {call['callId']} from {call.get('peer')} (video={call.get('video')})")
        status, ticket = api(
            args.base, args.apikey, "POST", "/call/stream-ticket",
            {"callId": call["callId"], "video": want_video},
        )
        if status != 200:
            sys.exit(f"stream-ticket failed: {status} {ticket}")
    call_id = call["callId"]
    ws_url = "ws" + args.base[4:] + ticket["path"]

    def control(action, **extra):
        status, body = api(args.base, args.apikey, "POST", "/call/video", {"callId": call_id, "action": action, **extra})
        print(f"video {action}: {status}" + ("" if status == 200 else f" {body}"))

    wav_path = args.record or f"call-{call_id}.wav"
    h264_path = wav_path.rsplit(".", 1)[0] + ".h264"
    received = video_units = keyframes = 0
    video_file = open(h264_path, "wb") if want_video else None
    restart = asyncio.Event()

    async with websockets.connect(ws_url, max_size=4 << 20) as ws, _wav(wav_path) as wav:
        start = json.loads(await ws.recv())
        print("start:", {k: v for k, v in start.items() if k != "event"})

        # The stream is attached while the call still rings, so no audio is lost.
        if not outgoing:
            status, body = api(args.base, args.apikey, "POST", "/call/answer", {"callId": call_id})
            if status != 200:
                sys.exit(f"answer failed: {status} {body}")
            print("answered; phase:", body.get("phase"))
        else:
            print("stream open; waiting for the other side to pick up (Ctrl+C hangs up)")

        async def send_tone():
            for frame in tone_frames(args.tone):
                await ws.send(json.dumps({"event": "media", "payload": base64.b64encode(frame).decode()}))
                await asyncio.sleep(FRAME_MS / 1000)

        async def send_video(units):
            await asyncio.sleep(2)  # let the call come up
            if not start.get("video"):
                if args.video_mode == "enable":
                    print("the call is audio only: turning the camera on")
                else:
                    print("the call is audio only: asking the peer to upgrade to video")
                control(args.video_mode)
                await asyncio.sleep(3)
            i, sent = 0, 0
            while True:
                if restart.is_set():
                    restart.clear()
                    i = 0
                    print("WhatsApp asked for a keyframe: starting the file over")
                await ws.send(json.dumps({"event": "video", "payload": base64.b64encode(units[i]).decode()}))
                sent += 1
                i = (i + 1) % len(units)
                await asyncio.sleep(1 / args.fps)

        tasks = []
        if args.tone > 0:
            tasks.append(asyncio.create_task(send_tone()))
        if args.video_in:
            units = access_units(open(args.video_in, "rb").read())
            if not units or not is_keyframe(units[0]):
                sys.exit(f"{args.video_in}: expected Annex-B H.264 starting with a keyframe (see --help)")
            print(f"sending {len(units)} pictures from {args.video_in} at {args.fps} fps")
            tasks.append(asyncio.create_task(send_video(units)))

        try:
            async for raw in ws:
                msg = json.loads(raw)
                event = msg["event"]
                if event == "media":
                    pcm = base64.b64decode(msg["payload"])
                    wav.writeframes(pcm)
                    received += len(pcm) // 2
                    if args.echo:
                        await ws.send(json.dumps({"event": "media", "payload": msg["payload"]}))
                elif event == "video":
                    video_file.write(base64.b64decode(msg["payload"]))
                    video_units += 1
                    if msg.get("keyframe"):
                        keyframes += 1
                        if keyframes == 1:
                            print(f"first video keyframe (orientation {msg.get('orientation')})")
                elif event == "keyframe_request":
                    restart.set()
                elif event == "video_state":
                    print("peer video state:", {k: msg.get(k) for k in ("state", "active", "upgrade", "orientation")})
                    if msg.get("upgrade"):
                        control("accept")
                elif event == "stop":
                    print("call ended:", msg.get("reason"))
                    break
                elif event == "error":
                    print("stream error:", msg.get("code"), msg.get("message"))
        except asyncio.CancelledError:
            api(args.base, args.apikey, "POST", "/call/hangup", {"callId": call_id})
            print("hung up")
            raise
        finally:
            for t in tasks:
                t.cancel()
            if video_file:
                video_file.close()
    print(f"received {received / SAMPLE_RATE:.1f} s of audio -> {wav_path}")
    if want_video:
        print(f"received {video_units} video pictures ({keyframes} keyframes) -> {h264_path}")


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
    p.add_argument("--dial", metavar="NUMBER", help="place a call to NUMBER instead of waiting for one")
    p.add_argument("--record", help="WAV file for the caller's audio (default call-<id>.wav)")
    p.add_argument("--echo", action="store_true", help="send the caller's audio back")
    p.add_argument("--tone", type=float, default=0, metavar="SECONDS", help="play a 440 Hz tone")
    p.add_argument("--video", action="store_true", help="carry the call's video too (with --dial: place a video call)")
    p.add_argument("--video-in", metavar="FILE", help="send this Annex-B H.264 file as video (implies --video)")
    p.add_argument("--video-mode", choices=("start", "enable"), default="start",
                   help="how --video-in begins on an audio call: ask the peer to upgrade (start) "
                        "or just turn the camera on (enable)")
    p.add_argument("--fps", type=float, default=15, help="pictures per second of --video-in")
    try:
        asyncio.run(main(p.parse_args()))
    except KeyboardInterrupt:
        pass
