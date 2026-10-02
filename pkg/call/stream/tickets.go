// Package call_stream carries the audio of a call over a WebSocket.
//
// A browser cannot set an apikey header on a WebSocket, and putting the apikey in the
// URL would leave it in access logs and proxies. So the client asks for a ticket with
// its normal authentication and opens the socket with that: a random token that works
// once, for one call of one instance, for a few seconds.
package call_stream

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"sync"
	"time"
)

const (
	// TicketTTL is how long a ticket can be redeemed after it was issued.
	TicketTTL = 30 * time.Second
	// maxTickets bounds the tickets waiting to be redeemed, so that asking for them
	// cannot be used to grow memory.
	maxTickets = 1000
)

// ErrTooManyTickets: too many tickets are waiting to be redeemed.
var ErrTooManyTickets = errors.New("too many pending stream tickets")

type ticket struct {
	instanceID string
	callID     string
	video      bool
	format     AudioFormat
	expires    time.Time
}

// Tickets issues and redeems the one-time tokens that open the audio stream.
type Tickets struct {
	mu  sync.Mutex
	m   map[string]ticket // sha256(token) -> ticket: a memory dump does not leak usable tokens
	now func() time.Time
}

func NewTickets() *Tickets {
	return &Tickets{m: make(map[string]ticket), now: time.Now}
}

func tokenKey(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// Issue creates a ticket for a call of an instance. video says whether the stream it
// opens carries the call's video besides its audio. Its audio is the default format.
func (t *Tickets) Issue(instanceID, callID string, video bool) (token string, ttl time.Duration, err error) {
	return t.IssueFormat(instanceID, callID, video, DefaultAudioFormat)
}

// IssueFormat is Issue with the audio format the stream will carry (see ParseAudioFormat).
func (t *Tickets) IssueFormat(instanceID, callID string, video bool, format AudioFormat) (token string, ttl time.Duration, err error) {
	raw := make([]byte, 24)
	if _, err := rand.Read(raw); err != nil {
		return "", 0, err
	}
	token = base64.RawURLEncoding.EncodeToString(raw)

	t.mu.Lock()
	defer t.mu.Unlock()

	now := t.now()
	if len(t.m) >= maxTickets {
		for k, v := range t.m {
			if !now.Before(v.expires) {
				delete(t.m, k)
			}
		}
		if len(t.m) >= maxTickets {
			return "", 0, ErrTooManyTickets
		}
	}
	t.m[tokenKey(token)] = ticket{instanceID: instanceID, callID: callID, video: video, format: format.normalized(), expires: now.Add(TicketTTL)}
	return token, TicketTTL, nil
}

// Redeem consumes a ticket. It returns the instance the ticket was issued to and
// whether it asks for video, and only when the token is known, has not expired, has not
// been used and was issued for callID. A ticket is spent by any attempt to redeem it,
// right or wrong.
func (t *Tickets) Redeem(token, callID string) (instanceID string, video bool, ok bool) {
	instanceID, video, _, ok = t.RedeemFormat(token, callID)
	return instanceID, video, ok
}

// RedeemFormat is Redeem that also returns the audio format the ticket asked for.
func (t *Tickets) RedeemFormat(token, callID string) (instanceID string, video bool, format AudioFormat, ok bool) {
	if token == "" {
		return "", false, AudioFormat{}, false
	}
	key := tokenKey(token)

	t.mu.Lock()
	tk, found := t.m[key]
	if found {
		delete(t.m, key)
	}
	t.mu.Unlock()

	if !found || !t.now().Before(tk.expires) {
		return "", false, AudioFormat{}, false
	}
	if subtle.ConstantTimeCompare([]byte(tk.callID), []byte(callID)) != 1 {
		return "", false, AudioFormat{}, false
	}
	return tk.instanceID, tk.video, tk.format, true
}
