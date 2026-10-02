package call_stream

import (
	"errors"
	"math"
	"testing"
)

func TestParseAudioFormat(t *testing.T) {
	for _, c := range []struct {
		encoding string
		rate     int
		want     AudioFormat
	}{
		{"", 0, AudioFormat{EncodingPCM, 16000}},
		{"pcm", 24000, AudioFormat{EncodingPCM, 24000}},
		{"audio/pcm-s16le", 8000, AudioFormat{EncodingPCM, 8000}},
		{"", 8000, AudioFormat{EncodingPCM, 8000}},
		{"audio/x-mulaw", 0, AudioFormat{EncodingMulaw, 8000}},
		{"MULAW", 8000, AudioFormat{EncodingMulaw, 8000}},
		{"pcmu", 0, AudioFormat{EncodingMulaw, 8000}},
		{"audio/pcma", 0, AudioFormat{EncodingAlaw, 8000}},
		{" alaw ", 8000, AudioFormat{EncodingAlaw, 8000}},
	} {
		got, err := ParseAudioFormat(c.encoding, c.rate)
		if err != nil || got != c.want {
			t.Errorf("ParseAudioFormat(%q, %d) = %+v, %v; want %+v", c.encoding, c.rate, got, err, c.want)
		}
	}

	for _, c := range []struct {
		encoding string
		rate     int
	}{
		{"opus", 0},      // not carried
		{"audio/mp3", 0}, // not carried
		{"pcm", 44100},   // rate not offered
		{"pcm", -16000},
		{"mulaw", 16000}, // G.711 is 8 kHz
		{"alaw", 24000},
	} {
		if _, err := ParseAudioFormat(c.encoding, c.rate); !errors.Is(err, ErrInvalidAudioFormat) {
			t.Errorf("ParseAudioFormat(%q, %d) = %v, want ErrInvalidAudioFormat", c.encoding, c.rate, err)
		}
	}
}

func TestTheZeroFormatIsTheDefault(t *testing.T) {
	if !(AudioFormat{}).isDefault() || !DefaultAudioFormat.isDefault() {
		t.Fatal("the default format is not recognised as such")
	}
	if (AudioFormat{EncodingPCM, 8000}).isDefault() || (AudioFormat{EncodingMulaw, 8000}).isDefault() {
		t.Fatal("a converted format counts as the default")
	}
}

func TestG711KnownValues(t *testing.T) {
	if got := linearToMulaw(0); got != 0xFF {
		t.Errorf("mu-law of silence = %#x, want 0xFF", got)
	}
	if got := mulawToLinear(0xFF); got != 0 {
		t.Errorf("mu-law 0xFF = %d, want 0", got)
	}
	if got := linearToAlaw(0); got != 0xD5 {
		t.Errorf("A-law of silence = %#x, want 0xD5", got)
	}
	if got := alawToLinear(0xD5); got < 0 || got > 16 {
		t.Errorf("A-law 0xD5 = %d, want a value next to zero", got)
	}
	// the loudest values must not wrap around into the opposite sign
	if mulawToLinear(linearToMulaw(32767)) < 30000 || mulawToLinear(linearToMulaw(-32768)) > -30000 {
		t.Error("mu-law wrapped around at full scale")
	}
	if alawToLinear(linearToAlaw(32767)) < 30000 || alawToLinear(linearToAlaw(-32768)) > -30000 {
		t.Error("A-law wrapped around at full scale")
	}
}

// G.711 is a logarithmic code: the error grows with the level, to about 1/16 of it.
func TestG711RoundTripStaysWithinTheCodecsPrecision(t *testing.T) {
	for _, c := range []struct {
		name string
		enc  func(int16) byte
		dec  func(byte) int16
	}{
		{"mu-law", linearToMulaw, mulawToLinear},
		{"A-law", linearToAlaw, alawToLinear},
	} {
		for v := -32768; v <= 32767; v++ {
			got := int(c.dec(c.enc(int16(v))))
			clipped := min(max(v, -32635), 32635)
			if limit := math.Abs(float64(clipped))/16 + 16; math.Abs(float64(got-clipped)) > limit {
				t.Fatalf("%s: %d came back as %d (allowed error %.0f)", c.name, v, got, limit)
			}
		}
	}
}

func framesOf(x []float32) [][]float32 {
	var out [][]float32
	for len(x) >= frameSamples {
		out = append(out, x[:frameSamples])
		x = x[frameSamples:]
	}
	return out
}

// What leaves the call as 16 kHz frames and comes back from the client in the same
// format must be the same audio, whatever the format.
func TestConverterRoundTripKeepsTheAudio(t *testing.T) {
	const peak = 0.5
	want := peak / math.Sqrt2
	for _, f := range []AudioFormat{
		{EncodingPCM, 8000}, {EncodingPCM, 24000}, {EncodingPCM, 16000},
		{EncodingMulaw, 8000}, {EncodingAlaw, 8000},
	} {
		c := newConverter(f)
		var back []float32
		for _, frame := range framesOf(tone(1000, peak, 16000, 1.5)) {
			back = append(back, c.decode(c.encode(frame))...)
		}
		if level := rms(back, 1000); math.Abs(level-want) > 0.06*want {
			t.Errorf("%+v: a tone came back with rms %.4f, want %.4f", f, level, want)
		}
	}
}

func TestEncodedFramesHaveTheSizeOfTheFormat(t *testing.T) {
	for _, c := range []struct {
		f     AudioFormat
		bytes int
	}{
		{AudioFormat{EncodingPCM, 8000}, 480 * 2},
		{AudioFormat{EncodingPCM, 24000}, 1440 * 2},
		{AudioFormat{EncodingMulaw, 8000}, 480},
		{AudioFormat{EncodingAlaw, 8000}, 480},
	} {
		conv := newConverter(c.f)
		var last []byte
		for _, frame := range framesOf(tone(440, 0.3, 16000, 1)) {
			last = conv.encode(frame)
		}
		if len(last) != c.bytes {
			t.Errorf("%+v: a 60 ms frame is %d bytes, want %d", c.f, len(last), c.bytes)
		}
	}
}

// An odd number of PCM bytes in one chunk is half a sample; the other half arrives with
// the next chunk.
func TestConverterJoinsASampleSplitBetweenChunks(t *testing.T) {
	enc := newConverter(AudioFormat{EncodingPCM, 24000})
	data := enc.encode(tone(500, 0.4, 16000, 0.5))

	whole := newConverter(AudioFormat{EncodingPCM, 24000}).decode(data)
	c := newConverter(AudioFormat{EncodingPCM, 24000})
	split := append(c.decode(data[:101]), c.decode(data[101:])...)

	if len(whole) != len(split) {
		t.Fatalf("%d samples at once, %d split", len(whole), len(split))
	}
	for i := range whole {
		if whole[i] != split[i] {
			t.Fatalf("sample %d differs: %v vs %v", i, whole[i], split[i])
		}
	}
}
