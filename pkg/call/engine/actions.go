package call_engine

import (
	"errors"
	"fmt"
)

var (
	// ErrCallNotFound: the instance has no such call (never had, or it already ended).
	ErrCallNotFound = errors.New("no active call with that id")
	// ErrWrongState: the call exists but is not in a state that allows the action.
	ErrWrongState = errors.New("the call is not in a state that allows this")
)

// Answer answers a tracked incoming call that is still ringing.
func (m *Manager) Answer(instanceID, callID string) (*Tracked, error) {
	t, ok := m.Get(instanceID, callID)
	if !ok {
		return nil, ErrCallNotFound
	}

	// Two requests must not both see "ringing" and both answer.
	t.act.Lock()
	defer t.act.Unlock()

	if t.direction != Incoming {
		return t, fmt.Errorf("%w: only incoming calls can be answered", ErrWrongState)
	}
	if phase := t.call.Phase(); phase != PhaseRinging {
		return t, fmt.Errorf("%w: the call is %s, not ringing", ErrWrongState, phase)
	}
	if err := t.call.Answer(); err != nil {
		return t, fmt.Errorf("answer: %w", err)
	}
	return t, nil
}

// Hangup ends a tracked call whatever its phase: an incoming call that still rings is
// rejected, any other is hung up. The call is over here even when telling the peer
// fails, which is why it is forgotten either way and err only says the peer may not
// have been told.
func (m *Manager) Hangup(instanceID, callID string) (*Tracked, error) {
	t, ok := m.Get(instanceID, callID)
	if !ok {
		return nil, ErrCallNotFound
	}

	t.act.Lock()
	defer t.act.Unlock()

	var err error
	reason := "hangup"
	if t.direction == Incoming && t.call.Phase() == PhaseRinging {
		err = t.call.Reject()
		reason = "rejected"
	} else {
		err = t.call.Hangup()
	}
	m.finish(instanceID, t, reason)
	return t, err
}
