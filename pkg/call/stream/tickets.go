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

// StreamOptions are what a ticket asks of the stream it opens. The zero value is the
// plain stream: audio only, 16 kHz PCM, everything in JSON.
type StreamOptions struct {
	// Video makes the stream carry the call's video besides its audio.
	Video bool
	// Format is the audio the stream carries (see ParseAudioFormat).
	Format AudioFormat
	// Binary sends and accepts the audio and video as binary WebSocket frames instead of
	// base64 in JSON (see the protocol description in handler.go). Control messages stay
	// JSON.
	Binary bool
	// Speech adds the speech_start and speech_end events: the stream says when the peer
	// begins and stops talking.
	Speech bool
}

type ticket struct {
	instanceID string
	callID     string
	opts       StreamOptions
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
// opens carries the call's video besides its audio. Everything else is the default.
func (t *Tickets) Issue(instanceID, callID string, video bool) (token string, ttl time.Duration, err error) {
	return t.IssueWith(instanceID, callID, StreamOptions{Video: video})
}

// IssueWith is Issue with every option of the stream.
func (t *Tickets) IssueWith(instanceID, callID string, opts StreamOptions) (token string, ttl time.Duration, err error) {
	raw := make([]byte, 24)
	if _, err := rand.Read(raw); err != nil {
		return "", 0, err
	}
	token = base64.RawURLEncoding.EncodeToString(raw)
	opts.Format = opts.Format.normalized()

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
	t.m[tokenKey(token)] = ticket{instanceID: instanceID, callID: callID, opts: opts, expires: now.Add(TicketTTL)}
	return token, TicketTTL, nil
}

// Redeem consumes a ticket. It returns the instance the ticket was issued to and
// whether it asks for video, and only when the token is known, has not expired, has not
// been used and was issued for callID. A ticket is spent by any attempt to redeem it,
// right or wrong.
func (t *Tickets) Redeem(token, callID string) (instanceID string, video bool, ok bool) {
	instanceID, opts, ok := t.RedeemWith(token, callID)
	return instanceID, opts.Video, ok
}

// RedeemWith is Redeem that also returns the other options the ticket asked for.
func (t *Tickets) RedeemWith(token, callID string) (instanceID string, opts StreamOptions, ok bool) {
	if token == "" {
		return "", StreamOptions{}, false
	}
	key := tokenKey(token)

	t.mu.Lock()
	tk, found := t.m[key]
	if found {
		delete(t.m, key)
	}
	t.mu.Unlock()

	if !found || !t.now().Before(tk.expires) {
		return "", StreamOptions{}, false
	}
	if subtle.ConstantTimeCompare([]byte(tk.callID), []byte(callID)) != 1 {
		return "", StreamOptions{}, false
	}
	return tk.instanceID, tk.opts, true
}
