package utils

import (
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"os"
	"sync/atomic"
	"syscall"
	"time"
)

// Outbound HTTP used to go through http.Get / &http.Client{}, which have no timeout at
// all: a media URL, a link-preview site or web.whatsapp.com that accepts the connection
// and then stalls held the calling request forever (and, for the WhatsApp Web version
// lookup, a mutex that every instance start goes through).
//
// The URLs these clients fetch come from the API caller (media, stickers, link previews,
// status, profile pictures) or from the instance's own configuration (webhooks), so they
// are also the way to make this server talk to something it should not: another host of
// the internal network, the machine's own services, or the cloud metadata endpoint
// (169.254.169.254) that hands out credentials. Reproduced on the test stack: a
// /send/link whose URL named a container reachable only on the internal network made the
// server request it, redirects included.
//
// The check is made where it cannot be bypassed: when the connection is made, on the
// address that was actually resolved (net.Dialer.Control), so a name that resolves to an
// internal address (or changes its answer between the check and the connection, "DNS
// rebinding"), a literal IP and every redirect are all covered.

// NetPolicy says which destinations an outbound client may connect to.
type NetPolicy int

const (
	// PolicyPublic: only public addresses. For URLs supplied by API callers (media,
	// stickers, link previews, status).
	PolicyPublic NetPolicy = iota
	// PolicyNoMetadata: anything but link-local and the cloud metadata addresses. For
	// webhooks, whose receiver is very often on the internal network (n8n, the CRM, the
	// application next door), but is never the metadata service.
	PolicyNoMetadata
	// PolicyAny: no restriction. Only for endpoints the operator configured (the audio
	// converter API), never for a URL that came from a request.
	PolicyAny
)

// allowPrivate relaxes PolicyPublic to PolicyNoMetadata (ALLOW_PRIVATE_URLS=true), for
// deployments that really do fetch media from their own network.
var allowPrivate atomic.Bool

func init() { allowPrivate.Store(os.Getenv("ALLOW_PRIVATE_URLS") == "true") }

// SetAllowPrivateURLs sets what ALLOW_PRIVATE_URLS sets, at runtime (for tests, which
// serve files from 127.0.0.1).
func SetAllowPrivateURLs(allow bool) { allowPrivate.Store(allow) }

// BlockedAddressError is the refusal of a destination by the outbound policy.
type BlockedAddressError struct {
	Address string
	Reason  string
}

func (e *BlockedAddressError) Error() string {
	return fmt.Sprintf("connection to %s is not allowed (%s)", e.Address, e.Reason)
}

var (
	// not covered by the Is* helpers
	cgnat       = netip.MustParsePrefix("100.64.0.0/10")
	thisNetwork = netip.MustParsePrefix("0.0.0.0/8")
	ietfProto   = netip.MustParsePrefix("192.0.0.0/24")
	benchmark   = netip.MustParsePrefix("198.18.0.0/15")
	reserved    = netip.MustParsePrefix("240.0.0.0/4")
	// cloud metadata endpoints that are not link-local
	alibabaMetadata = netip.MustParseAddr("100.100.100.200")
	awsIPv6Metadata = netip.MustParseAddr("fd00:ec2::254")
)

// blockedReason returns why addr may not be connected to under the policy, or "".
func blockedReason(addr netip.Addr, policy NetPolicy) string {
	addr = addr.Unmap()
	if policy == PolicyAny {
		return ""
	}
	if policy == PolicyPublic && allowPrivate.Load() {
		policy = PolicyNoMetadata
	}

	// Never reachable under any restricted policy.
	switch {
	case addr.IsUnspecified():
		return "unspecified address"
	case addr.IsLinkLocalUnicast() || addr.IsLinkLocalMulticast():
		return "link-local address (cloud metadata)"
	case addr == alibabaMetadata || addr == awsIPv6Metadata:
		return "cloud metadata address"
	case addr.IsMulticast():
		return "multicast address"
	}
	if policy == PolicyNoMetadata {
		return ""
	}

	switch {
	case addr.IsLoopback():
		return "loopback address"
	case addr.IsPrivate():
		return "private network address"
	case cgnat.Contains(addr):
		return "shared address space (100.64.0.0/10)"
	case thisNetwork.Contains(addr), ietfProto.Contains(addr), benchmark.Contains(addr), reserved.Contains(addr):
		return "reserved address"
	}
	return ""
}

// controlFor returns the net.Dialer.Control hook that enforces the policy on the address
// about to be connected to.
func controlFor(policy NetPolicy) func(network, address string, c syscall.RawConn) error {
	return func(network, address string, _ syscall.RawConn) error {
		host, _, err := net.SplitHostPort(address)
		if err != nil {
			return &BlockedAddressError{Address: address, Reason: "unparsable address"}
		}
		addr, err := netip.ParseAddr(host)
		if err != nil {
			return &BlockedAddressError{Address: address, Reason: "not an IP address"}
		}
		if reason := blockedReason(addr, policy); reason != "" {
			return &BlockedAddressError{Address: address, Reason: reason}
		}
		return nil
	}
}

func newClient(total time.Duration, policy NetPolicy) *http.Client {
	base, _ := http.DefaultTransport.(*http.Transport)
	var t *http.Transport
	if base != nil {
		t = base.Clone()
	} else {
		t = &http.Transport{}
	}
	t.DialContext = (&net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second, Control: controlFor(policy)}).DialContext
	t.TLSHandshakeTimeout = 10 * time.Second
	// Time to the first byte of the answer; the body may take as long as the total.
	t.ResponseHeaderTimeout = 30 * time.Second
	return &http.Client{
		Transport: t,
		Timeout:   total,
		// Every hop is connected through the same dialer, so a redirect to a blocked
		// address fails there; this only bounds the chain.
		CheckRedirect: func(_ *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return fmt.Errorf("stopped after %d redirects", len(via))
			}
			return nil
		},
	}
}

// NewWebhookClient is the client that delivers events to the webhook URL of an instance.
func NewWebhookClient(timeout time.Duration) *http.Client {
	return newClient(timeout, PolicyNoMetadata)
}

// NewPublicClient is a client for URLs supplied by API callers, with its own timeout.
func NewPublicClient(timeout time.Duration) *http.Client {
	return newClient(timeout, PolicyPublic)
}

// DownloadClient fetches user-supplied media (images, videos, documents by URL). The
// total limit is generous so that a large file on a slow link still completes. Public
// addresses only.
var DownloadClient = newClient(5*time.Minute, PolicyPublic)

// QuickClient is for small lookups (link previews, the WhatsApp Web version). Public
// addresses only.
var QuickClient = newClient(15*time.Second, PolicyPublic)

// TrustedClient is for endpoints the operator configured (the audio converter API), which
// may well be a container next door. Never use it for a URL that came from a request.
var TrustedClient = newClient(5*time.Minute, PolicyAny)
