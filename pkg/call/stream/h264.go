package call_stream

import "bytes"

// H.264 NAL unit types this package cares about.
const (
	nalIDR = 5 // an IDR picture: a keyframe, decoding can start here
	nalAUD = 9 // access unit delimiter: must stay first
	nalSPS = 7
	nalPPS = 8
)

var (
	startCode3 = []byte{0, 0, 1}
	startCode4 = []byte{0, 0, 0, 1}
)

// nalUnits splits an Annex-B access unit (NAL units prefixed by 00 00 01 or 00 00 00 01)
// into its NAL units, start codes removed. It returns nil when the data does not begin
// with a start code, which is how raw or AVCC-framed H.264 (length prefixed, what MP4
// files carry) shows up: that must not be sent to WhatsApp.
func nalUnits(au []byte) [][]byte {
	var pos int
	switch {
	case bytes.HasPrefix(au, startCode4):
		pos = 4
	case bytes.HasPrefix(au, startCode3):
		pos = 3
	default:
		return nil
	}

	var units [][]byte
	for pos <= len(au) {
		next := bytes.Index(au[pos:], startCode3)
		if next < 0 {
			if unit := au[pos:]; len(unit) > 0 {
				units = append(units, unit)
			}
			break
		}
		// the zero before a 4-byte start code belongs to the start code
		if unit := bytes.TrimRight(au[pos:pos+next], "\x00"); len(unit) > 0 {
			units = append(units, unit)
		}
		pos += next + 3
	}
	return units
}

func nalType(unit []byte) byte { return unit[0] & 0x1f }

// isKeyframe reports whether the access unit contains an IDR picture.
func isKeyframe(au []byte) bool {
	for _, u := range nalUnits(au) {
		if nalType(u) == nalIDR {
			return true
		}
	}
	return false
}

// headerRepair remembers the last SPS and PPS a client sent and puts them back into a
// keyframe that arrives without them.
//
// A decoder cannot start on an IDR picture alone: it needs the SPS and PPS that say how
// to read it. Encoders can be told to repeat them with every keyframe, but many send
// them once at the start of the stream, so a keyframe sent later on request (WhatsApp
// asks for one when it loses video) reaches the peer undecodable, which looks like
// video that never renders.
type headerRepair struct {
	sps, pps []byte
}

// repair returns au ready to send. A keyframe that lacks the SPS or the PPS gets them
// (when they are known), placed as decoders expect: after the access unit delimiter if
// there is one, SPS then PPS, then the rest. ok is false when au is not Annex-B H.264.
func (h *headerRepair) repair(au []byte) (out []byte, ok bool) {
	units := nalUnits(au)
	if len(units) == 0 {
		return nil, false
	}

	var hasSPS, hasPPS, hasIDR bool
	for _, u := range units {
		switch nalType(u) {
		case nalSPS:
			hasSPS = true
			h.sps = append(h.sps[:0], u...)
		case nalPPS:
			hasPPS = true
			h.pps = append(h.pps[:0], u...)
		case nalIDR:
			hasIDR = true
		}
	}

	if !hasIDR || (hasSPS && hasPPS) || len(h.sps) == 0 || len(h.pps) == 0 {
		return au, true
	}

	out = make([]byte, 0, len(au)+len(h.sps)+len(h.pps)+8)
	put := func(unit []byte) { out = append(append(out, startCode4...), unit...) }
	if nalType(units[0]) == nalAUD {
		put(units[0])
		units = units[1:]
	}
	put(h.sps)
	put(h.pps)
	for _, u := range units {
		if t := nalType(u); t != nalSPS && t != nalPPS {
			put(u)
		}
	}
	return out, true
}
