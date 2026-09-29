package whatsmeow_service

import (
	"encoding/json"
	"time"

	"go.mau.fi/whatsmeow/types/events"
)

// Three whatsmeow events used to fall into the generic "Unhandled event" log line
// although each one explains a failure users report:
//
//   - NotifyAccountReachoutTimelock: WhatsApp restricted the account from starting
//     conversations with people who never wrote to it. Sends to such contacts fail
//     with error 463 (issues #50, #124, #115).
//   - StreamError: a <stream:error> with a code whatsmeow does not know (the known
//     ones become other events). It is what precedes the dead client of issue #185.
//   - ClientOutdated: the server rejected the client version (connect failure 405).

// ReachoutTimelockStatus is the account restriction reported by WhatsApp.
type ReachoutTimelockStatus struct {
	Active          bool       `json:"active"`
	EnforcementType string     `json:"enforcementType,omitempty"`
	EndsAt          *time.Time `json:"endsAt,omitempty"`
	ReceivedAt      time.Time  `json:"receivedAt"`
}

// InEffect reports whether the restriction is still to be considered active at now.
func (s *ReachoutTimelockStatus) InEffect(now time.Time) bool {
	if s == nil || !s.Active {
		return false
	}
	return s.EndsAt == nil || s.EndsAt.After(now)
}

// reachoutTimelockFromEvent converts the event. A zero end time means "unknown".
func reachoutTimelockFromEvent(evt *events.NotifyAccountReachoutTimelock, now time.Time) *ReachoutTimelockStatus {
	st := &ReachoutTimelockStatus{
		Active:          evt.IsActive,
		EnforcementType: evt.EnforcementType,
		ReceivedAt:      now,
	}
	if !evt.TimeEnforcementEnds.IsZero() {
		end := evt.TimeEnforcementEnds.Time
		st.EndsAt = &end
	}
	return st
}

// webhookData is the payload published for the event.
func (s *ReachoutTimelockStatus) webhookData() map[string]interface{} {
	data := map[string]interface{}{
		"active":          s.Active,
		"enforcementType": s.EnforcementType,
	}
	if s.EndsAt != nil {
		data["endsAt"] = s.EndsAt.Format(time.RFC3339)
	}
	return data
}

// StreamErrorInfo is the last unknown <stream:error> seen by a client.
type StreamErrorInfo struct {
	Code string    `json:"code"`
	Raw  string    `json:"raw,omitempty"`
	At   time.Time `json:"at"`
}

func streamErrorFromEvent(evt *events.StreamError, now time.Time) *StreamErrorInfo {
	info := &StreamErrorInfo{Code: evt.Code, At: now}
	if evt.Raw != nil {
		// The node is only meant for humans reading a log/webhook: its JSON form is enough.
		if raw, err := json.Marshal(evt.Raw); err == nil {
			info.Raw = string(raw)
		}
	}
	return info
}

func (i *StreamErrorInfo) webhookData() map[string]interface{} {
	data := map[string]interface{}{"code": i.Code}
	if i.Raw != "" {
		data["raw"] = i.Raw
	}
	return data
}

// invalidateWebVersionCache forgets the cached WhatsApp Web version, so the next
// (re)connection looks it up again. The cache lives for an hour; after a 405 that
// hour is exactly the time during which every retry would be refused again with the
// same stale version.
func invalidateWebVersionCache() {
	cachedWebVersionMu.Lock()
	cachedWebVersion = nil
	cachedWebVersionMu.Unlock()
}

// ReachoutTimelock returns the restriction currently in effect for the instance, or
// nil when there is none (or it already ended).
func (w *whatsmeowService) ReachoutTimelock(instanceId string) *ReachoutTimelockStatus {
	mycli := w.myClientPointer.Get(instanceId)
	if mycli == nil {
		return nil
	}
	st := mycli.reachoutTimelock.Load()
	if !st.InEffect(time.Now()) {
		return nil
	}
	return st
}
