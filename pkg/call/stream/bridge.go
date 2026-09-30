package call_stream

import (
	"encoding/binary"
	"sync"
	"time"

	call_engine "github.com/evolution-foundation/evolution-go/pkg/call/engine"
)

const (
	frameSamples = call_engine.FrameSamples

	// toClientFrames is how much of the peer's audio waits for a slow client: three
	// seconds. Past that the oldest audio is dropped, because a listener that is behind
	// wants the present, not a growing delay.
	toClientFrames = 50

	// maxOutboundSamples is how much audio a client may have queued to be sent: thirty
	// seconds. A text-to-speech engine produces a sentence much faster than real time
	// and expects it to be played out at the right pace, so this is deliberately large.
	maxOutboundSamples = 30 * call_engine.SampleRate

	// tailFlushAfter is how long the last, incomplete frame of a burst of client audio
	// waits for more before it is padded with silence and sent.
	tailFlushAfter = 120 * time.Millisecond
)

// bridge is the audio side of one stream: the AudioSink that receives the peer's audio
// and the AudioSource that supplies the client's, both without ever blocking the
// library's goroutines. The library sends a frame every 60 ms and wants silence, not a
// stall, when there is nothing to send: a call that stops transmitting loses its relay.
type bridge struct {
	stats *call_engine.StreamStats

	toClient chan []float32 // peer audio, read by the socket writer

	mu       sync.Mutex
	pending  []float32 // client audio not yet sent to the peer
	odd      []byte    // a lone byte left over when a chunk had an odd length
	lastPush time.Time
	closed   bool
	done     chan struct{}
	now      func() time.Time
}

func newBridge(stats *call_engine.StreamStats) *bridge {
	return &bridge{
		stats:    stats,
		toClient: make(chan []float32, toClientFrames),
		done:     make(chan struct{}),
		now:      time.Now,
	}
}

// WriteFrame receives the peer's audio from the library.
func (b *bridge) WriteFrame(frame []float32) error {
	if len(frame) == 0 {
		return nil
	}
	// The library may reuse its buffer once this returns.
	frame = append([]float32(nil), frame...)

	select {
	case b.toClient <- frame:
	default:
		// full: make room by dropping the oldest frame
		select {
		case <-b.toClient:
			b.stats.DroppedToClient.Add(1)
		default:
		}
		select {
		case b.toClient <- frame:
		default:
			b.stats.DroppedToClient.Add(1)
		}
	}
	return nil
}

// ReadFrame is called by the library's send loop every 60 ms. It returns one frame of
// the client's audio, or nil (silence) when there is none.
func (b *bridge) ReadFrame() ([]float32, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if len(b.pending) >= frameSamples {
		return b.takeLocked(frameSamples), nil
	}
	// The tail of an utterance is shorter than a frame; without this it would wait for
	// audio that may only come much later.
	if n := len(b.pending); n > 0 && b.now().Sub(b.lastPush) >= tailFlushAfter {
		frame := make([]float32, frameSamples)
		copy(frame, b.pending)
		b.pending = b.pending[:0]
		b.stats.FromClient.Add(1)
		return frame, nil
	}
	return nil, nil
}

func (b *bridge) takeLocked(n int) []float32 {
	frame := make([]float32, n)
	copy(frame, b.pending)
	b.pending = append(b.pending[:0], b.pending[n:]...)
	b.stats.FromClient.Add(1)
	return frame
}

// Push queues a chunk of client audio: signed 16-bit little-endian mono at 16 kHz, of
// any length. Audio that does not fit within maxOutboundSamples is dropped.
func (b *bridge) Push(pcm []byte) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return
	}

	if len(b.odd) > 0 {
		pcm = append(b.odd, pcm...)
		b.odd = nil
	}
	if len(pcm)%2 == 1 {
		b.odd = []byte{pcm[len(pcm)-1]}
		pcm = pcm[:len(pcm)-1]
	}
	samples := len(pcm) / 2
	if samples == 0 {
		return
	}
	if len(b.pending)+samples > maxOutboundSamples {
		b.stats.DroppedFromClient.Add(uint64((samples + frameSamples - 1) / frameSamples))
		return
	}

	for i := 0; i < samples; i++ {
		b.pending = append(b.pending, float32(int16(binary.LittleEndian.Uint16(pcm[2*i:])))/32768.0)
	}
	b.lastPush = b.now()
}

// Clear drops the client audio that has not been sent yet (an assistant that is
// interrupted mid-sentence must not finish it).
func (b *bridge) Clear() {
	b.mu.Lock()
	b.pending = b.pending[:0]
	b.odd = nil
	b.mu.Unlock()
}

// Close is called by the library when the call ends, and by the socket when it goes
// away. Both may happen, more than once.
func (b *bridge) Close() error {
	b.mu.Lock()
	if !b.closed {
		b.closed = true
		close(b.done)
	}
	b.pending = nil
	b.mu.Unlock()
	return nil
}

// pcm16 converts frames to signed 16-bit little-endian mono, clamping like a recorder
// would.
func pcm16(frame []float32) []byte {
	out := make([]byte, len(frame)*2)
	for i, s := range frame {
		v := s * 32768.0
		switch {
		case v > 32767:
			v = 32767
		case v < -32768:
			v = -32768
		}
		binary.LittleEndian.PutUint16(out[2*i:], uint16(int16(v)))
	}
	return out
}
