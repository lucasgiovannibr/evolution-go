package send_service

import (
	"sync"
	"time"

	"golang.org/x/sync/singleflight"
)

// Every send to a person used to ask WhatsApp "is this number registered?" (one usync
// query, two when the first said no) before sending, with no memory of the answer. That
// is the biggest fixed cost of a send (100-400 ms) and a stream of contact lookups that
// WhatsApp watches. The answer rarely changes, so it is remembered.
const (
	// userExistsNegativeTTL is how long "not registered" is remembered: short, because the
	// person may register at any moment and a wrong "no" blocks every send to them.
	userExistsNegativeTTL = 5 * time.Minute

	// userExistsMaxEntries bounds the memory of the cache. When it is reached the cache is
	// emptied: crude, but it can never grow without limit and refills with what is in use.
	userExistsMaxEntries = 100_000
)

type userExistsEntry struct {
	remoteJID string
	found     bool
	expires   time.Time
}

// userExistsCache remembers the result of the "is this number on WhatsApp" check, per
// instance and number. Concurrent checks of the same number share one query.
type userExistsCache struct {
	positiveTTL time.Duration
	now         func() time.Time

	mu      sync.Mutex
	entries map[string]userExistsEntry
	flights singleflight.Group
}

// newUserExistsCache returns a cache that keeps registered numbers for positiveTTL; a
// non-positive TTL gives nil, which disables caching.
func newUserExistsCache(positiveTTL time.Duration) *userExistsCache {
	if positiveTTL <= 0 {
		return nil
	}
	return &userExistsCache{positiveTTL: positiveTTL, now: time.Now, entries: map[string]userExistsEntry{}}
}

// lookup returns the remembered answer for the number or calls fetch to get it. An error
// from fetch is returned and never remembered. A nil cache always calls fetch.
func (c *userExistsCache) lookup(instanceID, phone string, fetch func() (remoteJID string, found bool, err error)) (string, bool, error) {
	if c == nil {
		return fetch()
	}
	key := instanceID + "|" + phone

	c.mu.Lock()
	e, ok := c.entries[key]
	if ok && !c.now().Before(e.expires) {
		delete(c.entries, key)
		ok = false
	}
	c.mu.Unlock()
	if ok {
		return e.remoteJID, e.found, nil
	}

	type result struct {
		remoteJID string
		found     bool
	}
	v, err, _ := c.flights.Do(key, func() (interface{}, error) {
		remoteJID, found, err := fetch()
		if err != nil {
			return nil, err
		}
		c.store(key, remoteJID, found)
		return result{remoteJID, found}, nil
	})
	if err != nil {
		return "", false, err
	}
	r := v.(result)
	return r.remoteJID, r.found, nil
}

func (c *userExistsCache) store(key, remoteJID string, found bool) {
	ttl := userExistsNegativeTTL
	if found {
		ttl = c.positiveTTL
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.entries) >= userExistsMaxEntries {
		c.entries = map[string]userExistsEntry{}
	}
	c.entries[key] = userExistsEntry{remoteJID: remoteJID, found: found, expires: c.now().Add(ttl)}
}

// forget drops what is remembered about a number (a send to it failed in a way that
// suggests the answer changed).
func (c *userExistsCache) forget(instanceID, phone string) {
	if c == nil {
		return
	}
	c.mu.Lock()
	delete(c.entries, instanceID+"|"+phone)
	c.mu.Unlock()
}
