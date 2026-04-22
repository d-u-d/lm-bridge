import './style.css';
import './app.css';

import { GetModelInfo, GetRecentCalls, GetStats, GetIntegrationEnabled, SetIntegration, GetActiveTask, CancelActiveTask, GetVersion } from '../wailsjs/go/main/App';

// ── helpers ──────────────────────────────────────────────────

function fmtTokens(n) {
  if (!n) return '—';
  return n >= 1000 ? (n / 1000).toFixed(1) + 'k' : String(n);
}

function fmtLatency(ms) {
  if (!ms) return '—';
  return ms >= 1000 ? (ms / 1000).toFixed(1) + 's' : ms + 'ms';
}

function isSlowMs(ms) {
  return ms > 5000;
}

// ── render ───────────────────────────────────────────────────

function renderApp(modelInfo, calls, stats, integrationEnabled) {
  const statusDot  = modelInfo.online ? 'online' : 'offline';
  const statusText = modelInfo.online ? 'Online' : 'Offline';
  const modelLabel = modelInfo.model_name
    ? shortModelName(modelInfo.model_name)
    : (modelInfo.online ? 'Model loaded' : 'LM Studio offline');

  document.getElementById('app').innerHTML = `
    <div class="header">
      <span class="header-title">LM Bridge</span>
      <span class="header-sep"></span>
      <div class="model-status">
        <span class="dot ${statusDot}"></span>
        <span class="model-name">${modelLabel}</span>
        <span style="color:var(--muted)">${statusText}</span>
      </div>
      <button class="refresh-btn" id="refreshBtn">↻ Refresh</button>
    </div>

    <div id="activeTaskBar" class="active-task-bar" style="display:none"></div>

    <div class="integration-bar ${integrationEnabled ? 'enabled' : 'disabled'}">
      <span class="integration-dot"></span>
      <span class="integration-label">Claude Code Integration</span>
      <span class="integration-status">${integrationEnabled ? 'ON' : 'OFF'}</span>
      <button class="integration-toggle" id="integrationToggle">
        ${integrationEnabled ? 'Disable' : 'Enable'}
      </button>
    </div>

    <div class="stats-bar">
      <div class="stat">
        <span class="stat-value">${stats.total_calls ?? 0}</span>
        <span class="stat-label">Calls</span>
      </div>
      <div class="stat">
        <span class="stat-value accent">${fmtTokens(stats.total_tokens)}</span>
        <span class="stat-label">Tokens used</span>
      </div>
      <div class="stat">
        <span class="stat-value">${fmtLatency(stats.avg_latency_ms)}</span>
        <span class="stat-label">Avg latency</span>
      </div>
      <div class="stat">
        <span class="stat-value green">~${fmtTokens(stats.saved_tokens)}</span>
        <span class="stat-label">Saved from Claude ctx</span>
      </div>
    </div>

    <div class="calls-header">
      <span class="calls-title">Recent Calls</span>
      <span class="calls-count">${calls.length}</span>
    </div>

    <div class="cols-header">
      <span>Time</span>
      <span>Mode</span>
      <span>Task</span>
      <span style="text-align:right">Latency</span>
      <span style="text-align:right">Tokens</span>
      <span style="text-align:center">OK</span>
    </div>

    <div class="calls-scroll" id="callsList">
      ${calls.length === 0 ? renderEmpty() : calls.map(renderRow).join('')}
    </div>

    <div class="footer" id="version">…</div>
  `;

  document.getElementById('refreshBtn').addEventListener('click', load);

  document.getElementById('integrationToggle').addEventListener('click', async () => {
    await SetIntegration(!integrationEnabled);
    load();
  });
}

function renderEmpty() {
  return `
    <div class="empty-state">
      <div class="icon">🤖</div>
      <p>No calls yet.</p>
      <p>Try: <code>lm-bridge query "hello"</code></p>
    </div>
  `;
}

function renderRow(c) {
  const ok = !c.error;
  const taskClass = ok ? '' : 'error';
  const taskText  = ok ? escHtml(c.task) : `✗ ${escHtml(c.error || c.task)}`;
  const latClass  = isSlowMs(c.latency_ms) ? 'slow' : '';

  return `
    <div class="call-row" title="${escAttr(c.task)}">
      <span class="call-time">${escHtml(c.time)}</span>
      <span class="badge ${escHtml(c.mode)}">${escHtml(c.mode)}</span>
      <span class="call-task ${taskClass}">${taskText}</span>
      <span class="call-latency ${latClass}">${fmtLatency(c.latency_ms)}</span>
      <span class="call-tokens">${fmtTokens(c.tokens)}</span>
      <span class="call-status">${ok ? '✓' : '✗'}</span>
    </div>
  `;
}

function shortModelName(name) {
  // "lmstudio-community/Qwen3-Coder-30B-A3B-GGUF/..." → "Qwen3-Coder-30B-A3B"
  const parts = name.split('/');
  const base  = parts[parts.length - 1] || parts[0];
  return base.replace(/[-_](GGUF|gguf|Q\d.*|q\d.*|fp\d.*).*$/, '').substring(0, 40);
}

function escHtml(s) {
  return String(s ?? '')
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;');
}

function escAttr(s) {
  return escHtml(s).replace(/"/g, '&quot;');
}

// ── load & bootstrap ─────────────────────────────────────────

let appVersion = '…';

async function load() {
  try {
    const [modelInfo, calls, stats, integrationEnabled] = await Promise.all([
      GetModelInfo(),
      GetRecentCalls(),
      GetStats(),
      GetIntegrationEnabled(),
    ]);
    renderApp(modelInfo, calls, stats, integrationEnabled);
  } catch (err) {
    renderApp(
      { online: false, model_name: '' },
      [],
      { total_calls: 0, total_tokens: 0, avg_latency_ms: 0, saved_tokens: 0 },
      false,
    );
  }
  const el = document.getElementById('version');
  if (el) el.textContent = appVersion;
}

// Fetch version once at startup
GetVersion().then(v => { appVersion = v; const el = document.getElementById('version'); if (el) el.textContent = v; }).catch(() => {});

load();

// ── Active task polling ───────────────────────────────────────────────────────

let taskTimer = null;

async function pollActiveTask() {
  let task = null;
  try { task = await GetActiveTask(); } catch (_) {}

  const bar = document.getElementById('activeTaskBar');
  if (!bar) return;

  if (!task) {
    bar.style.display = 'none';
    return;
  }

  const pct = task.progress ?? 0;
  const phase = pct >= 100 ? '⚙️ Generating…' : `📥 Loading prompt ${pct.toFixed(0)}%`;
  const elapsed = task.elapsed_s >= 60
    ? `${Math.floor(task.elapsed_s/60)}m ${task.elapsed_s%60}s`
    : `${task.elapsed_s}s`;

  bar.style.display = 'flex';
  bar.innerHTML = `
    <span class="at-badge ${task.mode}">${task.mode}</span>
    <span class="at-phase">${phase}</span>
    <div class="at-bar"><div class="at-fill" style="width:${Math.min(pct,100)}%"></div></div>
    <span class="at-elapsed">${elapsed}</span>
    <button class="at-cancel" id="cancelBtn">✕</button>
  `;
  document.getElementById('cancelBtn').addEventListener('click', async () => {
    try { await CancelActiveTask(); } catch (_) {}
    bar.style.display = 'none';
  });
}

function startTaskPolling() {
  if (taskTimer) return;
  taskTimer = setInterval(pollActiveTask, 2000);
}

function stopTaskPolling() {
  if (taskTimer) { clearInterval(taskTimer); taskTimer = null; }
}

// Запускаем polling при открытии окна
startTaskPolling();
pollActiveTask();
