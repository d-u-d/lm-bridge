import './style.css';
import './app.css';

import { GetModelInfo, GetRecentCalls, GetStats, GetIntegrationEnabled, SetIntegration, GetActiveTask, CancelActiveTask, GetVersion, GetProviderConfig, SaveProviderConfig, GetOpenRouterFreeModels } from '../wailsjs/go/main/App';
import { EventsOn } from '../wailsjs/runtime/runtime';

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

function escHtml(s) {
  return String(s ?? '')
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;');
}

function escAttr(s) {
  return escHtml(s).replace(/"/g, '&quot;');
}

// ── view state ───────────────────────────────────────────────

let currentView = 'dashboard';

// ── render dashboard ─────────────────────────────────────────

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
      <button class="refresh-btn" id="settingsBtn">⚙ Settings</button>
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
      <span>Model</span>
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
  document.getElementById('settingsBtn').addEventListener('click', () => {
    currentView = 'settings';
    renderSettingsView();
  });

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
  const modelShort = c.model ? shortModelName(c.model) : '—';
  const providerClass = c.provider === 'openrouter' ? 'or' : 'lm';

  return `
    <div class="call-row" title="${escAttr(c.task)}">
      <span class="call-time">${escHtml(c.time)}</span>
      <span class="badge ${escHtml(c.mode)}">${escHtml(c.mode)}</span>
      <span class="call-model" title="${escAttr(c.model)}">
        <span class="provider-dot ${providerClass}"></span>${escHtml(modelShort)}
      </span>
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

// ── render settings ──────────────────────────────────────────

async function renderSettingsView() {
  let cfg = { provider: 'lmstudio', lmstudio_url: '', openrouter_api_key: '', openrouter_model: '' };
  try { cfg = await GetProviderConfig(); } catch (_) {}

  const isOR = cfg.provider === 'openrouter';

  document.getElementById('app').innerHTML = `
    <div class="header">
      <button class="back-btn" id="backBtn">← Back</button>
      <span class="header-sep"></span>
      <span class="header-title" style="color:var(--muted)">Settings</span>
    </div>

    <div class="settings-panel">
      <div class="settings-group">
        <span class="settings-label">Provider</span>
        <div class="radio-group">
          <label class="radio-option">
            <input type="radio" name="provider" value="lmstudio" ${!isOR ? 'checked' : ''}>
            <span class="radio-text">
              <span class="radio-title">LM Studio</span>
              <span class="radio-desc">Local model, runs offline</span>
            </span>
          </label>
          <label class="radio-option">
            <input type="radio" name="provider" value="openrouter" ${isOR ? 'checked' : ''}>
            <span class="radio-text">
              <span class="radio-title">OpenRouter</span>
              <span class="radio-desc">Cloud API, free models available</span>
            </span>
          </label>
        </div>
      </div>

      <div class="settings-group" id="lmstudio-group" ${isOR ? 'style="display:none"' : ''}>
        <label class="settings-label" for="lmstudio-url">LM Studio URL</label>
        <input class="settings-input" type="text" id="lmstudio-url"
               value="${escAttr(cfg.lmstudio_url)}"
               placeholder="http://localhost:1234/v1">
      </div>

      <div class="settings-group" id="openrouter-group" ${!isOR ? 'style="display:none"' : ''}>
        <label class="settings-label" for="or-api-key">API Key</label>
        <input class="settings-input" type="password" id="or-api-key"
               value="${escAttr(cfg.openrouter_api_key)}"
               placeholder="sk-or-v1-...">

        <label class="settings-label" style="margin-top:12px">Model</label>
        <div class="model-row">
          <select class="settings-select" id="or-model">
            ${cfg.openrouter_model
              ? `<option value="${escAttr(cfg.openrouter_model)}" selected>${escHtml(cfg.openrouter_model)}</option>`
              : `<option value="">— select a model —</option>`}
          </select>
          <button class="load-btn" id="loadModelsBtn">Load free models</button>
        </div>
      </div>

      <div class="settings-footer">
        <button class="save-btn" id="saveBtn">Save</button>
        <span class="save-status" id="saveStatus"></span>
      </div>
    </div>

    <div class="footer" id="version">…</div>
  `;

  const el = document.getElementById('version');
  if (el) el.textContent = appVersion;

  document.getElementById('backBtn').addEventListener('click', () => {
    currentView = 'dashboard';
    load();
  });

  // Show/hide provider-specific sections
  document.querySelectorAll('input[name="provider"]').forEach(r => {
    r.addEventListener('change', () => {
      const sel = document.querySelector('input[name="provider"]:checked').value;
      document.getElementById('lmstudio-group').style.display = sel === 'openrouter' ? 'none' : 'block';
      document.getElementById('openrouter-group').style.display = sel === 'openrouter' ? 'block' : 'none';
    });
  });

  document.getElementById('loadModelsBtn').addEventListener('click', async () => {
    const btn = document.getElementById('loadModelsBtn');
    btn.disabled = true;
    btn.textContent = 'Loading…';

    // Temporarily save the key so backend can use it for the request
    const key = document.getElementById('or-api-key').value.trim();
    try {
      await SaveProviderConfig({
        provider: 'openrouter',
        lmstudio_url: document.getElementById('lmstudio-url').value.trim(),
        openrouter_api_key: key,
        openrouter_model: document.getElementById('or-model').value,
      });
    } catch (_) {}

    try {
      const models = (await GetOpenRouterFreeModels()) || [];
      const select = document.getElementById('or-model');
      const current = select.value;
      if (models.length === 0) {
        select.innerHTML = `<option value="">No free models found</option>`;
      } else {
        select.innerHTML = models.map(m =>
          `<option value="${escAttr(m)}" ${m === current ? 'selected' : ''}>${escHtml(m)}</option>`
        ).join('');
      }
    } catch (e) {
      const select = document.getElementById('or-model');
      select.innerHTML = `<option value="">Error: ${escHtml(String(e))}</option>`;
    }

    btn.disabled = false;
    btn.textContent = 'Load free models';
  });

  document.getElementById('saveBtn').addEventListener('click', async () => {
    const provider = document.querySelector('input[name="provider"]:checked').value;
    try {
      await SaveProviderConfig({
        provider,
        lmstudio_url: document.getElementById('lmstudio-url').value.trim(),
        openrouter_api_key: document.getElementById('or-api-key').value.trim(),
        openrouter_model: document.getElementById('or-model').value,
      });
      const s = document.getElementById('saveStatus');
      s.textContent = 'Saved ✓';
      setTimeout(() => { s.textContent = ''; }, 2000);
    } catch (e) {
      const s = document.getElementById('saveStatus');
      s.textContent = 'Error: ' + String(e);
      s.style.color = 'var(--red)';
    }
  });
}

// ── load & bootstrap ─────────────────────────────────────────

let appVersion = '…';

async function load() {
  if (currentView === 'settings') {
    await renderSettingsView();
    return;
  }
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

// Instant refresh when CLI writes a new call to the DB
EventsOn('calls:updated', () => {
  if (currentView === 'dashboard') load();
});
