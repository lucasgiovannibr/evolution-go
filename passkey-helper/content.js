/*
 * WhatyGo Passkey Helper
 * ------------------------
 * Executa a cerimonia WebAuthn (passkey) do WhatsApp Web no dominio correto
 * (web.whatsapp.com) para concluir um pareamento iniciado pelo WhatyGo.
 *
 * Fluxo:
 * 1. O manager/CRM abre https://web.whatsapp.com/#wapk=<payload>, onde payload
 *    e um base64url de JSON { t: <token_da_instancia>, b: <url_base_da_api> }.
 * 2. Este content script le o payload, faz polling do status da cerimonia na
 *    API, e quando o desafio (publicKey) esta disponivel pede a webauthn-main.js
 *    (executada no mundo MAIN da pagina) que rode navigator.credentials.get() —
 *    permitido pois roda no origin whatsapp.com e visivel a gerenciadores de senha.
 * 3. A assertion e enviada de volta para a API; quando o pareamento exige a
 *    confirmacao manual do codigo, um botao de confirmar e exibido.
 *
 * Nenhuma host_permission e necessaria: as chamadas partem do origin
 * web.whatsapp.com e o backend libera CORS para essa origem.
 *
 * A extensao e whitelabel: nada e hardcoded, tudo vem no payload da URL.
 */
(function () {
  "use strict";

  var STORAGE_KEY = "__evo_wapk_ceremony__";
  var POLL_INTERVAL_MS = 2000;

  // ---------------------------------------------------------------------------
  // base64url helpers
  // ---------------------------------------------------------------------------
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

  // ---------------------------------------------------------------------------
  // Ceremony payload (URL hash -> sessionStorage)
  // ---------------------------------------------------------------------------
  function readCeremonyFromHash() {
    try {
      var m = (location.hash || "").match(/[#&]wapk=([^&]+)/);
      if (!m) return null;
      var json = atob(m[1].replace(/-/g, "+").replace(/_/g, "/"));
      var obj = JSON.parse(json);
      if (obj && obj.t && obj.b) return { t: obj.t, b: obj.b };
    } catch (e) {}
    return null;
  }

  function getCeremony() {
    var fromHash = readCeremonyFromHash();
    if (fromHash) {
      try {
        sessionStorage.setItem(STORAGE_KEY, JSON.stringify(fromHash));
      } catch (e) {}
      try {
        history.replaceState(null, "", location.pathname + location.search);
      } catch (e) {}
      return fromHash;
    }
    try {
      var raw = sessionStorage.getItem(STORAGE_KEY);
      if (raw) return JSON.parse(raw);
    } catch (e) {}
    return null;
  }

  function clearCeremony() {
    try {
      sessionStorage.removeItem(STORAGE_KEY);
    } catch (e) {}
  }

  // ---------------------------------------------------------------------------
  // API calls (WhatyGo passkey ceremony endpoints)
  // ---------------------------------------------------------------------------
  function apiBase(cer) {
    return String(cer.b || "").replace(/\/+$/, "");
  }

  async function fetchStatus(cer) {
    var res = await fetch(apiBase(cer) + "/passkey-ceremony/" + encodeURIComponent(cer.t), {
      method: "GET",
      headers: { Accept: "application/json" },
    });
    var json = await res.json().catch(function () {
      return null;
    });
    if (!res.ok) {
      throw new Error((json && json.error) || "Falha ao consultar o status (HTTP " + res.status + ").");
    }
    return (json && json.data) || json || {};
  }

  async function sendResponse(cer, webauthnResponse) {
    var res = await fetch(apiBase(cer) + "/passkey-ceremony/" + encodeURIComponent(cer.t) + "/response", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(webauthnResponse),
    });
    var json = await res.json().catch(function () {
      return null;
    });
    if (!res.ok) {
      throw new Error((json && json.error) || "Falha ao enviar a chave de acesso.");
    }
  }

  async function sendConfirm(cer) {
    var res = await fetch(apiBase(cer) + "/passkey-ceremony/" + encodeURIComponent(cer.t) + "/confirm", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
    });
    var json = await res.json().catch(function () {
      return null;
    });
    if (!res.ok) {
      throw new Error((json && json.error) || "Falha ao confirmar o codigo.");
    }
  }

  // ---------------------------------------------------------------------------
  // WebAuthn ceremony
  // ---------------------------------------------------------------------------
  // navigator.credentials.get() must run in the PAGE world (webauthn-main.js,
  // manifest "world": "MAIN"): password managers such as 1Password only hook it
  // there, and from this isolated world Chrome asked for a physical security key.
  // The challenge goes over window.postMessage; the answer is the serialized
  // assertion. Network calls stay in this script (the page CSP would block them
  // from the page world).
  var bridgeSeq = 0;

  function getCredentialInPageWorld(pk) {
    return new Promise(function (resolve, reject) {
      var id = "evo-" + Date.now() + "-" + ++bridgeSeq;
      var timer = null;

      function cleanup() {
        window.removeEventListener("message", onMessage);
        if (timer) clearTimeout(timer);
      }

      function onMessage(ev) {
        if (ev.source !== window || !ev.data || ev.data.type !== "evo-wapk:response" || ev.data.id !== id) return;
        cleanup();
        if (ev.data.ok) resolve(ev.data.credential);
        else reject(new Error(ev.data.error || "Falha na autenticacao."));
      }

      window.addEventListener("message", onMessage);
      // Small grace period over the WebAuthn timeout; also covers the case where the
      // page-world script is missing (browser older than Chrome 111).
      timer = setTimeout(function () {
        cleanup();
        reject(new Error("Sem resposta da chave de acesso. Verifique se a extensao esta atualizada e se o Chrome e 111 ou superior."));
      }, ((pk && pk.timeout) || 60000) + 5000);

      window.postMessage({ type: "evo-wapk:request", id: id, publicKey: pk }, window.location.origin);
    });
  }

  // ---------------------------------------------------------------------------
  // UI
  // ---------------------------------------------------------------------------
  var prefersDark = false;
  try {
    prefersDark = window.matchMedia && window.matchMedia("(prefers-color-scheme: dark)").matches;
  } catch (e) {}

  var THEME = prefersDark
    ? { bg: "#111b21", fg: "#e9edef", sub: "#8696a0", border: "#25d366", card: "#202c33" }
    : { bg: "#ffffff", fg: "#111b21", sub: "#667781", border: "#25d366", card: "#f0f2f5" };

  function el(tag, styles, text) {
    var node = document.createElement(tag);
    if (styles) node.setAttribute("style", styles);
    if (text != null) node.textContent = text;
    return node;
  }

  var PANEL_STYLE = [
    "position:fixed",
    "top:18px",
    "right:18px",
    "z-index:2147483647",
    "width:340px",
    "max-width:calc(100vw - 36px)",
    "background:" + THEME.bg,
    "color:" + THEME.fg,
    "border:1px solid " + THEME.border,
    "border-radius:14px",
    "box-shadow:0 12px 34px rgba(0,0,0,0.28)",
    "font-family:-apple-system,Segoe UI,Roboto,Helvetica,Arial,sans-serif",
    "padding:18px",
  ].join(";");

  var BTN_STYLE = [
    "display:block",
    "width:100%",
    "box-sizing:border-box",
    "background:#25d366",
    "color:#04391f",
    "border:none",
    "border-radius:9px",
    "padding:11px 14px",
    "font-size:14px",
    "font-weight:700",
    "cursor:pointer",
    "margin-top:12px",
  ].join(";");

  var ui = null;

  function ensurePanel() {
    if (ui) return ui;

    var panel = el("div", PANEL_STYLE);
    panel.id = "evo-wapk-panel";

    var closeBtn = el(
      "div",
      "position:absolute;top:10px;right:14px;cursor:pointer;color:" + THEME.sub + ";font-size:20px;line-height:1;",
      "\u00d7"
    );
    closeBtn.title = "Fechar";
    closeBtn.addEventListener("click", function () {
      panel.remove();
      ui = null;
      stopPolling();
    });

    var title = el("div", "font-size:15px;font-weight:800;margin-bottom:4px;display:flex;align-items:center;gap:8px;");
    title.appendChild(el("span", "font-size:16px;", "\uD83D\uDD11"));
    title.appendChild(el("span", null, "Chave de acesso (passkey)"));

    var desc = el("div", "font-size:13px;line-height:1.5;color:" + THEME.sub + ";", "");

    var codeBox = el(
      "div",
      "display:none;margin-top:12px;padding:10px;border-radius:9px;background:" +
        THEME.card +
        ";text-align:center;font-family:ui-monospace,SFMono-Regular,Menlo,monospace;font-size:22px;font-weight:800;letter-spacing:3px;"
    );

    var status = el("div", "font-size:12px;margin-top:10px;color:" + THEME.sub + ";min-height:16px;");

    var btn = el("button", BTN_STYLE, "Autenticar com chave de acesso");
    btn.style.display = "none";

    panel.appendChild(closeBtn);
    panel.appendChild(title);
    panel.appendChild(desc);
    panel.appendChild(codeBox);
    panel.appendChild(btn);
    panel.appendChild(status);
    document.body.appendChild(panel);

    ui = { panel: panel, desc: desc, codeBox: codeBox, status: status, btn: btn };
    return ui;
  }

  function setDesc(text) {
    ensurePanel().desc.textContent = text;
  }

  function setStatus(text, kind) {
    var u = ensurePanel();
    u.status.textContent = text || "";
    u.status.style.color = kind === "error" ? "#e02e2a" : kind === "success" ? "#1fa855" : THEME.sub;
  }

  function showButton(label, onClick) {
    var u = ensurePanel();
    u.btn.textContent = label;
    u.btn.style.display = "block";
    u.btn.disabled = false;
    u.btn.style.opacity = "1";
    u.btn.style.cursor = "pointer";
    u.btn.onclick = onClick;
  }

  function hideButton() {
    ensurePanel().btn.style.display = "none";
  }

  function disableButton() {
    var u = ensurePanel();
    u.btn.disabled = true;
    u.btn.style.opacity = "0.6";
    u.btn.style.cursor = "default";
  }

  function showCode(code) {
    var u = ensurePanel();
    if (code) {
      u.codeBox.textContent = code;
      u.codeBox.style.display = "block";
    } else {
      u.codeBox.style.display = "none";
    }
  }

  // ---------------------------------------------------------------------------
  // State machine driven by polling
  // ---------------------------------------------------------------------------
  var pollTimer = null;
  var started = false; // ja passamos por algum estagio ativo
  var busy = false; // executando credentials.get / POST

  function stopPolling() {
    if (pollTimer) {
      clearTimeout(pollTimer);
      pollTimer = null;
    }
  }

  function schedulePoll(cer) {
    stopPolling();
    pollTimer = setTimeout(function () {
      poll(cer);
    }, POLL_INTERVAL_MS);
  }

  async function authenticate(cer, pk) {
    busy = true;
    disableButton();
    showCode("");
    setStatus("Aguardando a sua chave de acesso...");
    try {
      var assertion = await getCredentialInPageWorld(pk);
      setStatus("Enviando assinatura...");
      await sendResponse(cer, assertion);
      hideButton();
      setStatus("Assinatura enviada. Concluindo pareamento...");
    } catch (e) {
      setStatus((e && e.message) || String(e), "error");
      showButton("Tentar novamente", function () {
        authenticate(cer, pk);
      });
    } finally {
      busy = false;
      schedulePoll(cer);
    }
  }

  async function confirmCode(cer) {
    busy = true;
    disableButton();
    setStatus("Confirmando...");
    try {
      await sendConfirm(cer);
      hideButton();
      setStatus("Confirmado. Concluindo pareamento...");
    } catch (e) {
      setStatus((e && e.message) || String(e), "error");
      showButton("Tentar novamente", function () {
        confirmCode(cer);
      });
    } finally {
      busy = false;
      schedulePoll(cer);
    }
  }

  async function poll(cer) {
    if (busy) {
      schedulePoll(cer);
      return;
    }

    var st;
    try {
      st = await fetchStatus(cer);
    } catch (e) {
      setStatus((e && e.message) || String(e), "error");
      schedulePoll(cer);
      return;
    }

    var stage = st.stage || "";

    switch (stage) {
      case "challenge":
        started = true;
        setDesc("Clique para concluir o pareamento com a sua chave de acesso do WhatsApp.");
        showCode("");
        if (!busy) {
          showButton("Autenticar com chave de acesso", function () {
            authenticate(cer, st.publicKey);
          });
        }
        schedulePoll(cer);
        break;

      case "awaiting_confirmation":
        started = true;
        hideButton();
        showCode("");
        setDesc("Assinatura enviada ao WhatsApp.");
        setStatus("Aguardando codigo de confirmacao...");
        schedulePoll(cer);
        break;

      case "confirmation":
        started = true;
        setDesc("Verifique se o codigo abaixo e o mesmo exibido no seu celular.");
        showCode(st.code);
        if (st.skipHandoffUX) {
          hideButton();
          setStatus("Confirmando automaticamente...");
        } else {
          setStatus("");
          showButton("Confirmar codigo", function () {
            confirmCode(cer);
          });
        }
        schedulePoll(cer);
        break;

      case "confirmed":
        started = true;
        hideButton();
        showCode("");
        setDesc("Chave de acesso confirmada.");
        setStatus("Concluindo pareamento...");
        schedulePoll(cer);
        break;

      case "error":
        started = true;
        showCode("");
        setDesc("Ocorreu um erro no pareamento por chave de acesso.");
        setStatus(st.error || "Erro desconhecido.", "error");
        // Permite reiniciar caso o backend reemita o desafio
        schedulePoll(cer);
        break;

      case "":
      default:
        if (started) {
          // Estado limpo apos ter iniciado = pareamento concluido (PairSuccess)
          hideButton();
          showCode("");
          setDesc("Pareamento concluido com sucesso!");
          setStatus("Pode voltar ao WhatyGo. Esta aba ja pode ser fechada.", "success");
          clearCeremony();
          stopPolling();
        } else {
          setDesc("Aguardando o desafio de chave de acesso do WhatsApp...");
          setStatus("Escaneie o QR no WhatyGo para iniciar.");
          schedulePoll(cer);
        }
        break;
    }
  }

  // ---------------------------------------------------------------------------
  // Init
  // ---------------------------------------------------------------------------
  function start() {
    var cer = getCeremony();
    if (!cer) return;
    ensurePanel();
    setDesc("Preparando cerimonia de chave de acesso...");
    poll(cer);
  }

  if (document.body) {
    start();
  } else {
    document.addEventListener("DOMContentLoaded", start);
  }
})();
