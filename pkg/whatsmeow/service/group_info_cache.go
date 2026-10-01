package whatsmeow_service

import (
	"time"

	"github.com/patrickmn/go-cache"
	"go.mau.fi/whatsmeow/types"
	"golang.org/x/sync/singleflight"
)

// GetGroupInfo always asks WhatsApp (whatsmeow does not answer it from its own cache), and
// the message event handler called it for every message of a group: one extra round trip
// per message, inside WhatsApp's sequential event queue, and a source of IQ rate limits in
// a busy group.
//
// The answer is remembered for a short time and dropped when WhatsApp reports that the
// group changed (events.GroupInfo, events.JoinedGroup). The TTL is short because a
// group of 1000 members is ~100 KB and only groups with traffic are held.
const groupInfoTTL = 2 * time.Minute

type groupInfoCache struct {
	c       *cache.Cache
	flights singleflight.Group
}

func newGroupInfoCache(ttl time.Duration) *groupInfoCache {
	return &groupInfoCache{c: cache.New(ttl, ttl*2)}
}

func groupInfoKey(instanceID string, group types.JID) string {
	return instanceID + "|" + group.ToNonAD().String()
}

// get returns the remembered group or calls fetch. Errors are not remembered, and
// concurrent calls for one group share a single query.
func (g *groupInfoCache) get(key string, fetch func() (*types.GroupInfo, error)) (*types.GroupInfo, error) {
	if v, ok := g.c.Get(key); ok {
		return v.(*types.GroupInfo), nil
	}
	v, err, _ := g.flights.Do(key, func() (interface{}, error) {
		info, err := fetch()
		if err != nil {
			return nil, err
		}
		g.c.Set(key, info, cache.DefaultExpiration)
		return info, nil
	})
	if err != nil {
		return nil, err
	}
	return v.(*types.GroupInfo), nil
}

func (g *groupInfoCache) forget(key string) { g.c.Delete(key) }

// forgetInstance drops every group of an instance (it was deleted or logged out).
func (g *groupInfoCache) forgetInstance(instanceID string) {
	prefix := instanceID + "|"
	for key := range g.c.Items() {
		if len(key) >= len(prefix) && key[:len(prefix)] == prefix {
			g.c.Delete(key)
		}
	}
}

var groupInfos = newGroupInfoCache(groupInfoTTL)
