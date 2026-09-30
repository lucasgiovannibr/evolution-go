package call_engine

import (
	"errors"
	"fmt"
)

// ErrInvalidVideoRequest: the video action or its orientation is not one that exists.
var ErrInvalidVideoRequest = errors.New("invalid video request")

// VideoAction is something a client can ask the call to do about video.
type VideoAction string

const (
	// VideoStart asks the peer to turn an audio call into a video call and starts
	// sending our video once the peer accepts.
	VideoStart VideoAction = "start"
	// VideoAccept accepts the peer's request to turn the call into a video call. It
	// does not start our own camera.
	VideoAccept VideoAction = "accept"
	// VideoStop stops sending video; audio and the peer's video go on.
	VideoStop VideoAction = "stop"
	// VideoEnable and VideoDisable mute and unmute our video without ending it.
	VideoEnable  VideoAction = "enable"
	VideoDisable VideoAction = "disable"
	// VideoOrientation tells the peer how our camera is rotated (0..3 quarter turns
	// clockwise).
	VideoOrientation VideoAction = "orientation"
)

// Video does one video action on a tracked call. Only a call that is being connected
// or is running has video to act on. The library's own refusal (no pending upgrade to
// accept, call not active) comes back as ErrWrongState with its reason.
func (m *Manager) Video(instanceID, callID string, action VideoAction, orientation int) (*Tracked, error) {
	switch action {
	case VideoStart, VideoAccept, VideoStop, VideoEnable, VideoDisable:
	case VideoOrientation:
		if orientation < 0 || orientation > 3 {
			return nil, fmt.Errorf("%w: orientation must be 0, 1, 2 or 3 quarter turns", ErrInvalidVideoRequest)
		}
	default:
		return nil, fmt.Errorf("%w: action must be start, accept, stop, enable, disable or orientation", ErrInvalidVideoRequest)
	}

	t, ok := m.Get(instanceID, callID)
	if !ok {
		return nil, ErrCallNotFound
	}

	t.act.Lock()
	defer t.act.Unlock()

	if phase := t.call.Phase(); phase != PhaseConnecting && phase != PhaseActive {
		return t, fmt.Errorf("%w: the call is %s; video can only be changed once it is answered", ErrWrongState, phase)
	}

	var err error
	switch action {
	case VideoStart:
		err = t.call.StartVideo()
	case VideoAccept:
		err = t.call.AcceptVideo()
	case VideoStop:
		err = t.call.StopVideo()
	case VideoEnable:
		err = t.call.SetVideoEnabled(true)
	case VideoDisable:
		err = t.call.SetVideoEnabled(false)
	case VideoOrientation:
		err = t.call.SetVideoOrientation(orientation)
	}
	if err != nil {
		return t, fmt.Errorf("%w: video %s: %v", ErrWrongState, action, err)
	}
	return t, nil
}
