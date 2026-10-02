package call_stream

import (
	"math"
	"math/rand"
	"testing"
)

// tone is seconds of a sine at freq Hz, sampled at rate, with the given peak.
func tone(freq, peak float64, rate int, seconds float64) []float32 {
	out := make([]float32, int(float64(rate)*seconds))
	for i := range out {
		out[i] = float32(peak * math.Sin(2*math.Pi*freq*float64(i)/float64(rate)))
	}
	return out
}

// rms of the samples after the first skip (the filter's start-up).
func rms(x []float32, skip int) float64 {
	var sum float64
	n := 0
	for _, v := range x[skip:] {
		sum += float64(v) * float64(v)
		n++
	}
	return math.Sqrt(sum / float64(n))
}

func TestResamplerKeepsTheVoiceBandInEveryDirection(t *testing.T) {
	const peak = 0.5
	want := peak / math.Sqrt2
	for _, c := range []struct{ in, out int }{{16000, 8000}, {8000, 16000}, {16000, 24000}, {24000, 16000}} {
		r := newResampler(c.in, c.out)
		got := r.Process(tone(1000, peak, c.in, 1))
		if level := rms(got, 200); math.Abs(level-want) > 0.02*want {
			t.Errorf("%d -> %d Hz: a 1 kHz tone came out with rms %.4f, want %.4f", c.in, c.out, level, want)
		}
		// one second of input is one second of output, less the filter's small delay
		if math.Abs(float64(len(got)-c.out)) > 64 {
			t.Errorf("%d -> %d Hz: one second became %d samples, want about %d", c.in, c.out, len(got), c.out)
		}
	}
}

// 6 kHz is above what 8 kHz audio can hold: it must be removed, not folded back down to
// 2 kHz as a whistle.
func TestResamplerDoesNotAliasWhatTheLowerRateCannotHold(t *testing.T) {
	r := newResampler(16000, 8000)
	got := r.Process(tone(6000, 0.5, 16000, 1))
	if level := rms(got, 200); level > 0.01 {
		t.Fatalf("a 6 kHz tone leaked into 8 kHz audio with rms %.4f (the tone is 0.354)", level)
	}
}

func TestResamplerKeepsTheLevelOfSteadyAudio(t *testing.T) {
	r := newResampler(16000, 24000)
	in := make([]float32, 8000)
	for i := range in {
		in[i] = 0.25
	}
	got := r.Process(in)
	for i := 200; i < len(got)-200; i++ {
		if math.Abs(float64(got[i])-0.25) > 0.005 {
			t.Fatalf("sample %d = %v, want 0.25", i, got[i])
		}
	}
}

// A stream is cut into whatever chunks the library or the client produces; where the
// cuts fall must not change a single output sample.
func TestResamplerOutputDoesNotDependOnHowTheInputIsCut(t *testing.T) {
	in := tone(700, 0.4, 16000, 2)
	for _, c := range []struct{ in, out int }{{16000, 8000}, {16000, 24000}} {
		whole := newResampler(c.in, c.out).Process(in)

		rnd := rand.New(rand.NewSource(1))
		r := newResampler(c.in, c.out)
		var pieces []float32
		for rest := in; len(rest) > 0; {
			n := min(1+rnd.Intn(1500), len(rest))
			pieces = append(pieces, r.Process(rest[:n])...)
			rest = rest[n:]
		}
		if len(pieces) != len(whole) {
			t.Fatalf("%d -> %d: %d samples in pieces, %d at once", c.in, c.out, len(pieces), len(whole))
		}
		for i := range whole {
			if pieces[i] != whole[i] {
				t.Fatalf("%d -> %d: sample %d differs (%v vs %v)", c.in, c.out, i, pieces[i], whole[i])
			}
		}
	}
}

func TestNoResamplerIsNeededForTheSameRate(t *testing.T) {
	if newResampler(16000, 16000) != nil {
		t.Fatal("a resampler was built for equal rates")
	}
}

// The state it keeps must not grow with the length of the stream.
func TestResamplerForgetsInputItNoLongerNeeds(t *testing.T) {
	r := newResampler(16000, 8000)
	chunk := tone(300, 0.3, 16000, 0.06)
	for i := 0; i < 2000; i++ {
		r.Process(chunk)
	}
	if len(r.buf) > 4*r.taps+len(chunk) {
		t.Fatalf("the buffer holds %d samples after two minutes of audio", len(r.buf))
	}
}
