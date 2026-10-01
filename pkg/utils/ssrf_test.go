package utils

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
	"time"
)

func strict(t *testing.T) {
	t.Helper()
	SetAllowPrivateURLs(false)
	t.Cleanup(func() { SetAllowPrivateURLs(true) })
}

func TestBlockedReason(t *testing.T) {
	type tc struct {
		ip      string
		policy  NetPolicy
		blocked bool
	}
	cases := []tc{
		// public addresses are fine everywhere
		{"8.8.8.8", PolicyPublic, false},
		{"2606:4700:4700::1111", PolicyPublic, false},
		{"93.184.216.34", PolicyNoMetadata, false},

		// what a request must never make this server talk to
		{"127.0.0.1", PolicyPublic, true},
		{"::1", PolicyPublic, true},
		{"10.0.0.5", PolicyPublic, true},
		{"172.16.3.4", PolicyPublic, true},
		{"172.31.255.255", PolicyPublic, true},
		{"192.168.1.1", PolicyPublic, true},
		{"169.254.169.254", PolicyPublic, true}, // cloud metadata
		{"fe80::1", PolicyPublic, true},
		{"fd12:3456::1", PolicyPublic, true}, // ULA
		{"100.64.0.1", PolicyPublic, true},   // carrier-grade NAT
		{"0.0.0.0", PolicyPublic, true},
		{"::", PolicyPublic, true},
		{"224.0.0.1", PolicyPublic, true},
		{"240.0.0.1", PolicyPublic, true},
		{"198.18.0.1", PolicyPublic, true},
		{"::ffff:127.0.0.1", PolicyPublic, true}, // IPv4-mapped loopback
		{"::ffff:10.1.2.3", PolicyPublic, true},
		{"::ffff:169.254.169.254", PolicyPublic, true},

		// webhooks may go to the internal network, never to the metadata endpoints
		{"127.0.0.1", PolicyNoMetadata, false},
		{"10.0.0.5", PolicyNoMetadata, false},
		{"192.168.1.1", PolicyNoMetadata, false},
		{"169.254.169.254", PolicyNoMetadata, true},
		{"fe80::1", PolicyNoMetadata, true},
		{"100.100.100.200", PolicyNoMetadata, true}, // Alibaba metadata
		{"fd00:ec2::254", PolicyNoMetadata, true},   // AWS IPv6 metadata
		{"0.0.0.0", PolicyNoMetadata, true},

		// the operator's own endpoints
		{"10.0.0.5", PolicyAny, false},
		{"127.0.0.1", PolicyAny, false},
		{"169.254.169.254", PolicyAny, false},
	}
	for _, c := range cases {
		SetAllowPrivateURLs(false)
		got := blockedReason(netip.MustParseAddr(c.ip), c.policy) != ""
		if got != c.blocked {
			t.Errorf("%s under policy %d: blocked = %v, want %v", c.ip, c.policy, got, c.blocked)
		}
	}
	SetAllowPrivateURLs(true)
}

func TestAllowPrivateURLsRelaxesPublicButNotTheMetadata(t *testing.T) {
	defer SetAllowPrivateURLs(true)

	SetAllowPrivateURLs(true)
	for _, ip := range []string{"127.0.0.1", "10.0.0.5", "192.168.1.1", "100.64.0.1"} {
		if r := blockedReason(netip.MustParseAddr(ip), PolicyPublic); r != "" {
			t.Errorf("ALLOW_PRIVATE_URLS=true must allow %s, refused: %s", ip, r)
		}
	}
	for _, ip := range []string{"169.254.169.254", "fe80::1", "0.0.0.0", "100.100.100.200"} {
		if r := blockedReason(netip.MustParseAddr(ip), PolicyPublic); r == "" {
			t.Errorf("%s must stay blocked even with ALLOW_PRIVATE_URLS=true", ip)
		}
	}
}

// Reproduced on the test stack: a URL naming a host of the internal network was fetched.
func TestDownloadBytesRefusesAnInternalAddress(t *testing.T) {
	strict(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("the internal server must never be reached")
	}))
	defer srv.Close() // 127.0.0.1

	_, err := DownloadBytes(srv.URL, 1<<20)
	var blocked *BlockedAddressError
	if !errors.As(err, &blocked) {
		t.Fatalf("want a BlockedAddressError, got %v", err)
	}
	if !strings.Contains(err.Error(), "not allowed") {
		t.Fatalf("the refusal must say so: %v", err)
	}
}

func TestALiteralMetadataIPIsRefused(t *testing.T) {
	strict(t)
	for _, url := range []string{
		"http://169.254.169.254/latest/meta-data/iam/security-credentials/",
		"http://[fe80::1]/",
		"http://0.0.0.0:8080/",
		"http://[::ffff:169.254.169.254]/",
	} {
		client := NewPublicClient(2 * time.Second)
		resp, err := client.Get(url)
		if err == nil {
			resp.Body.Close()
			t.Errorf("%s was fetched", url)
			continue
		}
		var blocked *BlockedAddressError
		if !errors.As(err, &blocked) {
			t.Errorf("%s: expected a policy refusal, got %v", url, err)
		}
	}

	// The decimal and hex spellings of 169.254.169.254 are not parsed as addresses by Go (they
	// are looked up as host names, and fail): they cannot reach the endpoint either.
	for _, url := range []string{"http://2852039166/", "http://0xA9FEA9FE/"} {
		client := NewPublicClient(500 * time.Millisecond)
		if resp, err := client.Get(url); err == nil {
			resp.Body.Close()
			t.Errorf("%s was fetched", url)
		}
	}
}

// A redirect is a new connection through the same dialer: from an allowed server to the
// metadata endpoint, it is refused.
func TestRedirectToTheMetadataEndpointIsRefused(t *testing.T) {
	SetAllowPrivateURLs(true) // the redirecting server is on 127.0.0.1
	defer SetAllowPrivateURLs(true)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://169.254.169.254/latest/meta-data/", http.StatusFound)
	}))
	defer srv.Close()

	_, err := DownloadBytes(srv.URL, 1<<20)
	var blocked *BlockedAddressError
	if !errors.As(err, &blocked) {
		t.Fatalf("a redirect to the metadata endpoint must be refused, got %v", err)
	}
}

func TestRedirectChainIsBounded(t *testing.T) {
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, srv.URL+"/again", http.StatusFound)
	}))
	defer srv.Close()

	_, err := DownloadBytes(srv.URL, 1<<20)
	if err == nil || !strings.Contains(err.Error(), "redirects") {
		t.Fatalf("an endless redirect chain must stop, got %v", err)
	}
}

func TestWebhookClientReachesTheInternalNetworkButNotTheMetadata(t *testing.T) {
	strict(t) // PolicyPublic would refuse loopback; the webhook policy must not
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }))
	defer srv.Close()

	client := NewWebhookClient(2 * time.Second)
	resp, err := client.Post(srv.URL, "application/json", strings.NewReader("{}"))
	if err != nil {
		t.Fatalf("a webhook receiver on the internal network is the normal case: %v", err)
	}
	resp.Body.Close()

	if resp, err := client.Post("http://169.254.169.254/latest", "application/json", strings.NewReader("{}")); err == nil {
		resp.Body.Close()
		t.Fatal("a webhook must never reach the metadata endpoint")
	}
}

func TestTrustedClientIsUnrestricted(t *testing.T) {
	strict(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }))
	defer srv.Close()
	resp, err := TrustedClient.Get(srv.URL)
	if err != nil {
		t.Fatalf("the operator's own endpoints are not restricted: %v", err)
	}
	resp.Body.Close()
}
