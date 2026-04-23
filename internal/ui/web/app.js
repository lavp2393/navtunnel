(() => {
  // Token auth: se toma del query string y viaja en todas las requests.
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

  // --- Refs -------------------------------------------------------------
  const dot = $("status-dot");
  const statusText = $("status-text");
  const controlsState = $("controls-state");
  const btnConnect = $("btn-connect");
  const btnDisconnect = $("btn-disconnect");
  const ovpnPath = $("ovpn-path");
  const btnBrowse = $("btn-browse");
  const btnSaveConfig = $("btn-save-config");
  const btnClearLogs = $("btn-clear-logs");
  const logs = $("logs");
  const modal = $("modal");
  const modalTitle = $("modal-title");
  const modalMessage = $("modal-message");
  const modalForm = $("modal-form");
  const modalInput = $("modal-input");
  const modalRemember = $("modal-remember");
  const modalRememberLabel = $("modal-remember-label");
  const modalCancel = $("modal-cancel");

  // --- Tabs -------------------------------------------------------------
  document.querySelectorAll(".tab").forEach((btn) => {
    btn.addEventListener("click", () => {
      document.querySelectorAll(".tab").forEach((b) => {
        b.classList.remove("active");
        b.setAttribute("aria-selected", "false");
      });
      btn.classList.add("active");
      btn.setAttribute("aria-selected", "true");
      const target = btn.dataset.tab;
      document.querySelectorAll(".tab-panel").forEach((p) => p.classList.remove("active"));
      $("tab-" + target).classList.add("active");
    });
  });

  // --- Estado UI --------------------------------------------------------
  function setDot(cls) { dot.className = "dot " + cls; }

  const WAIT_STATES = new Set([
    "CONNECTING", "WAIT", "AUTH", "GET_CONFIG", "ASSIGN_IP",
    "ADD_ROUTES", "RECONNECTING", "RESOLVE", "TCP_CONNECT",
  ]);

  function setStatus(state, connected) {
    statusText.textContent = state.toUpperCase();
    controlsState.textContent = state;
    if (connected) {
      setDot("dot-on");
      btnConnect.disabled = true;
      btnDisconnect.disabled = false;
    } else if (WAIT_STATES.has(state)) {
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

  // Limitamos el DOM a las últimas N líneas: concatenar textContent sin
  // límite es O(n) por update y degrada muy rápido cuando openvpn dispara
  // logs durante la conexión.
  const MAX_LOG_LINES = 400;
  const logLines = [];
  function appendLog(line) {
    logLines.push(line);
    if (logLines.length > MAX_LOG_LINES) {
      logLines.splice(0, logLines.length - MAX_LOG_LINES);
    }
    logs.textContent = logLines.join("\n") + "\n";
    logs.scrollTop = logs.scrollHeight;
  }

  function humanBytes(n) {
    if (n < 1024) return n + " B";
    const units = ["KB", "MB", "GB", "TB"];
    let i = -1;
    do { n /= 1024; i++; } while (n >= 1024 && i < units.length - 1);
    return n.toFixed(n >= 10 ? 1 : 2) + " " + units[i];
  }
  const humanRate = (n) => humanBytes(n) + "/s";

  // --- Chart ------------------------------------------------------------
  //
  // Buffer circular de muestras (rx/tx) dibujado con canvas 2D. Se repinta
  // en cada requestAnimationFrame y la escala Y se auto-ajusta al máximo
  // reciente. Sin dependencias JS.

  const CHART_SAMPLES = 180; // ~3 min a 1 Hz
  const rxBuf = new Array(CHART_SAMPLES).fill(0);
  const txBuf = new Array(CHART_SAMPLES).fill(0);

  function pushSample(rx, tx) {
    rxBuf.shift(); rxBuf.push(rx);
    txBuf.shift(); txBuf.push(tx);
    markChartDirty();
  }

  const canvas = $("traffic-chart");
  const ctx = canvas.getContext("2d");
  let dpr = Math.max(1, window.devicePixelRatio || 1);

  function resizeCanvas() {
    dpr = Math.max(1, window.devicePixelRatio || 1);
    const rect = canvas.getBoundingClientRect();
    canvas.width = Math.floor(rect.width * dpr);
    canvas.height = Math.floor(rect.height * dpr);
  }
  window.addEventListener("resize", () => { resizeCanvas(); markChartDirty(); });

  function niceMax(max) {
    if (max <= 0) return 1024;
    const pow = Math.pow(10, Math.floor(Math.log10(max)));
    const n = max / pow;
    let nice = 10;
    if (n <= 1) nice = 1;
    else if (n <= 2) nice = 2;
    else if (n <= 5) nice = 5;
    return nice * pow;
  }

  function drawChart() {
    const w = canvas.width, h = canvas.height;
    if (w === 0 || h === 0) return;
    ctx.clearRect(0, 0, w, h);

    // Grid
    ctx.strokeStyle = "rgba(120, 220, 255, 0.08)";
    ctx.lineWidth = 1;
    for (let i = 1; i < 5; i++) {
      const y = (h * i) / 5;
      ctx.beginPath();
      ctx.moveTo(0, y);
      ctx.lineTo(w, y);
      ctx.stroke();
    }
    for (let i = 1; i < 6; i++) {
      const x = (w * i) / 6;
      ctx.beginPath();
      ctx.moveTo(x, 0);
      ctx.lineTo(x, h);
      ctx.stroke();
    }

    const rawMax = Math.max(1, ...rxBuf, ...txBuf);
    const max = niceMax(rawMax * 1.15);

    function drawSeries(buf, stroke, fill, glow) {
      ctx.save();
      ctx.shadowBlur = glow;
      ctx.shadowColor = stroke;

      // Área bajo la línea.
      ctx.beginPath();
      for (let i = 0; i < buf.length; i++) {
        const x = (i / (buf.length - 1)) * w;
        const y = h - (buf[i] / max) * h * 0.92;
        if (i === 0) ctx.moveTo(x, y);
        else ctx.lineTo(x, y);
      }
      ctx.lineTo(w, h);
      ctx.lineTo(0, h);
      ctx.closePath();
      ctx.fillStyle = fill;
      ctx.fill();

      // Línea.
      ctx.beginPath();
      for (let i = 0; i < buf.length; i++) {
        const x = (i / (buf.length - 1)) * w;
        const y = h - (buf[i] / max) * h * 0.92;
        if (i === 0) ctx.moveTo(x, y);
        else ctx.lineTo(x, y);
      }
      ctx.strokeStyle = stroke;
      ctx.lineWidth = 2 * dpr;
      ctx.lineJoin = "round";
      ctx.stroke();
      ctx.restore();
    }

    // RX (cian)
    const gradRx = ctx.createLinearGradient(0, 0, 0, h);
    gradRx.addColorStop(0, "rgba(0, 240, 255, 0.35)");
    gradRx.addColorStop(1, "rgba(0, 240, 255, 0)");
    drawSeries(rxBuf, "rgba(0, 240, 255, 0.95)", gradRx, 12 * dpr);

    // TX (magenta)
    const gradTx = ctx.createLinearGradient(0, 0, 0, h);
    gradTx.addColorStop(0, "rgba(255, 61, 160, 0.30)");
    gradTx.addColorStop(1, "rgba(255, 61, 160, 0)");
    drawSeries(txBuf, "rgba(255, 61, 160, 0.95)", gradTx, 12 * dpr);

    // Etiqueta Y del pico en la esquina.
    ctx.save();
    ctx.fillStyle = "rgba(108, 127, 168, 0.7)";
    ctx.font = `${10 * dpr}px JetBrains Mono, monospace`;
    ctx.textBaseline = "top";
    ctx.fillText("MAX ≈ " + humanRate(max), 6 * dpr, 6 * dpr);
    ctx.restore();
  }

  // El chart se anima a ~20 fps en lugar de 60: webkit2gtk con glow+gradients
  // consume CPU considerable a 60 fps y no aporta nada perceptible con
  // datos que llegan a 1 Hz del bytecount.
  let chartDirty = true;
  function markChartDirty() { chartDirty = true; }
  function animate() {
    if (chartDirty) {
      drawChart();
      chartDirty = false;
    }
  }
  setInterval(animate, 50);

  // --- Modal ------------------------------------------------------------
  let modalSubmit = null;

  function openModal({ title, message, type, showRemember, onSubmit }) {
    modalTitle.textContent = title;
    modalMessage.textContent = message || "";
    modalInput.type = type || "text";
    modalInput.value = "";
    modalRememberLabel.classList.toggle("hidden", !showRemember);
    modal.classList.remove("hidden");
    setTimeout(() => modalInput.focus(), 40);
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

  // --- SSE --------------------------------------------------------------
  function subscribeEvents() {
    const es = new EventSource("/api/events?t=" + encodeURIComponent(token));
    es.onmessage = (ev) => {
      let msg;
      try { msg = JSON.parse(ev.data); } catch { return; }
      handleServerEvent(msg);
    };
    es.onerror = () => {
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
        if (msg.localTunIP) $("m-tun").textContent = msg.localTunIP;
        if (msg.remoteIP) $("m-server").textContent = msg.remoteIP;
        break;
      case "connected":
        setStatus("CONNECTED", true);
        break;
      case "disconnected":
        setStatus("DESCONECTADO", false);
        pushSample(0, 0); // cierre suave del chart
        break;
      case "bytecount": {
        $("m-rx").textContent = humanBytes(msg.bytesIn || 0);
        $("m-tx").textContent = humanBytes(msg.bytesOut || 0);
        const rIn = msg.rateIn || 0, rOut = msg.rateOut || 0;
        $("ro-rx-rate").textContent = humanRate(rIn);
        $("ro-tx-rate").textContent = humanRate(rOut);
        pushSample(rIn, rOut);
        break;
      }
      case "ask-user":
        openModal({
          title: "USUARIO",
          message: msg.message || "Ingresa tu usuario corporativo",
          type: "text",
          showRemember: true,
          onSubmit: ({ value, remember }) => sendCredential("user", value, remember),
        });
        break;
      case "ask-pass":
        openModal({
          title: "CONTRASEÑA",
          message: msg.message || "Ingresa tu contraseña",
          type: "password",
          showRemember: true,
          onSubmit: ({ value, remember }) => sendCredential("pass", value, remember),
        });
        break;
      case "ask-otp":
        openModal({
          title: "CÓDIGO OTP",
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
    setDot("dot-wait");
    statusText.textContent = "INICIANDO...";
    try {
      const r = await api("/api/connect", { method: "POST", body: "{}" });
      if (!r.ok) {
        const body = await r.text();
        appendLog("Error al conectar: " + body);
        setStatus("DESCONECTADO", false);
      }
    } catch (e) {
      appendLog("Error al conectar: " + e.message);
      setStatus("DESCONECTADO", false);
    }
  });

  btnDisconnect.addEventListener("click", async () => {
    btnDisconnect.disabled = true;
    btnConnect.disabled = true;
    statusText.textContent = "DESCONECTANDO...";
    setDot("dot-wait");
    try { await api("/api/disconnect", { method: "POST", body: "{}" }); } catch {}
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
    if (r.ok) appendLog("✓ Configuración guardada: " + path);
    else appendLog("Error guardando configuración: " + (await r.text()));
  });

  btnClearLogs.addEventListener("click", () => {
    logLines.length = 0;
    logs.textContent = "";
  });

  // --- Bootstrap --------------------------------------------------------
  async function init() {
    resizeCanvas();
    markChartDirty();

    try {
      const r = await api("/api/state");
      if (r.ok) {
        const s = await r.json();
        if (s.ovpnPath) ovpnPath.value = s.ovpnPath;
        if (s.metrics) {
          const m = s.metrics;
          setStatus(m.state || "DESCONECTADO", m.state === "CONNECTED");
          if (m.localTunnelIP) $("m-tun").textContent = m.localTunnelIP;
          if (m.remoteServerIP) $("m-server").textContent = m.remoteServerIP;
          $("m-rx").textContent = humanBytes(m.bytesIn || 0);
          $("m-tx").textContent = humanBytes(m.bytesOut || 0);
          $("ro-rx-rate").textContent = humanRate(m.bytesInRate || 0);
          $("ro-tx-rate").textContent = humanRate(m.bytesOutRate || 0);
        }
        if (s.recentLogs) for (const l of s.recentLogs) appendLog(l);
      }
    } catch {}
    subscribeEvents();
  }

  init();
})();
