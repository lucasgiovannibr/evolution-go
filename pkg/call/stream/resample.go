package call_stream

import "math"

// resampler converts a stream of mono samples from one rate to another while keeping
// state between calls, so a stream can be cut into chunks anywhere (the library hands
// over 60 ms frames, a client sends whatever size it likes) without a click at the seams.
//
// It is a windowed-sinc interpolator: every output sample is the sum of the input
// samples around its position, weighted by a sinc low-pass (cutoff at the lower of the
// two Nyquist frequencies, which is what keeps 16 kHz speech from aliasing when it is
// brought down to 8 kHz) shaped by a Blackman window. The rates used here are in a small
// integer ratio (16:8, 16:24), so the weights repeat with a short period and are computed
// once. The cost is a few dozen multiplications per output sample, nothing next to the
// audio codec.
//
// The output lags the input by about taps samples (a couple of milliseconds).
type resampler struct {
	l, m int         // output samples per l/m input samples: the position of output n is n*m/l
	taps int         // input samples used on each side of an output position
	w    [][]float32 // w[phase][k]: weights of input samples i0-taps+1 .. i0+taps

	buf  []float32 // input not yet fully used, starting at absolute index base
	base int64
	n    int64 // next output sample
}

// zeroCrossings is how many zero crossings of the low-pass the filter keeps on each
// side: more is a steeper transition band and more work.
const zeroCrossings = 16

func gcd(a, b int) int {
	for b != 0 {
		a, b = b, a%b
	}
	return a
}

// newResampler returns a converter from inRate to outRate, or nil when the rates are
// equal (nothing to do).
func newResampler(inRate, outRate int) *resampler {
	if inRate == outRate {
		return nil
	}
	g := gcd(inRate, outRate)
	r := &resampler{l: outRate / g, m: inRate / g}

	cutoff := math.Min(1, float64(outRate)/float64(inRate)) // relative to the input's Nyquist
	r.taps = int(math.Ceil(zeroCrossings / cutoff))

	r.w = make([][]float32, r.l)
	for p := 0; p < r.l; p++ {
		weights := make([]float64, 2*r.taps)
		var sum float64
		for k := -r.taps + 1; k <= r.taps; k++ {
			d := float64(k) - float64(p)/float64(r.l) // distance of input sample i0+k from the output position
			x := d / float64(r.taps)
			if math.Abs(x) >= 1 {
				continue
			}
			v := cutoff * sinc(cutoff*d) * blackman(x)
			weights[k+r.taps-1] = v
			sum += v
		}
		r.w[p] = make([]float32, len(weights))
		for i, v := range weights {
			r.w[p][i] = float32(v / sum) // unity gain at DC
		}
	}

	// the signal before the first sample is silence
	r.buf = make([]float32, r.taps)
	r.base = int64(-r.taps)
	return r
}

func sinc(x float64) float64 {
	if x == 0 {
		return 1
	}
	return math.Sin(math.Pi*x) / (math.Pi * x)
}

// blackman is the Blackman window at x in (-1, 1), 1 at the centre.
func blackman(x float64) float64 {
	return 0.42 + 0.5*math.Cos(math.Pi*x) + 0.08*math.Cos(2*math.Pi*x)
}

// Process takes the next samples of the input and returns the output samples that can
// be computed so far.
func (r *resampler) Process(in []float32) []float32 {
	r.buf = append(r.buf, in...)

	var out []float32
	for {
		pos := r.n * int64(r.m)
		i0 := pos / int64(r.l) // integer part of the position, in input samples
		last := r.base + int64(len(r.buf)) - 1
		if i0+int64(r.taps) > last {
			break
		}
		weights := r.w[pos%int64(r.l)]
		start := i0 - int64(r.taps) + 1 - r.base // index in buf of the first sample used
		var acc float32
		for k, w := range weights {
			acc += w * r.buf[start+int64(k)]
		}
		out = append(out, acc)
		r.n++
	}

	// forget the input no later output will use
	if keep := r.n*int64(r.m)/int64(r.l) - int64(r.taps) + 1; keep > r.base {
		drop := int(keep - r.base)
		r.buf = append(r.buf[:0], r.buf[drop:]...)
		r.base = keep
	}
	return out
}
