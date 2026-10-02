package call_stream

import (
	"testing"
	"time"
)

var t0 = time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)

// level is a 60 ms frame (16 kHz) at the given level: a constant, so its rms is the level.
func level(v float32) []float32 {
	f := make([]float32, frameSamples)
	for i := range f {
		f[i] = v
	}
	return f
}

const frameLen = 60 * time.Millisecond

// speak feeds n frames of speech from at, one every 60 ms, and returns the events.
func speak(v *vad, from time.Time, n int, lvl float32) (events []*speechEvent, next time.Time) {
	at := from
	for i := 0; i < n; i++ {
		if ev := v.Frame(at, level(lvl)); ev != nil {
			events = append(events, ev)
		}
		at = at.Add(frameLen)
	}
	return events, at
}

func TestComfortNoiseIsNotSpeech(t *testing.T) {
	v := newVAD()
	// what a silent or muted peer sends: two or three frames a second at -74 dBFS
	for i := 0; i < 20; i++ {
		if ev := v.Frame(t0.Add(time.Duration(i)*400*time.Millisecond), level(0.0002)); ev != nil {
			t.Fatalf("comfort noise started speech: %+v", ev)
		}
	}
}

func TestSpeechStartsAfterAFewFramesAndIsToldOnce(t *testing.T) {
	v := newVAD()
	events, _ := speak(v, t0, 10, 0.1)
	if len(events) != 1 || !events[0].start {
		t.Fatalf("events = %+v, want one start", events)
	}
	// it began with the first frame of the run, not with the one that made up the minimum
	if !events[0].at.Equal(t0) {
		t.Fatalf("speech began at %v, want %v", events[0].at, t0)
	}
}

func TestAClickIsNotSpeech(t *testing.T) {
	v := newVAD()
	v.Frame(t0, level(0.0002))
	if ev := v.Frame(t0.Add(frameLen), level(0.2)); ev != nil { // one loud frame
		t.Fatalf("one frame started speech: %+v", ev)
	}
	for i := 2; i < 12; i++ {
		if ev := v.Frame(t0.Add(time.Duration(i)*frameLen), level(0.0002)); ev != nil {
			t.Fatalf("a click turned into speech: %+v", ev)
		}
	}
}

func TestSpeechEndsAfterTheHangoverEvenWhenTheNextFramesAreFar(t *testing.T) {
	v := newVAD()
	_, next := speak(v, t0, 10, 0.1) // 600 ms of speech
	last := next                     // the end of the last frame of speech

	// a quiet peer sends a frame of comfort noise now and then, not one every 60 ms
	if ev := v.Frame(last.Add(300*time.Millisecond), level(0.0002)); ev != nil {
		t.Fatalf("speech ended after only 300 ms: %+v", ev)
	}
	ev := v.Frame(last.Add(700*time.Millisecond), level(0.0002))
	if ev == nil || ev.start {
		t.Fatalf("speech did not end after the hangover: %+v", ev)
	}
	// it ended where it was last heard, not when the next frame happened to arrive
	if !ev.at.Equal(last) || ev.length != last.Sub(t0) {
		t.Fatalf("end = %v after %v, want %v after %v", ev.at, ev.length, last, last.Sub(t0))
	}
}

// With nothing arriving at all (a muted peer sends nothing for a while) the end has to be
// found by the clock.
func TestSpeechEndsOnTheClockWhenNoAudioArrives(t *testing.T) {
	v := newVAD()
	_, last := speak(v, t0, 8, 0.1)

	if ev := v.Tick(last.Add(500 * time.Millisecond)); ev != nil {
		t.Fatalf("ended after 500 ms: %+v", ev)
	}
	ev := v.Tick(last.Add(650 * time.Millisecond))
	if ev == nil || ev.start || !ev.at.Equal(last) {
		t.Fatalf("tick at the hangover: %+v", ev)
	}
	if again := v.Tick(last.Add(5 * time.Second)); again != nil {
		t.Fatalf("the end was told twice: %+v", again)
	}
}

func TestAPauseBetweenWordsDoesNotEndSpeech(t *testing.T) {
	v := newVAD()
	_, at := speak(v, t0, 6, 0.1)
	for i := 0; i < 5; i++ { // 300 ms of silence
		if ev := v.Frame(at, level(0.0002)); ev != nil {
			t.Fatalf("a short pause ended speech: %+v", ev)
		}
		at = at.Add(frameLen)
	}
	if events, _ := speak(v, at, 6, 0.1); len(events) != 0 {
		t.Fatalf("the words after the pause were told as new speech: %+v", events)
	}
}

func TestTheNextSentenceIsNewSpeech(t *testing.T) {
	v := newVAD()
	_, at := speak(v, t0, 8, 0.1)
	if ev := v.Tick(at.Add(time.Second)); ev == nil || ev.start {
		t.Fatalf("first speech did not end: %+v", ev)
	}
	events, _ := speak(v, at.Add(2*time.Second), 6, 0.1)
	if len(events) != 1 || !events[0].start || !events[0].at.Equal(at.Add(2*time.Second)) {
		t.Fatalf("second speech = %+v", events)
	}
}

// A room with a fan is louder than a quiet one: the bar for speech rises with it.
func TestANoisyRoomRaisesTheBar(t *testing.T) {
	v := newVAD()
	at := t0
	for i := 0; i < 100; i++ { // a few seconds of a room at 0.006, under the floor
		v.Frame(at, level(0.006))
		at = at.Add(frameLen)
	}
	if events, next := speak(v, at, 8, 0.015); len(events) != 0 { // louder than the floor, not than the room
		t.Fatalf("a murmur over a noisy room was taken for speech: %+v", events)
	} else {
		at = next
	}
	if events, _ := speak(v, at, 8, 0.2); len(events) != 1 || !events[0].start {
		t.Fatalf("a voice over a noisy room was not heard: %+v", events)
	}
}

func TestAQuietRoomLetsASoftVoiceThrough(t *testing.T) {
	v := newVAD()
	for i := 0; i < 50; i++ {
		v.Frame(t0.Add(time.Duration(i)*frameLen), level(0.0002))
	}
	if events, _ := speak(v, t0.Add(time.Hour), 8, 0.02); len(events) != 1 { // -34 dBFS
		t.Fatalf("a soft voice was not heard: %+v", events)
	}
}

func TestCloseEndsTheSpeechThereIs(t *testing.T) {
	v := newVAD()
	if ev := v.Close(); ev != nil {
		t.Fatalf("closing with no speech told %+v", ev)
	}
	_, last := speak(v, t0, 8, 0.1)
	ev := v.Close()
	if ev == nil || ev.start || !ev.at.Equal(last) {
		t.Fatalf("close = %+v", ev)
	}
	if again := v.Close(); again != nil {
		t.Fatalf("closed twice: %+v", again)
	}
}

func TestAFrameOfAnySizeCountsForItsLength(t *testing.T) {
	v := newVAD()
	// the decoder may hand over more than one frame at a time: 3 frames in one piece
	big := make([]float32, 3*frameSamples)
	for i := range big {
		big[i] = 0.1
	}
	ev := v.Frame(t0, big)
	if ev == nil || !ev.start {
		t.Fatalf("a long loud frame did not start speech: %+v", ev)
	}
	if v.Frame(t0, nil) != nil {
		t.Fatal("an empty frame was an event")
	}
}
