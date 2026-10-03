/*
 * WhatyGo Passkey Helper - MAIN world bridge
 * --------------------------------------------
 * Runs in the PAGE's JavaScript world (manifest "world": "MAIN") and does one
 * thing only: navigator.credentials.get().
 *
 * Why: password managers and platform authenticators (1Password, Bitwarden, the
 * Google Password Manager UI...) hook navigator.credentials in the page world.
 * A content script runs in an isolated world where those hooks do not exist, so
 * Chrome fell back to "Insert your security key and touch it" (issue #173).
 *
 * This script does NOT perform any network request (web.whatsapp.com's CSP would
 * block them): it only receives the challenge from content.js through
 * window.postMessage and answers with the serialized assertion.
 */
(function () {
  "use strict";

  var REQUEST = "evo-wapk:request";
  var RESPONSE = "evo-wapk:response";

  function b64uToBuf(value) {
    var b64 = String(value || "").replace(/-/g, "+").replace(/_/g, "/");
    while (b64.length % 4) b64 += "=";
    var bin = atob(b64);
    var bytes = new Uint8Array(bin.length);
    for (var i = 0; i < bin.length; i++) bytes[i] = bin.charCodeAt(i);
    return bytes.buffer;
  }

  function bufToB64u(buf) {
    var bytes = new Uint8Array(buf);
    var bin = "";
    for (var i = 0; i < bytes.length; i++) bin += String.fromCharCode(bytes[i]);
    return btoa(bin).replace(/\+/g, "-").replace(/\//g, "_").replace(/=+$/g, "");
  }

  function buildOptions(pk) {
    var options = {
      challenge: b64uToBuf(pk.challenge),
      timeout: pk.timeout || 60000,
      rpId: pk.rpId || "whatsapp.com",
      userVerification: pk.userVerification || "required",
    };
    // Do not force credential transports and omit allowCredentials when empty, so
    // the browser / password manager picks the authenticator (a "usb" hint made
    // Chrome ask for a physical security key).
    var allow = (pk.allowCredentials || []).map(function (c) {
      return { type: c.type || "public-key", id: b64uToBuf(c.id) };
    });
    if (allow.length) options.allowCredentials = allow;
    return options;
  }

  function serialize(cred) {
    var r = cred.response;
    var body = {
      id: cred.id,
      rawId: bufToB64u(cred.rawId),
      type: cred.type,
      response: {
        clientDataJSON: bufToB64u(r.clientDataJSON),
        authenticatorData: bufToB64u(r.authenticatorData),
        signature: bufToB64u(r.signature),
      },
    };
    if (r.userHandle && r.userHandle.byteLength) {
      body.response.userHandle = bufToB64u(r.userHandle);
    }
    return body;
  }

  window.addEventListener("message", async function (ev) {
    if (ev.source !== window || !ev.data || ev.data.type !== REQUEST) return;

    var id = ev.data.id;
    try {
      var cred = await navigator.credentials.get({ publicKey: buildOptions(ev.data.publicKey || {}) });
      if (!cred) throw new Error("Autenticacao cancelada.");
      window.postMessage({ type: RESPONSE, id: id, ok: true, credential: serialize(cred) }, window.location.origin);
    } catch (e) {
      var msg = (e && e.name ? e.name + ": " : "") + ((e && e.message) || String(e));
      window.postMessage({ type: RESPONSE, id: id, ok: false, error: msg }, window.location.origin);
    }
  });
})();
