package whatsmeow_service

import (
	"sync"
	"time"

	"go.mau.fi/whatsmeow"
)

// ProxyRuntimeStatus is what the running client actually did with the instance's
// proxy. A stored proxy configuration does not prove the client uses it: on
// authentication errors the connection used to silently fall back to a direct
// connection, exposing the server's real IP. Never holds credentials.
type ProxyRuntimeStatus struct {
	// RuntimeEnabled: the current client is configured to go through the proxy.
	RuntimeEnabled bool `json:"runtimeEnabled"`
	// FallbackWithoutProxy: the proxy failed and the client connected directly.
	FallbackWithoutProxy bool `json:"fallbackWithoutProxy"`
	// LastError is the last proxy problem, without credentials.
	LastError     string     `json:"lastError,omitempty"`
	LastAppliedAt *time.Time `json:"lastAppliedAt,omitempty"`
}

var proxyRuntime sync.Map // instanceID -> ProxyRuntimeStatus

func setProxyRuntime(instanceID string, update func(*ProxyRuntimeStatus)) {
	var st ProxyRuntimeStatus
	if v, ok := proxyRuntime.Load(instanceID); ok {
		st = v.(ProxyRuntimeStatus)
	}
	update(&st)
	proxyRuntime.Store(instanceID, st)
}

// proxyEnabled records that the client was successfully configured to use the proxy.
func proxyEnabled(instanceID string) {
	now := time.Now()
	setProxyRuntime(instanceID, func(st *ProxyRuntimeStatus) {
		*st = ProxyRuntimeStatus{RuntimeEnabled: true, LastAppliedAt: &now}
	})
}

// proxyFailed records a proxy problem. fellBack tells whether the client went on
// to connect directly.
func proxyFailed(instanceID, reason string, fellBack bool) {
	setProxyRuntime(instanceID, func(st *ProxyRuntimeStatus) {
		st.RuntimeEnabled = false
		st.FallbackWithoutProxy = fellBack
		st.LastError = reason
	})
}

// GetProxyRuntimeStatus returns the recorded status; ok is false when the
// current client is not using a proxy at all.
func GetProxyRuntimeStatus(instanceID string) (ProxyRuntimeStatus, bool) {
	v, ok := proxyRuntime.Load(instanceID)
	if !ok {
		return ProxyRuntimeStatus{}, false
	}
	return v.(ProxyRuntimeStatus), true
}

// fallbackWithoutProxy is called when the connection failed with a proxy
// authentication error. With PROXY_FAIL_CLOSED it refuses to continue (returns
// false) so the real IP is never exposed; otherwise it drops the proxy, records
// the fallback and returns true so the caller can connect directly.
func (w whatsmeowService) fallbackWithoutProxy(instanceID string, client *whatsmeow.Client, cause error) bool {
	if w.config.ProxyFailClosed {
		proxyFailed(instanceID, "proxy authentication failed (PROXY_FAIL_CLOSED: not falling back to a direct connection)", false)
		w.loggerWrapper.GetLogger(instanceID).LogError("[%s] Proxy authentication failed and PROXY_FAIL_CLOSED is set; not connecting directly: %v", instanceID, cause)
		w.clientPointer.Delete(instanceID)
		return false
	}
	proxyFailed(instanceID, "proxy authentication failed; connected without proxy", true)
	client.SetProxy(nil)
	return true
}
