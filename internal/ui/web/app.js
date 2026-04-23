(() => {
  // Token auth: lo tomamos del query string y lo mandamos en cada request.
  const params = new URLSearchParams(location.search);
  const token = params.get("t") || "";

  const api = (path, opts = {}) => {
    const url = path + (path.includes("?") ? "&" : "?") + "t=" + encodeURIComponent(token);
    return fetch(url, {
      ...opts,
      headers: { "Content-Type": "application/json", ...(opts.headers || {}) },
    });
  };

  const $ = (id) => document.getElementById(id);

  const dot = $("status-dot");
  const statusText = $("status-text");
  const btnConnect = $("btn-connect");
  const btnDisconnect = $("btn-disconnect");
  const ovpnPath = $("ovpn-path");
  const btnBrowse = $("btn-browse");
  const btnSaveConfig = $("btn-save-config");
  const logs = $("logs");
  const modal = $("modal");
  const modalTitle = $("modal-title");
  const modalMessage = $("modal-message");
  const modalForm = $("modal-form");
  const modalInput = $("modal-input");
  const modalRemember = $("modal-remember");
  const modalRememberLabel = $("modal-remember-label");
  const modalCancel = $("modal-cancel");

  let lastRx = 0, lastTx = 0;

  // --- Estado de UI -----------------------------------------------------
  function setDot(cls) {
    dot.className = "dot " + cls;
  }

  function setStatus(state, connected) {
    statusText.textContent = state;
    if (connected) {
      setDot("dot-on");
      btnConnect.disabled = true;
      btnDisconnect.disabled = false;
    } else if (["CONNECTING", "WAIT", "AUTH", "GET_CONFIG", "ASSIGN_IP", "ADD_ROUTES", "RECONNECTING", "RESOLVE", "TCP_CONNECT"].includes(state)) {
      setDot("dot-wait");
      btnConnect.disabled = true;
      btnDisconnect.disabled = false;
    } else if (state === "ERROR") {
      setDot("dot-err");
      btnConnect.disabled = false;
      btnDisconnect.disabled = true;
    } else {
      setDot("dot-off");
      btnConnect.disabled = false;
      btnDisconnect.disabled = true;
    }
  }

  function appendLog(line) {
    logs.textContent += line + "\n";
    logs.scrollTop = logs.scrollHeight;
  }

  function humanBytes(n) {
    if (n < 1024) return n + " B";
    const units = ["KB", "MB", "GB", "TB"];
    let i = -1;
    do { n /= 1024; i++; } while (n >= 1024 && i < units.length - 1);
    return n.toFixed(n >= 10 ? 1 : 2) + " " + units[i];
  }

  function humanRate(n) { return humanBytes(n) + "/s"; }

  function relTime(iso) {
    if (!iso) return "—";
    const t = new Date(iso).getTime();
    if (!t) return "—";
    const secs = Math.floor((Date.now() - t) / 1000);
    if (secs < 60) return secs + "s";
    if (secs < 3600) return Math.floor(secs / 60) + "m " + (secs % 60) + "s";
    const h = Math.floor(secs / 3600);
    const m = Math.floor((secs % 3600) / 60);
    return h + "h " + m + "m";
  }

  // --- Modal ------------------------------------------------------------
  let modalSubmit = null;

  function openModal({ title, message, type, showRemember, onSubmit }) {
    modalTitle.textContent = title;
    modalMessage.textContent = message || "";
    modalInput.type = type || "text";
    modalInput.value = "";
    modalRememberLabel.classList.toggle("hidden", !showRemember);
    modal.classList.remove("hidden");
    modalInput.focus();
    modalSubmit = onSubmit;
  }

  function closeModal() {
    modal.classList.add("hidden");
    modalSubmit = null;
  }

  modalForm.addEventListener("submit", (e) => {
    e.preventDefault();
    const value = modalInput.value;
    const remember = modalRemember.checked;
    if (modalSubmit) modalSubmit({ value, remember });
    closeModal();
  });

  modalCancel.addEventListener("click", closeModal);

  // --- Eventos del servidor --------------------------------------------
  function subscribeEvents() {
    const es = new EventSource("/api/events?t=" + encodeURIComponent(token));
    es.onmessage = (ev) => {
      let msg;
      try { msg = JSON.parse(ev.data); } catch { return; }
      handleServerEvent(msg);
    };
    es.onerror = () => {
      // El server puede reiniciarse; reintentamos cada 2 s.
      es.close();
      setTimeout(subscribeEvents, 2000);
    };
  }

  function handleServerEvent(msg) {
    switch (msg.type) {
      case "log":
        appendLog(msg.message);
        break;
      case "state":
        setStatus(msg.state, msg.state === "CONNECTED");
        $("m-state").textContent = msg.state;
        if (msg.localTunIP) $("m-tun").textContent = msg.localTunIP;
        if (msg.remoteIP) $("m-server").textContent = msg.remoteIP;
        break;
      case "connected":
        setStatus("CONNECTED", true);
        break;
      case "disconnected":
        setStatus("Desconectado", false);
        $("m-state").textContent = "—";
        break;
      case "bytecount":
        $("m-rx").textContent = humanBytes(msg.bytesIn);
        $("m-tx").textContent = humanBytes(msg.bytesOut);
        if (msg.rateIn !== undefined) $("m-rx-rate").textContent = humanRate(msg.rateIn);
        if (msg.rateOut !== undefined) $("m-tx-rate").textContent = humanRate(msg.rateOut);
        break;
      case "ask-user":
        openModal({
          title: "Usuario",
          message: msg.message || "Ingresa tu usuario corporativo",
          type: "text",
          showRemember: true,
          onSubmit: ({ value, remember }) => sendCredential("user", value, remember),
        });
        break;
      case "ask-pass":
        openModal({
          title: "Contraseña",
          message: msg.message || "Ingresa tu contraseña",
          type: "password",
          showRemember: true,
          onSubmit: ({ value, remember }) => sendCredential("pass", value, remember),
        });
        break;
      case "ask-otp":
        openModal({
          title: "Código OTP",
          message: msg.message || "Ingresa tu código de un solo uso",
          type: "text",
          showRemember: false,
          onSubmit: ({ value }) => sendCredential("otp", value, false),
        });
        break;
      case "auth-failed":
        appendLog("✗ " + (msg.message || "Autenticación fallida"));
        break;
      case "fatal":
        appendLog("✗ FATAL: " + (msg.message || "Error fatal"));
        setStatus("ERROR", false);
        break;
    }
  }

  async function sendCredential(stage, value, remember) {
    await api("/api/credential", {
      method: "POST",
      body: JSON.stringify({ stage, value, remember }),
    });
  }

  // --- Acciones ---------------------------------------------------------
  btnConnect.addEventListener("click", async () => {
    btnConnect.disabled = true;
    try {
      const r = await api("/api/connect", { method: "POST", body: "{}" });
      if (!r.ok) {
        const body = await r.text();
        appendLog("Error al conectar: " + body);
        btnConnect.disabled = false;
      }
    } catch (e) {
      appendLog("Error al conectar: " + e.message);
      btnConnect.disabled = false;
    }
  });

  btnDisconnect.addEventListener("click", async () => {
    // Feedback inmediato: el backend hace la limpieza async y confirma
    // por SSE con un evento "disconnected".
    btnDisconnect.disabled = true;
    btnConnect.disabled = true;
    statusText.textContent = "Desconectando...";
    setDot("dot-wait");
    try {
      await api("/api/disconnect", { method: "POST", body: "{}" });
    } catch {}
  });

  btnBrowse.addEventListener("click", async () => {
    try {
      const r = await api("/api/pick-file", { method: "POST", body: "{}" });
      if (r.ok) {
        const { path } = await r.json();
        if (path) ovpnPath.value = path;
      } else if (r.status === 501) {
        appendLog("No hay diálogo nativo disponible, escribí la ruta manualmente.");
      }
    } catch (e) {
      appendLog("Error abriendo diálogo: " + e.message);
    }
  });

  btnSaveConfig.addEventListener("click", async () => {
    const path = ovpnPath.value.trim();
    if (!path) return;
    const r = await api("/api/config", {
      method: "POST",
      body: JSON.stringify({ ovpnPath: path }),
    });
    if (r.ok) {
      appendLog("✓ Configuración guardada: " + path);
    } else {
      appendLog("Error guardando configuración: " + (await r.text()));
    }
  });

  // --- Bootstrap --------------------------------------------------------
  async function init() {
    try {
      const r = await api("/api/state");
      if (r.ok) {
        const s = await r.json();
        if (s.ovpnPath) ovpnPath.value = s.ovpnPath;
        if (s.metrics) {
          setStatus(s.metrics.state || "Desconectado", s.metrics.state === "CONNECTED");
          if (s.metrics.state) $("m-state").textContent = s.metrics.state;
          if (s.metrics.localTunnelIP) $("m-tun").textContent = s.metrics.localTunnelIP;
          if (s.metrics.remoteServerIP) $("m-server").textContent = s.metrics.remoteServerIP;
          $("m-rx").textContent = humanBytes(s.metrics.bytesIn || 0);
          $("m-tx").textContent = humanBytes(s.metrics.bytesOut || 0);
        }
        if (s.recentLogs) {
          for (const l of s.recentLogs) appendLog(l);
        }
      }
    } catch {}
    subscribeEvents();
  }

  init();
})();
