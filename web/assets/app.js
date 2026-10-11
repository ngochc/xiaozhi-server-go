(function () {
  'use strict';

  const app = document.getElementById('app');
  const toastRoot = document.getElementById('toast-root');
  const modalRoot = document.getElementById('modal-root');
  const assetBase = window.__STATIC_WEB_BASE__ || '/';
  const apiBase = location.protocol === 'file:'
    ? (localStorage.getItem('xiaozhi_api_base') || 'http://localhost:8080')
    : location.origin;

  const state = {
    user: readUser(),
    branding: {
      name: 'Xiaozhi',
      title: 'Xiaozhi Admin Console',
      welcome: 'Welcome to Xiaozhi Admin Console',
      logoUrl: asset('logo.png'),
      homeLink: 'https://github.com/AnimeAIChat/xiaozhi-server-go/tree/main',
      copyright: 'Xiaozhi',
    },
    advancedTab: 'application',
  };

  const routeTitles = {
    login: 'Login',
    welcome: 'Welcome',
    agents: 'Agents',
    agentConfig: 'Configure Agent',
    devices: 'Manage Devices',
    history: 'Chat History',
    profile: 'Profile',
    dashboard: 'Dashboard',
    adminSettings: 'System Settings',
    providers: 'Model Providers',
    unbindDevice: 'Unbind Device',
    advancedConfig: 'Advanced Settings',
    diagnostics: 'Setup Wizard',
    whiteList: 'Whitelist',
  };

  function asset(path) {
    return `${assetBase}${path}`.replace(/\/{2,}/g, '/').replace(':/', '://');
  }

  function readUser() {
    try {
      return JSON.parse(localStorage.getItem('user') || 'null');
    } catch (_error) {
      return null;
    }
  }

  function saveUser(user) {
    state.user = user;
    localStorage.setItem('user', JSON.stringify(user));
  }

  function clearUser() {
    state.user = null;
    localStorage.removeItem('user');
  }

  function escapeHtml(value) {
    return String(value ?? '')
      .replace(/&/g, '&amp;')
      .replace(/</g, '&lt;')
      .replace(/>/g, '&gt;')
      .replace(/"/g, '&quot;')
      .replace(/'/g, '&#39;');
  }

  function prettyJson(value) {
    try {
      return JSON.stringify(value ?? {}, null, 2);
    } catch (_error) {
      return '{}';
    }
  }

  function parseJson(value, fallback = {}) {
    if (value == null || value === '') return fallback;
    if (typeof value === 'object') return value;
    try {
      return JSON.parse(value);
    } catch (_error) {
      return fallback;
    }
  }

  function byId(id) {
    return document.getElementById(id);
  }

  function qs(selector, root = document) {
    return root.querySelector(selector);
  }

  function qsa(selector, root = document) {
    return Array.from(root.querySelectorAll(selector));
  }

  function formatDate(value) {
    if (!value) return 'None';
    let date;
    if (typeof value === 'number') {
      date = new Date(value * 1000);
    } else {
      date = new Date(value);
    }
    if (Number.isNaN(date.getTime()) || date.getFullYear() < 2020) return 'N/A';
    return date.toLocaleString();
  }

  function formatPercent(value) {
    if (value == null || value === '') return '-';
    const number = Number(value);
    if (Number.isNaN(number)) return String(value);
    return `${Number(number.toFixed(2))}%`;
  }

  function normalizeList(payload) {
    if (Array.isArray(payload)) return payload;
    if (Array.isArray(payload?.data)) return payload.data;
    if (Array.isArray(payload?.data?.data)) return payload.data.data;
    if (Array.isArray(payload?.list)) return payload.list;
    return [];
  }

  function normalizeTablePayload(payload) {
    const data = normalizeList(payload);
    return {
      data,
      total: payload?.total ?? payload?.data?.total ?? data.length,
      success: payload?.success ?? payload?.status === 'ok' ?? true,
    };
  }

  function formValues(form) {
    const values = {};
    const data = new FormData(form);
    data.forEach((value, key) => {
      values[key] = value;
    });
    qsa('input[type="checkbox"]', form).forEach((input) => {
      values[input.name] = input.checked;
    });
    return values;
  }

  function toast(message, type = 'info') {
    const item = document.createElement('div');
    item.className = `toast ${type}`;
    item.textContent = message;
    toastRoot.appendChild(item);
    setTimeout(() => {
      item.remove();
    }, 3600);
  }

  function showError(error, fallback = 'Operation failed') {
    const message = error?.data?.message || error?.data?.error || error?.message || fallback;
    toast(message, 'error');
  }

  function openModal({ title, body, footer }) {
    modalRoot.innerHTML = `
      <div class="modal-backdrop" role="dialog" aria-modal="true">
        <div class="modal">
          <div class="modal-header">
            <h3>${escapeHtml(title)}</h3>
            <button class="btn ghost small" data-close-modal>Close</button>
          </div>
          <div class="modal-body">${body}</div>
          <div class="modal-footer">${footer || ''}</div>
        </div>
      </div>
    `;
    qsa('[data-close-modal]', modalRoot).forEach((button) => {
      button.addEventListener('click', closeModal);
    });
  }

  function closeModal() {
    modalRoot.innerHTML = '';
  }

  function confirmAction(title, content, onConfirm) {
    openModal({
      title,
      body: `<p>${escapeHtml(content)}</p>`,
      footer: `
        <button class="btn" data-close-modal>Cancel</button>
        <button class="btn danger" id="confirm-ok">Confirm</button>
      `,
    });
    byId('confirm-ok').addEventListener('click', async () => {
      try {
        await onConfirm();
        closeModal();
      } catch (error) {
        showError(error);
      }
    });
  }

  async function request(path, options = {}) {
    const url = path.startsWith('http') ? path : `${apiBase}${path}`;
    const headers = new Headers(options.headers || {});
    headers.set('Accept', headers.get('Accept') || 'application/json');
    if (state.user?.token) {
      headers.set('Authorization', `Bearer ${state.user.token}`);
    }

    const init = {
      method: options.method || 'GET',
      headers,
      credentials: 'same-origin',
    };

    if (options.body instanceof FormData) {
      init.body = options.body;
    } else if (options.body !== undefined) {
      headers.set('Content-Type', headers.get('Content-Type') || 'application/json');
      init.body = typeof options.body === 'string' ? options.body : JSON.stringify(options.body);
    }

    let response;
    try {
      response = await fetch(url, init);
    } catch (error) {
      const networkError = new Error(`Unable to connect to server: ${apiBase}`);
      networkError.cause = error;
      throw networkError;
    }

    const contentType = response.headers.get('content-type') || '';
    const payload = contentType.includes('application/json')
      ? await response.json().catch(() => ({}))
      : await response.text();

    if (!response.ok) {
      const error = new Error(payload?.message || payload?.error || response.statusText);
      error.status = response.status;
      error.data = payload;
      if (response.status === 401) {
        clearUser();
        navigate('/login');
      }
      throw error;
    }

    return payload;
  }

  function getRoutePath() {
    if (location.protocol === 'file:') {
      return (location.hash || '#/welcome').slice(1) || '/welcome';
    }
    return location.pathname === '/' ? '/welcome' : location.pathname;
  }

  function routeInfo(path = getRoutePath()) {
    const clean = path.replace(/\/+$/, '') || '/welcome';
    if (clean === '/login' || clean === '/console/login') return { key: 'login', public: true, path: '/login' };
    if (clean === '/welcome' || clean === '/') return { key: 'welcome', path: '/welcome' };
    if (clean === '/console/agents') return { key: 'agents', path: '/console/agents' };
    if (clean === '/console/settings') return { key: 'profile', path: '/console/settings' };
    let match = clean.match(/^\/console\/agents\/([^/]+)\/config$/);
    if (match) return { key: 'agentConfig', path: clean, params: { id: match[1] } };
    match = clean.match(/^\/console\/agents\/([^/]+)\/device$/);
    if (match) return { key: 'devices', path: clean, params: { id: match[1] } };
    match = clean.match(/^\/console\/agents\/([^/]+)\/history$/);
    if (match) return { key: 'history', path: clean, params: { id: match[1] } };
    if (clean === '/admin/dashboard') return { key: 'dashboard', path: clean };
    if (clean === '/admin/settings') return { key: 'adminSettings', path: clean };
    if (clean === '/admin/providers') return { key: 'providers', path: clean };
    if (clean === '/admin/unbindDevice') return { key: 'unbindDevice', path: clean };
    if (clean === '/admin/systemConfig') return { key: 'advancedConfig', path: clean };
    if (clean === '/admin/diagnostics') return { key: 'diagnostics', path: clean };
    if (clean === '/admin/whiteList') return { key: 'whiteList', path: clean };
    return { key: 'welcome', path: '/welcome' };
  }

  function navigate(path) {
    if (location.protocol === 'file:') {
      location.hash = path;
      render();
      return;
    }
    history.pushState({}, '', path);
    render();
  }

  function canSeeAdmin() {
    return state.user?.role === 'admin' || state.user?.role === 'observer';
  }

  function canWriteAdmin() {
    return state.user?.role === 'admin';
  }

  function shell(route) {
    const adminItems = canSeeAdmin()
      ? `
        <div class="nav-group">
          <div class="nav-title">Administration</div>
          ${navItem('/admin/dashboard', 'Dashboard', route.path)}
          ${navItem('/admin/settings', 'System Settings', route.path)}
          ${navItem('/admin/providers', 'Model Providers', route.path)}
          ${navItem('/admin/diagnostics', 'Setup Wizard', route.path)}
          ${navItem('/admin/unbindDevice', 'Unbind Device', route.path)}
          ${navItem('/admin/systemConfig', 'Advanced Settings', route.path)}
          ${navItem('/admin/whiteList', 'Whitelist', route.path)}
        </div>
      `
      : '';

    app.innerHTML = `
      <div class="layout">
        <aside class="sidebar" id="sidebar">
          <div class="brand">
            <img src="${escapeHtml(state.branding.logoUrl || asset('logo.png'))}" alt="">
            <span>${escapeHtml(state.branding.name || 'Xiaozhi')}</span>
          </div>
          <nav class="nav">
            <div class="nav-group">
              ${navItem('/welcome', 'Welcome', route.path)}
            </div>
            <div class="nav-group">
              <div class="nav-title">Console</div>
              ${navItem('/console/agents', 'Agents', route.path)}
              ${navItem('/console/settings', 'Profile', route.path)}
            </div>
            ${adminItems}
          </nav>
        </aside>
        <section class="main">
          <header class="topbar">
            <div class="topbar-left">
              <button class="btn ghost mobile-menu" id="mobile-menu">Menu</button>
              <h1 class="page-title" id="page-title">${escapeHtml(routeTitles[route.key] || '')}</h1>
            </div>
            <div class="topbar-right">
              <a href="${escapeHtml(state.branding.homeLink || '#')}" target="_blank" rel="noreferrer">GitHub</a>
              <div class="user-menu">
                <button class="avatar-button" id="avatar-button">
                  <span class="avatar">${escapeHtml((state.user?.username || 'U').slice(0, 1).toUpperCase())}</span>
                  <span>${escapeHtml(state.user?.username || 'User')}</span>
                </button>
                <div class="dropdown" id="user-dropdown">
                  <button data-link="/console/settings">Profile</button>
                  <button data-action="logout">Log Out</button>
                </div>
              </div>
            </div>
          </header>
          <main class="content" id="page"></main>
        </section>
      </div>
    `;

    byId('mobile-menu')?.addEventListener('click', () => {
      byId('sidebar')?.classList.toggle('open');
    });
    byId('avatar-button')?.addEventListener('click', (event) => {
      event.stopPropagation();
      byId('user-dropdown')?.classList.toggle('open');
    });
  }

  function navItem(path, label, currentPath) {
    const active = currentPath === path || (path !== '/welcome' && currentPath.startsWith(path));
    return `
      <a class="nav-item ${active ? 'active' : ''}" href="${path}" data-link="${path}">
        <span class="nav-icon">•</span>
        <span>${escapeHtml(label)}</span>
      </a>
    `;
  }

  function setTitle(title) {
    const fullTitle = title ? `${title} - ${state.branding.title}` : state.branding.title;
    document.title = fullTitle;
    const titleEl = byId('page-title');
    if (titleEl) titleEl.textContent = title;
  }

  function page() {
    return byId('page');
  }

  function pageHeader(title, subtitle = '', actions = '') {
    return `
      <div class="page-header">
        <div>
          <h2>${escapeHtml(title)}</h2>
          ${subtitle ? `<p>${escapeHtml(subtitle)}</p>` : ''}
        </div>
        <div class="toolbar">${actions}</div>
      </div>
    `;
  }

  function loadingCard(text = 'Loading...') {
    return `<div class="card"><div class="app-loading" style="min-height:180px"><div class="loading-mark"></div><div>${escapeHtml(text)}</div></div></div>`;
  }

  function emptyState(text, action = '') {
    return `<div class="empty">${escapeHtml(text)}${action ? `<div style="margin-top:16px">${action}</div>` : ''}</div>`;
  }

  async function render() {
    const route = routeInfo();
    state.user = readUser();

    if (route.key !== 'login' && !state.user) {
      renderLogin();
      return;
    }

    if (route.key === 'login') {
      renderLogin();
      return;
    }

    shell(route);
    setTitle(routeTitles[route.key] || 'Welcome');

    const renderers = {
      welcome: renderWelcome,
      agents: renderAgents,
      agentConfig: renderAgentConfig,
      devices: renderDevices,
      history: renderHistory,
      profile: renderProfile,
      dashboard: renderDashboard,
      adminSettings: renderAdminSettings,
      providers: renderProviders,
      unbindDevice: renderUnbindDevice,
      advancedConfig: renderAdvancedConfig,
      diagnostics: renderDiagnostics,
      whiteList: renderWhiteList,
    };

    try {
      await (renderers[route.key] || renderWelcome)(route);
    } catch (error) {
      page().innerHTML = `
        ${pageHeader(routeTitles[route.key] || 'Page', '')}
        <div class="alert error">${escapeHtml(error.message || 'Failed to load page')}</div>
      `;
    }
  }

  function renderLogin() {
    setTitle('Login');
    app.innerHTML = `
      <div class="login-page">
        <main class="login-main">
          <form class="login-card" id="login-form">
            <div class="login-brand">
              <img src="${escapeHtml(state.branding.logoUrl || asset('logo.png'))}" alt="">
              <h1 class="login-title">${escapeHtml(state.branding.title)}</h1>
              <div class="login-subtitle">Account Login</div>
            </div>
            <div class="form">
              <div class="field">
                <label for="login-username">Username</label>
                <input id="login-username" name="username" autocomplete="username" placeholder="admin" required>
              </div>
              <div class="field">
                <label for="login-password">Password</label>
                <input id="login-password" name="password" type="password" autocomplete="current-password" placeholder="Temporary password from startup log" required>
              </div>
              <label class="field checkbox">
                <input type="checkbox" name="autoLogin" checked>
                <span>Remember Me</span>
              </label>
              <button class="btn primary" type="submit">Sign In</button>
              <div class="subtle">Default user admin, default password 123456. Please change it after login.</div>
            </div>
          </form>
        </main>
        <footer class="footer-note">© ${escapeHtml(state.branding.copyright || 'Xiaozhi')}</footer>
      </div>
    `;

    byId('login-form').addEventListener('submit', async (event) => {
      event.preventDefault();
      const values = formValues(event.currentTarget);
      try {
        const result = await request('/api/user/login', {
          method: 'POST',
          body: {
            username: values.username,
            password: values.password,
          },
        });
        if (result?.status === 'ok' && result?.data?.token) {
          saveUser(result.data);
          toast('Login successful', 'success');
          navigate('/welcome');
          return;
        }
        toast(result?.message || 'Login failed', 'error');
      } catch (error) {
        showError(error, 'Login failed');
      }
    });
  }

  async function renderDiagnostics() {
    page().innerHTML = `
      ${pageHeader('Setup Wizard', 'Run checks to verify model providers and server addresses. Secret keys and tokens are not displayed in results.')}
      <div class="card">
        <h3>1. Configure Model Providers</h3>
        <p class="muted">Configure and select ASR, LLM, and TTS providers. Credentials are stored only in your local configuration.</p>
        <button class="btn" data-link="/admin/providers">Open Model Providers</button>
      </div>
      <div class="card">
        <h3>2. Configure Device Access Address</h3>
        <p class="muted">Set the WebSocket address accessible by your devices in System Settings. Local devices cannot reach localhost.</p>
        <button class="btn" data-link="/admin/systemConfig">Open System Settings</button>
      </div>
      <div class="card">
        <div class="toolbar" style="justify-content:space-between;align-items:center">
          <div>
            <h3 style="margin:0">3. Run Connectivity Checks</h3>
            <p class="muted" style="margin:6px 0 0">Verify WebSocket, HTTP server, and active ASR, LLM, and TTS configurations.</p>
          </div>
          <button class="btn primary" id="run-diagnostics">Start Check</button>
        </div>
        <div id="diagnostics-results" style="margin-top:16px"></div>
      </div>
      <div class="card">
        <div class="toolbar" style="justify-content:space-between;align-items:center;gap:16px">
          <div>
            <h3 style="margin:0">4. Create Setup Assistant</h3>
            <p class="muted" style="margin:6px 0 0">Create the default Setup Assistant once ASR, LLM, and TTS are configured. Unbound devices will automatically bind to it.</p>
          </div>
          <button class="btn primary" id="activate-onboarding" disabled>Checking configuration...</button>
        </div>
        <div id="onboarding-status" class="provider-test-result" aria-live="polite" style="margin-top:12px"></div>
      </div>
    `;

    const resultHost = byId('diagnostics-results');
    const onboardingHost = byId('onboarding-status');
    const onboardingButton = byId('activate-onboarding');
    const refreshOnboarding = async () => {
      try {
        const payload = await request('/api/admin/onboarding');
        const status = payload.data || {};
        const issues = Array.isArray(status.issues) ? status.issues : [];
        const ready = status.ready === true;
        const exists = status.exists === true;
        onboardingButton.disabled = !ready || !canWriteAdmin();
        onboardingButton.textContent = exists ? 'Sync Setup Assistant' : 'Create Setup Assistant';
        if (ready) {
          const assistantState = exists
            ? `Setup Assistant already exists (ID ${escapeHtml(status.agentId)}). You can sync current Provider selections.`
            : 'ASR, LLM, and TTS configurations are ready to create the Setup Assistant.';
          onboardingHost.innerHTML = `<div class="provider-test-message success">${assistantState} ${Number(status.pendingDevices || 0)}  unbound device(s) will be managed after creation.</div>`;
        } else {
          onboardingHost.innerHTML = `<div class="provider-test-message error">Please resolve before creation: ${escapeHtml(issues.join('；') || 'Provider configuration not ready')}</div>`;
        }
      } catch (error) {
        onboardingButton.disabled = true;
        onboardingButton.textContent = 'Unable to check config';
        onboardingHost.innerHTML = `<div class="provider-test-message error">${escapeHtml(error?.data?.message || error?.message || 'Unable to read Setup Assistant status')}</div>`;
      }
    };
    onboardingButton.addEventListener('click', () => {
      confirmAction('Create Setup Assistant', 'This will create the Setup Assistant and bind unbound devices. Continue?', async () => {
        onboardingButton.disabled = true;
        onboardingButton.textContent = 'Creating...';
        try {
          const result = await request('/api/admin/onboarding', { method: 'POST' });
          toast(result.message || 'Setup Assistant created', 'success');
          await refreshOnboarding();
        } catch (error) {
          showError(error, 'Failed to create Setup Assistant');
          await refreshOnboarding();
        }
      });
    });
    refreshOnboarding();
    const renderResults = (results) => {
      if (!results?.length) {
        resultHost.innerHTML = emptyState('No check results');
        return;
      }
      resultHost.innerHTML = `<div class="table-wrap"><table><thead><tr><th>Item</th><th>Result</th><th>Description</th><th>Duration</th></tr></thead><tbody>${results.map((result) => {
        const status = result.status === 'ok' ? 'Pass' : result.status === 'ready' ? 'Configured' : 'Needs Action';
        const className = result.status === 'ok' ? 'green' : result.status === 'ready' ? 'blue' : 'red';
        return `<tr><td>${escapeHtml(result.name)}</td><td><span class="tag ${className}">${status}</span></td><td>${escapeHtml(result.message)}</td><td>${result.latencyMs == null ? '-' : `${escapeHtml(result.latencyMs)} ms`}</td></tr>`;
      }).join('')}</tbody></table></div>`;
    };

    byId('run-diagnostics').addEventListener('click', async () => {
      const button = byId('run-diagnostics');
      button.disabled = true;
      button.textContent = 'Checking...';
      resultHost.innerHTML = loadingCard('Checking configuration and endpoints...');
      try {
        const payload = await request('/api/admin/config/diagnostics', { method: 'POST', body: {} });
        renderResults(payload.data || []);
      } catch (error) {
        resultHost.innerHTML = `<div class="alert error">${escapeHtml(error.message || 'Check failed')}</div>`;
      } finally {
        button.disabled = false;
        button.textContent = 'Recheck';
      }
    });
  }

  async function renderWelcome() {
    page().innerHTML = `
      ${pageHeader('Welcome', state.branding.welcome)}
      <div class="card welcome-host" id="welcome-host">${loadingCard('Loading welcome page...')}</div>
    `;
    const host = byId('welcome-host');
    try {
      const response = await fetch(asset('welcome.html'), { cache: 'no-cache' });
      const text = await response.text();
      const styles = Array.from(text.matchAll(/<style[^>]*>([\s\S]*?)<\/style>/gi))
        .map((match) => `<style>${match[1]}</style>`)
        .join('');
      const bodyMatch = text.match(/<body[^>]*>([\s\S]*?)<\/body>/i);
      host.innerHTML = `${styles}${bodyMatch ? bodyMatch[1] : text}`;
    } catch (_error) {
      host.innerHTML = `
        <div class="empty">
          <h3>${escapeHtml(state.branding.welcome)}</h3>
          <p>Welcome to the open-source admin console.</p>
        </div>
      `;
    }
  }

  async function renderDashboard() {
    page().innerHTML = `${pageHeader('Dashboard', 'System Overview')}${loadingCard()}`;
    const data = await request('/api/user/summary').then((res) => res.data || {});
    const stats = [
      ['Total Users', data.totle_users ?? '-'],
      ['Total Devices', data.totle_devices ?? '-'],
      ['Total Agents', data.totle_agents ?? '-'],
      ['Online Users', data.online_users ?? '-'],
      ['Active Devices', data.session_devices ?? '-'],
      ['Memory Usage', formatPercent(data.system_memory_use)],
      ['CPU Usage', formatPercent(data.system_cpu_use)],
    ];
    page().innerHTML = `
      ${pageHeader('Dashboard', 'System Overview')}
      <div class="grid stats-grid">
        ${stats.map(([title, value]) => `
          <div class="card">
            <div class="muted">${escapeHtml(title)}</div>
            <div class="stat-value">${escapeHtml(value)}</div>
          </div>
        `).join('')}
      </div>
    `;
  }

  async function getProvidersByType(type) {
    try {
      const payload = await request(`/api/user/providers/${type}`);
      return parseProviderMap(payload, type);
    } catch (_error) {
      return [];
    }
  }

  function parseProviderMap(payload, type) {
    const source = payload?.data || {};
    return Object.entries(source).map(([name, raw]) => {
      const config = parseJson(raw, {});
      return {
        id: `${type}:${name}`,
        name,
        type,
        status: 'active',
        config,
      };
    });
  }

  async function getModelProviders() {
    const groups = await Promise.all(['ASR', 'TTS', 'LLM', 'VLLLM'].map(getProvidersByType));
    return groups.flat();
  }

  function providerDisplay(provider) {
    const config = provider.config || {};
    if (provider.type === 'LLM' || provider.type === 'VLLLM') {
      return config.model_name || config.model || config.url || '';
    }
    if (provider.type === 'TTS') {
      return config.voice || config.type || '';
    }
    return config.type || config.url || config.output_dir || '';
  }

  function collectVoices(ttsProviders) {
    const voices = [];
    const seen = new Set();
    ttsProviders.forEach((provider) => {
      const config = provider.config || {};
      const raw = config.supported_voices || config.surported_voices || config.supportedVoices || [];
      if (Array.isArray(raw)) {
        raw.forEach((item) => {
          const value = typeof item === 'string' ? item : item?.name;
          if (!value || seen.has(value)) return;
          seen.add(value);
          voices.push({
            value,
            label: typeof item === 'string'
              ? item
              : (item.display_name_zh || item.display_name_cn || item.display_name || item.displayName || item.name),
            language: typeof item === 'object' && item.language ? item.language : 'Standard',
          });
        });
      }
      if (config.voice && !seen.has(config.voice)) {
        seen.add(config.voice);
        voices.push({ value: config.voice, label: config.voice, language: 'Standard' });
      }
    });
    return voices;
  }

  function voiceLabel(voices, value) {
    return voices.find((voice) => voice.value === value)?.label || value || '';
  }

  async function renderAgents() {
    page().innerHTML = `
      ${pageHeader('Agents', 'Create the Setup Assistant in Setup Wizard; new devices will bind to it automatically', `
        <input id="agent-search" placeholder="Search agents" style="min-height:32px;padding:6px 10px;border:1px solid var(--line);border-radius:6px">
        <button class="btn primary" id="create-agent">New Agent</button>
        <button class="btn" id="agent-add-device">Add Device</button>
      `)}
      <div id="agents-body">${loadingCard('Loading agents...')}</div>
    `;

    byId('agent-add-device').addEventListener('click', () => openBindDeviceModal());
    byId('create-agent').addEventListener('click', () => openCreateAgentModal());

    const [agentPayload, ttsProviders] = await Promise.all([
      request('/api/user/agent/list'),
      getProvidersByType('TTS'),
    ]);
    const voices = collectVoices(ttsProviders);
    const agents = normalizeList(agentPayload);

    const draw = () => {
      const keyword = byId('agent-search')?.value?.trim().toLowerCase() || '';
      const filtered = agents.filter((agent) => !keyword || String(agent.name || '').toLowerCase().includes(keyword));
      byId('agents-body').innerHTML = filtered.length
        ? `<div class="grid agent-grid">${filtered.map((agent) => agentCard(agent, voices)).join('')}</div>`
        : emptyState('No agents found', '<button class="btn primary" id="create-first-agent">Create First Agent</button>');
      byId('create-first-agent')?.addEventListener('click', openCreateAgentModal);
    };

    byId('agent-search').addEventListener('input', draw);
    draw();
  }

  function agentCard(agent, voices) {
    const deviceCount = Array.isArray(agent.deviceIDs) ? agent.deviceIDs.length : 0;
    return `
      <div class="card">
        <div class="card-title">
          <h3>${escapeHtml(agent.name || 'Unnamed Agent')}</h3>
          ${agent.isOnboarding ? '<span class="tag blue">Setup Assistant</span>' : `<button class="btn ghost danger small" data-delete-agent="${escapeHtml(agent.id)}">Delete</button>`}
        </div>
        <div class="chip-row">
          <span class="tag blue">ID ${escapeHtml(agent.id)}</span>
          <span class="tag">${escapeHtml(agent.LLM || '-')}</span>
        </div>
        <div class="divider"></div>
        <div class="form">
          <div><span class="muted">Voice: </span>${escapeHtml(voiceLabel(voices, agent.voice) || '-')}</div>
          ${agent.isOnboarding ? '<div><span class="muted">Role: </span>Default onboarding assistant for new devices</div>' : ''}
          <div><span class="muted">Last Active: </span>${escapeHtml(formatDate(agent.lastConversationAt))}</div>
          <div><span class="muted">Devices: </span>${deviceCount}</div>
        </div>
        <div class="divider"></div>
        <div class="inline-actions">
          <button class="btn" data-link="/console/agents/${escapeHtml(agent.id)}/config">Configure Role</button>
          <button class="btn" data-link="/console/agents/${escapeHtml(agent.id)}/history">Chat History</button>
          <button class="btn" data-link="/console/agents/${escapeHtml(agent.id)}/device">Manage Devices</button>
        </div>
      </div>
    `;
  }

  async function openCreateAgentModal() {
    openModal({
      title: 'New Agent',
      body: `
        <form class="form" id="create-agent-form">
          <div class="field">
            <label>Agent Name</label>
            <input name="name" placeholder="Enter agent name" required>
          </div>
        </form>
      `,
      footer: `
        <button class="btn" data-close-modal>Cancel</button>
        <button class="btn primary" id="create-agent-submit">Create</button>
      `,
    });
    byId('create-agent-submit').addEventListener('click', async () => {
      const form = byId('create-agent-form');
      if (!form.reportValidity()) return;
      const values = formValues(form);
      try {
        const [ttsProviders, llmProviders] = await Promise.all([
          getProvidersByType('TTS'),
          getProvidersByType('LLM'),
        ]);
        const voices = collectVoices(ttsProviders);
        const defaultVoice = voices[0]?.value || 'zh_female_wanwanxiaohe_moon_bigtts';
        const defaultVoiceName = voices[0]?.label || defaultVoice;
        const defaultLLM = llmProviders[0]?.name || 'Qwen';
        await request('/api/user/agent/create', {
          method: 'POST',
          body: {
            prompt: '',
            name: values.name,
            LLM: defaultLLM,
            language: 'Standard',
            voice: defaultVoice,
            voiceName: defaultVoiceName,
            asrSpeed: 2,
            speakSpeed: 2,
            tone: 50,
          },
        });
        toast('Agent created successfully', 'success');
        closeModal();
        render();
      } catch (error) {
        showError(error, 'Failed to create agent');
      }
    });
  }

  async function openBindDeviceModal(selectedAgentId = '') {
    const agents = normalizeList(await request('/api/user/agent/list'));
    if (!agents.length) {
      toast('Please create an agent first', 'warning');
      return;
    }
    openModal({
      title: 'Bind Device',
      body: `
        <form class="form" id="bind-device-form">
          <div class="field">
            <label>Agent</label>
            <select name="agentID" required>
              ${agents.map((agent) => `<option value="${escapeHtml(agent.id)}" ${String(agent.id) === String(selectedAgentId) ? 'selected' : ''}>${escapeHtml(agent.name || `Agent ${agent.id}`)}</option>`).join('')}
            </select>
          </div>
          <div class="field">
            <label>Device MAC Address</label>
            <input name="mac" placeholder="e.g. 94:A9:90:31:E2:E4" autocomplete="off" spellcheck="false" required>
          </div>
          <p class="muted">Device must have contacted this server's OTA endpoint. Supports colon, hyphen, or unformatted MAC.</p>
        </form>
      `,
      footer: `<button class="btn" data-close-modal>Cancel</button><button class="btn primary" id="bind-device-submit">Bind</button>`,
    });
    byId('bind-device-submit').addEventListener('click', async () => {
      const form = byId('bind-device-form');
      if (!form.reportValidity()) return;
      try {
        await request('/api/user/device/bind', { method: 'POST', body: formValues(form) });
        closeModal();
        toast('Device bound successfully', 'success');
        render();
      } catch (error) {
        showError(error, 'Binding failed: ensure device connected and not bound elsewhere');
      }
    });
  }

  async function renderAgentConfig(route) {
    const id = route.params.id;
    page().innerHTML = `
      ${pageHeader('Configure Role', `Agent ID: ${id}`, `<button class="btn" data-link="/console/agents">Back</button>`)}
      <div id="agent-config-body">${loadingCard()}</div>
    `;
    const [agentPayload, ttsProviders, llmProviders, templates] = await Promise.all([
      request(`/api/user/agent/${id}`),
      getProvidersByType('TTS'),
      getProvidersByType('LLM'),
      fetch(asset('role_template.json'), { cache: 'no-cache' }).then((res) => res.json()).catch(() => []),
    ]);
    const agent = agentPayload.data || {};
    const voices = collectVoices(ttsProviders);
    if (agent.voice && !voices.some((voice) => voice.value === agent.voice)) {
      voices.unshift({ value: agent.voice, label: agent.voiceName || agent.voice, language: agent.language || 'Standard' });
    }
    const languages = Array.from(new Set(voices.map((voice) => voice.language || 'Standard')));

    byId('agent-config-body').innerHTML = `
      <form class="form" id="agent-config-form">
        <div class="card">
          <div class="card-title"><h3>Role Templates</h3></div>
          <div class="chip-row">
            ${templates.map((tpl, index) => `
              <button type="button" class="btn small" data-template-index="${index}">${escapeHtml(tpl.name)}</button>
            `).join('') || '<span class="muted">No templates available</span>'}
          </div>
        </div>
        <div class="card">
          <div class="card-title"><h3>Basic Settings</h3></div>
          <div class="form-grid">
            <div class="field">
              <label>Assistant Name</label>
              <input name="assistantName" value="${escapeHtml(agent.name)}" required>
            </div>
            <div class="field">
              <label>Conversation Language</label>
              <select name="dialogLanguage" id="dialog-language">
                ${languages.map((language) => `<option value="${escapeHtml(language)}" ${language === agent.language ? 'selected' : ''}>${escapeHtml(language)}</option>`).join('')}
              </select>
            </div>
            <div class="field">
              <label>Voice</label>
              <select name="voiceType" id="voice-type"></select>
            </div>
            <div class="field">
              <label>Language Model (LLM)</label>
              <select name="languageModel" required>
                ${llmProviders.map((provider) => `
                  <option value="${escapeHtml(provider.name)}" ${provider.name === agent.LLM ? 'selected' : ''}>
                    ${escapeHtml(provider.name)}${providerDisplay(provider) ? ` (${escapeHtml(providerDisplay(provider))})` : ''}
                  </option>
                `).join('')}
              </select>
            </div>
          </div>
        </div>
        <div class="card">
          <div class="card-title"><h3>Role Introduction</h3></div>
          <div class="field">
            <label>System Prompt</label>
            <textarea name="roleDescription" rows="8" required>${escapeHtml(agent.prompt || '')}</textarea>
          </div>
        </div>
        <div class="card">
          <div class="card-title"><h3>Advanced Settings</h3></div>
          <div class="form-grid">
            <div class="field">
              <label>Speech Recognition Speed</label>
              <select name="recognitionSpeed">
                <option value="1" ${agent.asrSpeed === 1 ? 'selected' : ''}>Patient</option>
                <option value="2" ${agent.asrSpeed !== 1 && agent.asrSpeed !== 3 ? 'selected' : ''}>Normal</option>
                <option value="3" ${agent.asrSpeed === 3 ? 'selected' : ''}>Fast</option>
              </select>
            </div>
            <div class="field">
              <label>Speech Rate</label>
              <select name="speechSpeed">
                <option value="1" ${agent.speakSpeed === 1 ? 'selected' : ''}>Slow</option>
                <option value="2" ${agent.speakSpeed !== 1 && agent.speakSpeed !== 3 ? 'selected' : ''}>Normal</option>
                <option value="3" ${agent.speakSpeed === 3 ? 'selected' : ''}>Fast</option>
              </select>
            </div>
            <div class="field">
              <label>Voice Pitch</label>
              <input name="pitchValue" type="range" min="0" max="100" value="${escapeHtml(agent.tone ?? 50)}">
            </div>
          </div>
        </div>
        <div class="inline-actions">
          <button type="button" class="btn" id="agent-reset">Reset</button>
          <button type="submit" class="btn primary">Save</button>
        </div>
      </form>
    `;

    const syncVoiceOptions = () => {
      const selectedLanguage = byId('dialog-language').value;
      const filtered = voices.filter((voice) => (voice.language || 'Standard') === selectedLanguage);
      const options = (filtered.length ? filtered : voices).map((voice) => `
        <option value="${escapeHtml(voice.value)}" ${voice.value === agent.voice ? 'selected' : ''}>${escapeHtml(voice.label)}</option>
      `).join('');
      byId('voice-type').innerHTML = options || `<option value="${escapeHtml(agent.voice || '')}">${escapeHtml(agent.voiceName || agent.voice || 'Default Voice')}</option>`;
    };
    syncVoiceOptions();
    byId('dialog-language').addEventListener('change', syncVoiceOptions);

    qsa('[data-template-index]').forEach((button) => {
      button.addEventListener('click', () => {
        const tpl = templates[Number(button.dataset.templateIndex)];
        const name = qs('[name="assistantName"]').value || '';
        qs('[name="roleDescription"]').value = String(tpl.prompt || '').replace(/\{\{\s*assistant_name\s*\}\}/g, name);
      });
    });

    byId('agent-reset').addEventListener('click', () => renderAgentConfig(route));
    byId('agent-config-form').addEventListener('submit', async (event) => {
      event.preventDefault();
      const values = formValues(event.currentTarget);
      const selectedVoice = voices.find((voice) => voice.value === values.voiceType);
      try {
        await request(`/api/user/agent/${id}`, {
          method: 'PUT',
          body: {
            prompt: values.roleDescription,
            name: values.assistantName,
            LLM: values.languageModel,
            language: values.dialogLanguage,
            voice: values.voiceType,
            voiceName: selectedVoice?.label || values.voiceType,
            asrSpeed: Number(values.recognitionSpeed),
            speakSpeed: Number(values.speechSpeed),
            tone: Number(values.pitchValue),
          },
        });
        toast('Saved successfully', 'success');
      } catch (error) {
        showError(error, 'Failed to save');
      }
    });
  }

  async function renderDevices(route) {
    const id = route.params.id;
    page().innerHTML = `
      ${pageHeader('Manage Devices', `Agent ID: ${id}`, `
        <button class="btn" data-link="/console/agents">Back</button>
        <button class="btn primary" id="add-device">Add Device</button>
      `)}
      <div id="devices-body">${loadingCard()}</div>
    `;
    byId('add-device').addEventListener('click', () => openBindDeviceModal(id));
    const payload = await request(`/api/user/device/list/${id}`);
    const devices = normalizeList(payload);
    byId('devices-body').innerHTML = devices.length
      ? `<div class="grid device-grid">${devices.map(deviceCard).join('')}</div>`
      : emptyState('No devices found');
  }

  function deviceCard(device) {
    const deviceId = device.deviceId || device.DeviceID || '';
	    const macAddress = device.macAddress || device.MACAddress || deviceId;
    return `
      <div class="card">
        <div class="card-title">
          <h3>${escapeHtml(device.name || 'Unnamed Device')}</h3>
          <button class="btn ghost danger small" data-unbind-device="${escapeHtml(deviceId)}">Unbind</button>
        </div>
        <div class="form">
          <div><span class="muted">MAC Address: </span>${escapeHtml(maskDeviceId(macAddress))}</div>
          <div><span class="muted">Firmware: </span>${escapeHtml(device.version || '-')}</div>
          <div><span class="muted">Last Active: </span>${escapeHtml(formatDate(device.lastActiveTimeV2 || device.lastActiveTime))}</div>
          <div><span class="muted">Status: </span>${device.mode === 'ban' ? '<span class="tag red">Disabled</span>' : '<span class="tag green">Active</span>'}</div>
          <div><span class="muted">OTA Upgrade: </span>${device.ota ? '<span class="tag green">Enabled</span>' : '<span class="tag">Disabled</span>'}</div>
        </div>
        <div class="divider"></div>
        <button class="btn small" data-copy="${escapeHtml(macAddress)}">Copy MAC</button>
        <button class="btn small" data-toggle-device="${escapeHtml(deviceId)}" data-disabled="${device.mode === 'ban' ? 'true' : 'false'}">${device.mode === 'ban' ? 'Enable Device' : 'Disable Device'}</button>
        <button class="btn ghost small" data-clear-device-memory="${escapeHtml(deviceId)}">Clear Short-term Memory</button>
      </div>
    `;
  }

  function maskDeviceId(value) {
    const parts = String(value || '').split(':');
    if (parts.length === 6) return `${parts[0]}:${parts[1]}:**:**:${parts[4]}:${parts[5]}`;
    return value || '-';
  }

  async function renderHistory(route) {
    const id = route.params.id;
    page().innerHTML = `
      ${pageHeader('Chat History', `Agent ID: ${id}`, `<button class="btn" data-link="/console/agents">Back</button>`)}
      <div class="history-layout">
        <div class="conversation-list" id="conversation-list">${loadingCard()}</div>
        <div class="message-panel" id="message-panel">
          <div class="empty">Select a conversation</div>
        </div>
      </div>
    `;
    const payload = await request(`/api/user/agent/history_dialog_list/${id}`, { method: 'POST' });
    const conversations = normalizeList(payload);
    const list = byId('conversation-list');
    list.innerHTML = conversations.length
      ? conversations.map((item) => `
        <button class="conversation-item" data-conversation="${escapeHtml(item.id)}">
          <strong>${escapeHtml(formatDate(item.createdAt))}</strong>
          <div class="subtle">ID ${escapeHtml(item.id)} · ${escapeHtml(item.conversationId || '')}</div>
        </button>
      `).join('')
      : emptyState('No chat history');
  }

  async function selectConversation(id) {
    qsa('.conversation-item').forEach((button) => {
      button.classList.toggle('active', button.dataset.conversation === String(id));
    });
    const panel = byId('message-panel');
    panel.innerHTML = loadingCard('Loading conversation details...');
    const payload = await request(`/api/user/agent/history_dialog/${id}`);
    const dialog = payload?.data || {};
    const messages = parseJson(dialog.dialog, []);
    panel.innerHTML = `
      <div class="message-header">
        Conversation - ${escapeHtml(formatDate(dialog.createdAt))}
        <button class="btn danger small" style="float:right" data-delete-dialog="${escapeHtml(id)}">Delete Conversation</button>
      </div>
      <div class="message-list">
        ${Array.isArray(messages) && messages.length
          ? messages.map((message, index) => `
            <div class="message ${escapeHtml(message.role || 'assistant')}">
              <div class="subtle">${escapeHtml(message.role || `Message ${index + 1}`)}</div>
              ${escapeHtml(message.content || '')}
            </div>
          `).join('')
          : emptyState('No messages')}
      </div>
    `;
  }

  async function renderProfile() {
    page().innerHTML = `${pageHeader('Profile', 'Manage user profile and password')}${loadingCard()}`;
    const payload = await request('/api/user/profile');
    const user = payload.data || {};
    page().innerHTML = `
      ${pageHeader('Profile', 'Manage user profile and password')}
      <div class="card">
        <form class="form" id="profile-form">
          <div class="form-grid">
            <div class="field">
              <label>Username</label>
              <input value="${escapeHtml(user.username)}" disabled>
            </div>
            <div class="field">
              <label>Nickname</label>
              <input name="nickname" value="${escapeHtml(user.nickname || user.username || '')}">
            </div>
            <div class="field">
              <label>Email</label>
              <input name="email" type="email" value="${escapeHtml(user.email || '')}">
            </div>
            <div class="field">
              <label>Avatar URL</label>
              <input name="head_img" value="${escapeHtml(user.head_img || '')}">
            </div>
          </div>
          <div class="inline-actions">
            <button class="btn primary" type="submit">Save Profile</button>
            <button class="btn" type="button" id="change-password">Change Password</button>
          </div>
        </form>
      </div>
    `;
    byId('profile-form').addEventListener('submit', async (event) => {
      event.preventDefault();
      try {
        await request('/api/user/profile', { method: 'PUT', body: formValues(event.currentTarget) });
        toast('Saved successfully', 'success');
      } catch (error) {
        showError(error, 'Failed to save');
      }
    });
    byId('change-password').addEventListener('click', openPasswordModal);
  }

  function openPasswordModal() {
    openModal({
      title: 'Change Password',
      body: `
        <form class="form" id="password-form">
          <div class="field"><label>Old Password</label><input name="old_password" type="password" required></div>
          <div class="field"><label>New Password</label><input name="new_password" type="password" minlength="6" required></div>
          <div class="field"><label>Confirm New Password</label><input name="confirm_password" type="password" minlength="6" required></div>
        </form>
      `,
      footer: `<button class="btn" data-close-modal>Cancel</button><button class="btn primary" id="password-submit">Save</button>`,
    });
    byId('password-submit').addEventListener('click', async () => {
      const form = byId('password-form');
      if (!form.reportValidity()) return;
      const values = formValues(form);
      if (values.new_password !== values.confirm_password) {
        toast('New passwords do not match', 'warning');
        return;
      }
      try {
        await request('/api/user/change-password', {
          method: 'POST',
          body: {
            old_password: values.old_password,
            new_password: values.new_password,
          },
        });
        toast('Password changed successfully', 'success');
        closeModal();
      } catch (error) {
        showError(error, 'Failed to change password');
      }
    });
  }

  async function renderAdminSettings() {
    page().innerHTML = `${pageHeader('System Settings', 'Model selection and global prompt')}${loadingCard()}`;
    const payload = await request('/api/admin/system');
    const config = payload.data || {};
    page().innerHTML = `
      ${pageHeader('System Settings', 'Model selection and global prompt')}
      <div class="card">
        <form class="form" id="admin-settings-form">
          <div class="form-grid">
            ${selectField('selectedASR', 'ASR Service', config.selectedASR, config.asrList || [])}
            ${selectField('selectedTTS', 'TTS Service', config.selectedTTS, config.ttsList || [])}
            ${selectField('selectedLLM', 'LLM Service', config.selectedLLM, config.llmList || [])}
            ${selectField('selectedVLLLM', 'VLLM Service', config.selectedVLLLM, config.vllmList || [])}
          </div>
          <div class="field">
            <label>Prompt Settings</label>
            <textarea name="prompt" rows="10">${escapeHtml(config.prompt || '')}</textarea>
          </div>
          <div class="field">
            <label>Quick Reply Words</label>
            <div class="chip-row" id="quick-reply-list">
              ${(config.quickReplyWords || []).map((word, index) => quickReplyTag(word, index)).join('')}
            </div>
            <div class="input-with-button">
              <input id="quick-reply-input" placeholder="Enter new quick reply word">
              <button type="button" class="btn" id="quick-reply-add">Add</button>
            </div>
          </div>
          <div class="inline-actions">
            <button type="button" class="btn" id="settings-reset">Reset</button>
            <button type="submit" class="btn primary" ${canWriteAdmin() ? '' : 'disabled'}>Save Settings</button>
          </div>
        </form>
      </div>
    `;
    let quickReplies = [...(config.quickReplyWords || [])];
    const drawQuickReplies = () => {
      byId('quick-reply-list').innerHTML = quickReplies.map((word, index) => quickReplyTag(word, index)).join('');
    };
    byId('quick-reply-add').addEventListener('click', () => {
      const input = byId('quick-reply-input');
      const value = input.value.trim();
      if (!value || quickReplies.includes(value)) return;
      quickReplies.push(value);
      input.value = '';
      drawQuickReplies();
    });
    byId('quick-reply-list').addEventListener('click', (event) => {
      const target = event.target.closest('[data-remove-quick]');
      if (!target) return;
      event.stopPropagation();
      quickReplies.splice(Number(target.dataset.removeQuick), 1);
      drawQuickReplies();
    });
    byId('settings-reset').addEventListener('click', renderAdminSettings);
    byId('admin-settings-form').addEventListener('submit', async (event) => {
      event.preventDefault();
      const values = formValues(event.currentTarget);
      const next = {
        selectedASR: values.selectedASR,
        selectedTTS: values.selectedTTS,
        selectedLLM: values.selectedLLM,
        selectedVLLLM: values.selectedVLLLM,
        prompt: values.prompt,
        quickReplyWords: quickReplies,
      };
      try {
        await request('/api/admin/system', { method: 'POST', body: { data: JSON.stringify(next) } });
        toast('Configuration saved successfully', 'success');
      } catch (error) {
        showError(error, 'Failed to save');
      }
    });
  }

  function selectField(name, label, value, options) {
    return `
      <div class="field">
        <label>${escapeHtml(label)}</label>
        <select name="${escapeHtml(name)}">
          <option value="">Please select</option>
          ${options.map((option) => `<option value="${escapeHtml(option)}" ${option === value ? 'selected' : ''}>${escapeHtml(option)}</option>`).join('')}
        </select>
      </div>
    `;
  }

  function quickReplyTag(word, index) {
    return `<span class="tag blue">${escapeHtml(word)} <button type="button" class="btn link" data-remove-quick="${index}">×</button></span>`;
  }

  async function renderProviders() {
    page().innerHTML = `
      ${pageHeader('Model Providers', 'Manage ASR, TTS, LLM, and VLLM services', `
        <input id="provider-search" placeholder="Search providers" style="min-height:32px;padding:6px 10px;border:1px solid var(--line);border-radius:6px">
        <select id="provider-filter" style="min-height:32px;padding:6px 10px;border:1px solid var(--line);border-radius:6px">
          <option value="all">All Types</option>
          <option value="ASR">ASR</option>
          <option value="TTS">TTS</option>
          <option value="LLM">LLM</option>
          <option value="VLLLM">VLLLM</option>
        </select>
        <button class="btn primary" id="provider-add">Add Provider</button>
      `)}
      <div id="providers-body">${loadingCard()}</div>
    `;
    byId('provider-add').addEventListener('click', () => openProviderModal());
    const providers = await getModelProviders();
    const draw = () => {
      const keyword = byId('provider-search').value.trim().toLowerCase();
      const filter = byId('provider-filter').value;
      const filtered = providers.filter((provider) => {
        const text = `${provider.name} ${provider.type} ${providerDisplay(provider)}`.toLowerCase();
        return (filter === 'all' || provider.type === filter) && (!keyword || text.includes(keyword));
      });
      const stats = ['ASR', 'TTS', 'LLM', 'VLLLM'].map((type) => `${type}: ${providers.filter((p) => p.type === type).length}`).join('，');
      byId('providers-body').innerHTML = `
        <div class="card"><span class="muted">${escapeHtml(stats)}</span></div>
        ${filtered.length ? `<div class="grid provider-grid section">${filtered.map(providerCard).join('')}</div>` : emptyState('No matching providers found')}
      `;
    };
    byId('provider-search').addEventListener('input', draw);
    byId('provider-filter').addEventListener('change', draw);
    draw();
  }

  function providerCard(provider) {
    const config = provider.config || {};
    const configRows = Object.entries(config).filter(([key]) => key !== 'type').slice(0, 5).map(([key, value]) => `
      <div><span class="muted">${escapeHtml(providerFieldLabel(key))}：</span>${escapeHtml(displayProviderValue(key, value))}</div>
    `).join('');
    return `
      <div class="card provider-card">
        <div class="card-title">
          <div>
            <h3>${escapeHtml(provider.name)}</h3>
            <span class="subtle">${escapeHtml(providerTypeLabel(provider.type))}</span>
          </div>
          <div class="chip-row"><span class="tag blue">${escapeHtml(provider.type)}</span><span class="tag">${escapeHtml(config.type || 'Unset type')}</span></div>
        </div>
        <div class="form">${configRows || '<span class="muted">No configuration</span>'}</div>
        <div class="divider"></div>
        <div class="inline-actions">
          <button class="btn small" data-test-provider="${escapeHtml(provider.type)}:${escapeHtml(provider.name)}">Test</button>
          <button class="btn small" data-edit-provider="${escapeHtml(provider.type)}:${escapeHtml(provider.name)}">Edit</button>
          <button class="btn danger small" data-delete-provider="${escapeHtml(provider.type)}:${escapeHtml(provider.name)}">Delete</button>
        </div>
        <div class="provider-test-result" aria-live="polite"></div>
      </div>
    `;
  }

  const PROVIDER_FIELDS = {
    ASR: {
      doubao: { label: 'Volcengine ASR', fields: [['appid', 'App ID', 'e.g. 1234567890', true], ['access_token', 'Access Token', 'Token from console', true], ['output_dir', 'Temp Directory', 'tmp/'], ['end_window_size', 'Silence Timeout (ms)', '300', false, 'number']] },
      deepgram: { label: 'Deepgram', fields: [['addr', 'Service Address', 'wss://api.deepgram.com/v1/listen'], ['api_key', 'API Key', 'Enter Deepgram API Key', true], ['lang', 'Language', 'zh-CN'], ['output_dir', 'Temp Directory', 'tmp/']] },
      gosherpa: { label: 'GoSherpa', fields: [['addr', 'Service Address', 'ws://127.0.0.1:8848/asr']] },
      stepfun: { label: 'StepFun', fields: [['api_key', 'API Key', 'Enter StepFun API Key', true], ['model', 'Model Name', 'step-audio-2-mini'], ['voice', 'Voice', 'qingchunshaonv']] },
      iflytek: { label: 'iFlytek ASR', fields: [['appid', 'App ID', 'Enter APPID', true], ['api_key', 'API Key', 'Enter API Key', true], ['api_secret', 'API Secret', 'Enter API Secret', true], ['asr_url', 'Service Address', 'wss://iat-api.xfyun.cn/v2/iat']] },
    },
    TTS: {
      edge: { label: 'Microsoft Edge TTS', fields: [['voice', 'Voice', 'zh-CN-XiaoxiaoNeural'], ['output_dir', 'Temp Directory', 'tmp/']] },
      doubao: { label: 'Volcengine TTS', fields: [['appid', 'App ID', 'Enter APPID', true], ['token', 'Access Token', 'Enter Access Token', true], ['cluster', 'Cluster', 'Enter resource cluster name'], ['voice', 'Voice', 'zh_female_wanwanxiaohe_moon_bigtts'], ['output_dir', 'Temp Directory', 'tmp/']] },
      gosherpa: { label: 'GoSherpa', fields: [['cluster', 'Service Address', 'ws://127.0.0.1:8848/tts'], ['output_dir', 'Temp Directory', 'tmp/']] },
      deepgram: { label: 'Deepgram', fields: [['cluster', 'Service Address', 'wss://api.deepgram.com/v1/speak'], ['token', 'API Key', 'Enter Deepgram API Key', true], ['voice', 'Voice', 'aura-2-zeus-en'], ['output_dir', 'Temp Directory', 'tmp/']] },
      iflytek: { label: 'iFlytek TTS', fields: [['appid', 'App ID', 'Enter APPID', true], ['token', 'API Key', 'Enter API Key', true], ['cluster', 'API Secret', 'Enter API Secret', true], ['voice', 'Voice', 'xiaoyan'], ['output_dir', 'Temp Directory', 'tmp/']] },
    },
    LLM: {
      openai: { label: 'OpenAI Compatible', fields: [['url', 'API URL', 'https://api.example.com/v1'], ['api_key', 'API Key', 'Enter API Key', true], ['model_name', 'Model Name', 'e.g. gpt-4o-mini'], ['max_tokens', 'Max Tokens', '500', false, 'number']] },
      ollama: { label: 'Ollama Local Model', fields: [['url', 'Ollama URL', 'http://127.0.0.1:11434'], ['model_name', 'Model Name', 'e.g. qwen3:8b']] },
      dsh: { label: 'DSH Xiaozhi Bridge', suggestedName: 'DSHBridge', fields: [['url', 'Bridge WebSocket URL', 'ws://127.0.0.1:17980/xiaozhi', false, 'url', 'ws://127.0.0.1:17980/xiaozhi'], ['api_key', 'Bridge Token', 'Same as XIAOZHI_BRIDGE_TOKEN', true]] },
      doubao: { label: 'Volcengine Ark', fields: [['url', 'API URL', 'https://ark.cn-beijing.volces.com/api/v3'], ['api_key', 'API Key', 'Enter API Key', true], ['model_name', 'Model Name', 'Enter model endpoint ID'], ['max_tokens', 'Max Tokens', '4000', false, 'number']] },
      coze: { label: 'Coze', fields: [['bot_id', 'Bot ID', 'Enter Bot ID', true], ['user_id', 'User ID', 'Enter User ID', true], ['personal_access_token', 'Personal Access Token', 'Enter Personal Access Token', true], ['url', 'API URL', 'https://api.coze.cn']] },
    },
    VLLLM: {
      openai: { label: 'OpenAI Compatible Vision', fields: [['url', 'API URL', 'https://api.example.com/v1'], ['api_key', 'API Key', 'Enter API Key', true], ['model_name', 'Model Name', 'e.g. glm-4v-flash'], ['max_tokens', 'Max Tokens', '1024', false, 'number']] },
      ollama: { label: 'Ollama Vision', fields: [['url', 'Ollama URL', 'http://127.0.0.1:11434'], ['model_name', 'Model Name', 'e.g. qwen2.5vl:7b']] },
    },
  };

  const PROVIDER_FIELD_LABELS = { url: 'Endpoint URL', addr: 'Service Address', asr_url: 'ASR URL', cluster: 'Cluster / Endpoint', model_name: 'Model Name', model: 'Model', voice: 'Voice', type: 'Type', appid: 'App ID', lang: 'Language', output_dir: 'Temp Directory', max_tokens: 'Max Tokens' };

  function providerTypeLabel(type) {
    return { ASR: 'Speech Recognition (ASR)', TTS: 'Speech Synthesis (TTS)', LLM: 'Large Language Model (LLM)', VLLLM: 'Vision Language Model (VLLM)' }[type] || type;
  }

  function providerFieldLabel(key) {
    return PROVIDER_FIELD_LABELS[key] || key;
  }

  function displayProviderValue(key, value) {
    if (/(token|secret|key|password)/i.test(key)) return 'Configured (Hidden)';
    if (value == null) return '';
    const text = typeof value === 'object' ? JSON.stringify(value) : String(value);
    return text.length > 64 ? `${text.slice(0, 56)}…` : text;
  }

  async function openProviderModal(provider = null) {
    const config = provider?.config || { type: 'openai' };
    const selectedType = provider?.type || 'LLM';
    const selectedSubType = config.type || Object.keys(PROVIDER_FIELDS[selectedType] || {})[0] || 'openai';
    openModal({
      title: provider ? 'Edit Provider' : 'Add Provider',
      body: `
        <form class="form" id="provider-form">
          <div class="alert provider-form-intro">Select the category and provider type, then fill in fields. Advanced settings can be edited in Advanced JSON. Keys are kept private.</div>
          <div class="form-grid">
            <div class="field">
              <label>Category</label>
              <select name="type" ${provider ? 'disabled' : ''}>
                ${['ASR', 'TTS', 'LLM', 'VLLLM'].map((type) => `<option value="${type}" ${selectedType === type ? 'selected' : ''}>${type} · ${providerTypeLabel(type)}</option>`).join('')}
              </select>
            </div>
            <div class="field">
              <label>Provider Name</label>
              <input name="name" value="${escapeHtml(provider?.name || '')}" placeholder="e.g. My Provider" ${provider ? 'disabled' : ''} required>
            </div>
            <div class="field">
              <label>Specific Type</label>
              <select name="subType" id="provider-subtype" required></select>
            </div>
            <div class="field" id="provider-custom-subtype-field" hidden>
              <label for="provider-custom-subtype">Custom Subtype</label>
              <input name="customSubType" id="provider-custom-subtype" placeholder="e.g. my_provider">
            </div>
          </div>
          <div id="provider-config-fields" class="form-grid"></div>
          <details class="provider-advanced">
            <summary>Advanced JSON (Optional)</summary>
            <p class="subtle">Form fields sync here automatically. Edit directly only for extra unlisted parameters.</p>
            <textarea class="codebox" name="configJson" spellcheck="false">${escapeHtml(prettyJson(config))}</textarea>
          </details>
          <div id="provider-form-status" class="subtle">You can run tests from the provider card after saving.</div>
        </form>
      `,
      footer: `<button class="btn" data-close-modal>Cancel</button><button class="btn primary" id="provider-submit">Save</button>`,
    });

    const form = byId('provider-form');
    const typeInput = form.elements.type;
    const subTypeInput = form.elements.subType;
    const customSubTypeInput = form.elements.customSubType;
    const jsonInput = form.elements.configJson;
    const currentConfig = () => parseJson(jsonInput.value, {}) || {};
    const effectiveSubType = () => (subTypeInput.value === '__custom__' ? customSubTypeInput.value.trim() : subTypeInput.value.trim());
    const renderProviderFields = (resetConfig = false) => {
      const type = provider?.type || typeInput.value;
      const definitions = PROVIDER_FIELDS[type] || {};
      const knownSubType = Object.prototype.hasOwnProperty.call(definitions, selectedSubType.toLowerCase()) ? selectedSubType.toLowerCase() : '';
      const shouldSetInitialSubType = !resetConfig && !subTypeInput.dataset.initialized;
      const selectedBeforeRender = subTypeInput.value;
      subTypeInput.innerHTML = `${Object.entries(definitions).map(([value, definition]) => `<option value="${escapeHtml(value)}">${escapeHtml(`${value} · ${definition.label}`)}</option>`).join('')}<option value="__custom__">Custom Type...</option>`;
      if (resetConfig) subTypeInput.value = Object.keys(definitions)[0] || '__custom__';
      if (shouldSetInitialSubType) {
        subTypeInput.value = knownSubType || '__custom__';
        customSubTypeInput.value = knownSubType ? '' : selectedSubType;
        subTypeInput.dataset.initialized = 'true';
      } else if (!resetConfig) {
        subTypeInput.value = selectedBeforeRender;
      }
      if (!subTypeInput.value) subTypeInput.value = '__custom__';
      const customType = subTypeInput.value === '__custom__';
      const customTypeField = byId('provider-custom-subtype-field');
      customTypeField.hidden = !customType;
      customSubTypeInput.required = customType;
      const subType = effectiveSubType().toLowerCase();
      const definition = definitions[subType];
      if (resetConfig) {
        jsonInput.value = prettyJson({ type: subType || Object.keys(definitions)[0] || 'openai' });
      }
      const configValue = currentConfig();
      const fields = definition?.fields || [];
      byId('provider-config-fields').innerHTML = fields.length ? fields.map(([key, label, placeholder, secret, inputType, defaultValue]) => `
        <div class="field">
          <label for="provider-field-${escapeHtml(key)}">${escapeHtml(label)}</label>
          <div class="${secret ? 'input-with-button' : ''}">
            <input id="provider-field-${escapeHtml(key)}" data-provider-config-field="${escapeHtml(key)}" type="${secret ? 'password' : (inputType || 'text')}" value="${escapeHtml(configValue[key] ?? defaultValue ?? '')}" placeholder="${escapeHtml(placeholder || '')}" ${['appid', 'access_token', 'api_key', 'api_secret', 'token', 'model_name', 'model', 'voice'].includes(key) || (subType === 'dsh' && key === 'url') ? 'required' : ''}>
            ${secret ? `<button class="btn small" type="button" data-toggle-provider-secret aria-label="Toggle secret for ${escapeHtml(label)}">Show</button>` : ''}
          </div>
        </div>
      `).join('') : '<div class="alert warning">No preset form fields for this type. Please enter full config in Advanced JSON.</div>';
      qsa('[data-provider-config-field]', form).forEach((input) => input.addEventListener('input', syncProviderJson));
      qsa('[data-toggle-provider-secret]', form).forEach((button) => button.addEventListener('click', () => {
        const input = button.parentElement.querySelector('[data-provider-config-field]');
        const hidden = input.type === 'password';
        input.type = hidden ? 'text' : 'password';
        button.textContent = hidden ? 'Hide' : 'Show';
      }));
      if (definition?.suggestedName && !provider && !form.elements.name.value.trim()) form.elements.name.placeholder = `Suggested Name: ${definition.suggestedName}`;
      byId('provider-form-status').textContent = definition ? `Selected: ${providerTypeLabel(type)} · ${definition.label}${definition.suggestedName ? `; Suggested Name: ${definition.suggestedName}` : ''}` : 'Please enter custom type and complete config in Advanced JSON.';
    };
    const syncProviderJson = () => {
      const configValue = currentConfig();
      configValue.type = effectiveSubType();
      qsa('[data-provider-config-field]', form).forEach((input) => {
        const value = input.value.trim();
        if (!value) return;
        configValue[input.dataset.providerConfigField] = input.type === 'number' ? Number(value) : value;
      });
      jsonInput.value = prettyJson(configValue);
    };
    typeInput.addEventListener('change', () => {
      const type = typeInput.value;
      subTypeInput.innerHTML = '';
      customSubTypeInput.value = '';
      renderProviderFields(true);
    });
    subTypeInput.addEventListener('change', () => { syncProviderJson(); renderProviderFields(false); });
    customSubTypeInput.addEventListener('input', syncProviderJson);
    renderProviderFields(false);
    byId('provider-submit').addEventListener('click', async () => {
      if (!form.reportValidity()) return;
      const values = formValues(form);
      const type = provider?.type || values.type;
      const name = provider?.name || values.name;
      syncProviderJson();
      const config = parseJson(values.configJson, null);
      if (!config) {
        toast('Invalid JSON format', 'warning');
        return;
      }
      config.type = effectiveSubType() || config.type;
      try {
        if (provider) {
          await request(`/api/user/providers/${encodeURIComponent(type)}/${encodeURIComponent(name)}`, {
            method: 'PUT',
            body: { data: config },
          });
        } else {
          await request('/api/user/providers/create', {
            method: 'POST',
            body: { type, name, data: config },
          });
        }
        toast('Saved successfully', 'success');
        closeModal();
        render();
      } catch (error) {
        showError(error, 'Failed to save');
      }
    });
  }

  async function renderUnbindDevice() {
    page().innerHTML = `
      ${pageHeader('Unbind Device', 'Administrators can unbind devices by Device ID')}
      <div class="card">
        <form class="form" id="unbind-form">
          <div class="field">
            <label>Device ID</label>
            <input name="deviceID" placeholder="Enter Device ID" required>
          </div>
          <button class="btn danger" type="submit" ${canWriteAdmin() ? '' : 'disabled'}>Unbind</button>
        </form>
      </div>
    `;
    byId('unbind-form').addEventListener('submit', async (event) => {
      event.preventDefault();
      const values = formValues(event.currentTarget);
      try {
        await request('/api/admin/system/device', { method: 'DELETE', body: { deviceID: values.deviceID } });
        toast('Unbound successfully', 'success');
      } catch (error) {
        showError(error, 'Failed to unbind');
      }
    });
  }

  async function renderAdvancedConfig() {
    page().innerHTML = `
      ${pageHeader('Advanced Settings', 'Manage system runtime configuration')}
      <div class="card">
        <div class="tabs">
          ${[
            ['application', 'Application Config'],
            ['transport', 'Transport Config'],
            ['web', 'Web Config'],
            ['auth', 'Auth Config'],
            ['roles', 'Role Presets'],
            ['mcp-functions', 'Local MCP Tools'],
            ['exit-commands', 'Exit Commands'],
            ['log', 'Logging Config'],
          ].map(([key, label]) => `<button class="tab ${state.advancedTab === key ? 'active' : ''}" data-advanced-tab="${key}">${label}</button>`).join('')}
        </div>
        <div id="advanced-body">${loadingCard()}</div>
      </div>
    `;
    await drawAdvancedTab(state.advancedTab);
  }

  async function drawAdvancedTab(tab) {
    state.advancedTab = tab;
    qsa('[data-advanced-tab]').forEach((button) => button.classList.toggle('active', button.dataset.advancedTab === tab));
    const body = byId('advanced-body');
    body.innerHTML = loadingCard();
    if (tab === 'application') return drawSimpleConfig('application', [
      ['quickReply', 'Quick Reply', 'checkbox'],
      ['saveTtsAudio', 'Save TTS Audio', 'checkbox'],
      ['saveUserAudio', 'Save User Audio', 'checkbox'],
    ]);
    if (tab === 'web') return drawSimpleConfig('web', [
      ['port', 'Web Server Port', 'number'],
      ['staticDir', 'Static Directory', 'text'],
      ['websocket', 'WebSocket URL', 'text'],
      ['visionUrl', 'Vision Service URL', 'text'],
      ['activateText', 'Activation Prompt', 'text'],
    ]);
    if (tab === 'auth') return drawSimpleConfig('auth', [
      ['token', 'Access Token', 'password'],
      ['expiry', 'Expiry (Hours)', 'number'],
    ]);
    if (tab === 'log') return drawSimpleConfig('log', [
      ['logLevel', 'Log Level', 'select:debug,info,warn,error'],
      ['logDir', 'Log Directory', 'text'],
      ['logFile', 'Log Filename', 'text'],
    ]);
    if (tab === 'transport') return drawTransportConfig();
    if (tab === 'roles') return drawCollectionConfig('roles', 'Role', ['name', 'description', 'enabled']);
    if (tab === 'mcp-functions') return drawCollectionConfig('mcp-functions', 'Local MCP Tool', ['functionName', 'description', 'enabled']);
    if (tab === 'exit-commands') return drawCollectionConfig('exit-commands', 'Exit Command', ['command', 'enabled']);
  }

  async function drawSimpleConfig(type, fields) {
    const config = await request(`/api/admin/config/${type}`).then((res) => res.data || {});
    byId('advanced-body').innerHTML = `
      <form class="form" id="simple-config-form">
        <div class="form-grid">
          ${fields.map(([key, label, fieldType]) => simpleField(key, label, fieldType, config[key])).join('')}
        </div>
        <button class="btn primary" type="submit" ${canWriteAdmin() ? '' : 'disabled'}>Save Configuration</button>
      </form>
    `;
    byId('simple-config-form').addEventListener('submit', async (event) => {
      event.preventDefault();
      const values = formValues(event.currentTarget);
      fields.forEach(([key, _label, fieldType]) => {
        if (fieldType === 'number') values[key] = Number(values[key]);
      });
      try {
        await request(`/api/admin/config/${type}`, { method: 'PUT', body: values });
        toast('Saved successfully', 'success');
      } catch (error) {
        showError(error, 'Failed to save');
      }
    });
  }

  function simpleField(key, label, fieldType, value) {
    if (fieldType === 'checkbox') {
      return `<label class="field checkbox"><input type="checkbox" name="${escapeHtml(key)}" ${value ? 'checked' : ''}><span>${escapeHtml(label)}</span></label>`;
    }
    if (fieldType.startsWith('select:')) {
      const options = fieldType.slice(7).split(',');
      return `
        <div class="field">
          <label>${escapeHtml(label)}</label>
          <select name="${escapeHtml(key)}">
            ${options.map((option) => `<option value="${escapeHtml(option)}" ${option === value ? 'selected' : ''}>${escapeHtml(option)}</option>`).join('')}
          </select>
        </div>
      `;
    }
    return `
      <div class="field">
        <label>${escapeHtml(label)}</label>
        <input name="${escapeHtml(key)}" type="${fieldType}" value="${escapeHtml(value ?? '')}">
      </div>
    `;
  }

  async function drawTransportConfig() {
    const configs = await request('/api/admin/config/transport').then((res) => res.data || []);
    byId('advanced-body').innerHTML = `
      <form class="form" id="transport-form">
        <div class="field">
          <label>Transport Configuration JSON</label>
          <textarea class="codebox" name="transportJson" rows="18" spellcheck="false">${escapeHtml(prettyJson(configs))}</textarea>
        </div>
        <button class="btn primary" type="submit" ${canWriteAdmin() ? '' : 'disabled'}>Save Configuration</button>
      </form>
    `;
    byId('transport-form').addEventListener('submit', async (event) => {
      event.preventDefault();
      const values = formValues(event.currentTarget);
      const next = parseJson(values.transportJson, null);
      if (!Array.isArray(next)) {
        toast('Transport configuration must be a JSON array', 'warning');
        return;
      }
      try {
        await request('/api/admin/config/transport', { method: 'PUT', body: next });
        toast('Saved successfully', 'success');
      } catch (error) {
        showError(error, 'Failed to save');
      }
    });
  }

  async function drawCollectionConfig(endpoint, label, fields) {
    const payload = await request(`/api/admin/config/${endpoint}`);
    const rows = normalizeList(payload);
    byId('advanced-body').innerHTML = `
      <div class="toolbar" style="justify-content:space-between;margin-bottom:12px">
        <span class="muted">${rows.length} item(s) total</span>
        <button class="btn primary" id="collection-add" ${canWriteAdmin() ? '' : 'disabled'}>Add ${escapeHtml(label)}</button>
      </div>
      <div class="table-wrap">
        <table>
          <thead><tr>${fields.map((field) => `<th>${escapeHtml(field)}</th>`).join('')}<th>Actions</th></tr></thead>
          <tbody>
            ${rows.map((row) => `
              <tr>
                ${fields.map((field) => `<td>${field === 'enabled' ? (row[field] ? '<span class="tag green">Enabled</span>' : '<span class="tag">Disabled</span>') : escapeHtml(row[field] ?? '')}</td>`).join('')}
                <td>
                  <button class="btn small" data-edit-collection="${escapeHtml(endpoint)}:${escapeHtml(collectionId(row, endpoint))}">Edit</button>
                  <button class="btn danger small" data-delete-collection="${escapeHtml(endpoint)}:${escapeHtml(collectionId(row, endpoint))}">Delete</button>
                </td>
              </tr>
            `).join('') || `<tr><td colspan="${fields.length + 1}">${emptyState(`No ${label} configured`)}</td></tr>`}
          </tbody>
        </table>
      </div>
    `;
    byId('collection-add')?.addEventListener('click', () => openCollectionModal(endpoint, label, fields));
  }

  function collectionId(row, endpoint) {
    if (endpoint === 'roles') return row.name;
    if (endpoint === 'mcp-functions') return row.functionName;
    if (endpoint === 'exit-commands') return row.command;
    return row.id || row.name || '';
  }

  function openCollectionModal(endpoint, label, fields, row = null) {
    openModal({
      title: `${row ? 'Edit' : 'Add'} ${label}`,
      body: `
        <form class="form" id="collection-form">
          ${fields.map((field) => field === 'enabled'
            ? `<label class="field checkbox"><input type="checkbox" name="enabled" ${row?.enabled ? 'checked' : ''}><span>Enabled</span></label>`
            : `<div class="field"><label>${escapeHtml(field)}</label><input name="${escapeHtml(field)}" value="${escapeHtml(row?.[field] ?? '')}" required></div>`
          ).join('')}
        </form>
      `,
      footer: `<button class="btn" data-close-modal>Cancel</button><button class="btn primary" id="collection-submit">Save</button>`,
    });
    byId('collection-submit').addEventListener('click', async () => {
      const form = byId('collection-form');
      if (!form.reportValidity()) return;
      const values = formValues(form);
      try {
        if (row) {
          await request(`/api/admin/config/${endpoint}/${encodeURIComponent(collectionId(row, endpoint))}`, { method: 'PUT', body: values });
        } else {
          await request(`/api/admin/config/${endpoint}`, { method: 'POST', body: values });
        }
        closeModal();
        toast('Saved successfully', 'success');
        drawAdvancedTab(endpoint);
      } catch (error) {
        showError(error, 'Failed to save');
      }
    });
  }

  async function renderWhiteList() {
    page().innerHTML = `
      ${pageHeader('Whitelist', 'Manage authorized device MAC addresses', `
        <button class="btn primary" id="white-add">Add</button>
        <button class="btn" id="white-batch">Batch Add</button>
      `)}
      <div id="white-body">${loadingCard()}</div>
    `;
    byId('white-add').addEventListener('click', openWhiteListModal);
    byId('white-batch').addEventListener('click', openWhiteListUpload);
    try {
      const payload = await request('/api/admin/system/whiteList?current=1&pageSize=50');
      const table = normalizeTablePayload(payload);
      byId('white-body').innerHTML = `
        <div class="card">
          <div class="table-wrap">
            <table>
              <thead><tr><th>MAC Address</th><th>Actions</th></tr></thead>
              <tbody>
                ${table.data.map((row) => `
                  <tr>
                    <td>${escapeHtml(row.mac || row.deviceId || '')}</td>
                    <td><button class="btn danger small" data-delete-white="${escapeHtml(row.mac || row.deviceId || '')}">Delete</button></td>
                  </tr>
                `).join('') || `<tr><td colspan="2">${emptyState('No whitelist entries')}</td></tr>`}
              </tbody>
            </table>
          </div>
        </div>
      `;
    } catch (_error) {
      byId('white-body').innerHTML = `<div class="alert warning">Whitelist interface unavailable or current user lacks permission.</div>`;
    }
  }

  function openWhiteListModal() {
    openModal({
      title: 'Add to Whitelist',
      body: `
        <form class="form" id="white-form">
          <div class="field">
            <label>MAC Address</label>
            <input name="mac" pattern="^([0-9A-Fa-f]{2}[:-]){5}([0-9A-Fa-f]{2})$" placeholder="AA:BB:CC:DD:EE:FF" required>
          </div>
        </form>
      `,
      footer: `<button class="btn" data-close-modal>Cancel</button><button class="btn primary" id="white-submit">Add</button>`,
    });
    byId('white-submit').addEventListener('click', async () => {
      const form = byId('white-form');
      if (!form.reportValidity()) return;
      try {
        await request('/api/admin/system/whiteList/add', { method: 'POST', body: formValues(form) });
        closeModal();
        toast('Added successfully', 'success');
        render();
      } catch (error) {
        showError(error, 'Failed to add');
      }
    });
  }

  function openWhiteListUpload() {
    openModal({
      title: 'Batch Add to Whitelist',
      body: `
        <form class="form" id="white-upload-form">
          <div class="field">
            <label>CSV File</label>
            <input name="file" type="file" accept=".csv" required>
          </div>
        </form>
      `,
      footer: `<button class="btn" data-close-modal>Cancel</button><button class="btn primary" id="white-upload-submit">Upload</button>`,
    });
    byId('white-upload-submit').addEventListener('click', async () => {
      const form = byId('white-upload-form');
      if (!form.reportValidity()) return;
      const file = qs('input[type="file"]', form).files[0];
      const data = new FormData();
      data.append('file', file);
      try {
        await request('/api/admin/system/whiteList/batchAdd', { method: 'POST', body: data });
        closeModal();
        toast('Uploaded successfully', 'success');
        render();
      } catch (error) {
        showError(error, 'Upload failed');
      }
    });
  }

  document.addEventListener('click', async (event) => {
    const target = event.target instanceof Element ? event.target : event.target.parentElement;
    if (!target) return;

    const link = target.closest('[data-link]');
    if (link) {
      event.preventDefault();
      navigate(link.dataset.link || link.getAttribute('href'));
      return;
    }

    if (target.closest('[data-action="logout"]')) {
      clearUser();
      navigate('/login');
      return;
    }

    if (!target.closest('.user-menu')) {
      byId('user-dropdown')?.classList.remove('open');
    }

    const copy = target.closest('[data-copy]');
    if (copy) {
      navigator.clipboard?.writeText(copy.dataset.copy);
      toast('Copied to clipboard', 'success');
      return;
    }

    const removeQuick = target.closest('[data-remove-quick]');
    if (removeQuick) {
      removeQuick.closest('.tag')?.remove();
      return;
    }

    const deleteAgent = target.closest('[data-delete-agent]');
    if (deleteAgent) {
      confirmAction('Delete Agent', 'This will delete configuration and chat history permanently. Are you sure?', async () => {
        await request(`/api/user/agent/${deleteAgent.dataset.deleteAgent}`, { method: 'DELETE' });
        toast('Deleted successfully', 'success');
        render();
      });
      return;
    }

    const unbind = target.closest('[data-unbind-device]');
    if (unbind) {
      confirmAction('Unbind Device', 'Device must be bound again after unbinding. Continue?', async () => {
        await request('/api/user/device', { method: 'DELETE', body: { deviceID: unbind.dataset.unbindDevice } });
        toast('Unbound successfully', 'success');
        render();
      });
      return;
    }

    const toggleDevice = target.closest('[data-toggle-device]');
    if (toggleDevice) {
      const currentlyDisabled = toggleDevice.dataset.disabled === 'true';
      confirmAction(currentlyDisabled ? 'Enable Device' : 'Disable Device', currentlyDisabled ? 'Device will be allowed to start voice sessions again. Continue?' : 'Device will be blocked from starting voice sessions. Continue?', async () => {
        await request(`/api/user/device/${encodeURIComponent(toggleDevice.dataset.toggleDevice)}`, {
          method: 'PUT',
          body: { disabled: !currentlyDisabled },
        });
        toast(currentlyDisabled ? 'Device enabled' : 'Device disabled', 'success');
        render();
      });
      return;
    }

    const clearDeviceMemory = target.closest('[data-clear-device-memory]');
    if (clearDeviceMemory) {
      confirmAction('Clear Short-term Memory', 'This will clear short-term context for this device and agent. Continue?', async () => {
        await request(`/api/user/device/${encodeURIComponent(clearDeviceMemory.dataset.clearDeviceMemory)}/memory`, { method: 'DELETE' });
        toast('Short-term memory cleared', 'success');
      });
      return;
    }

    const conversation = target.closest('[data-conversation]');
    if (conversation) {
      try {
        await selectConversation(conversation.dataset.conversation);
      } catch (error) {
        showError(error, 'Failed to load conversation');
      }
      return;
    }

    const deleteDialog = target.closest('[data-delete-dialog]');
    if (deleteDialog) {
      confirmAction('Delete Conversation', 'This conversation will be permanently removed. Continue?', async () => {
        await request(`/api/user/agent/history_dialog/${deleteDialog.dataset.deleteDialog}`, { method: 'DELETE' });
        toast('Deleted successfully', 'success');
        render();
      });
      return;
    }

    const editProvider = target.closest('[data-edit-provider]');
    if (editProvider) {
      const [type, ...nameParts] = editProvider.dataset.editProvider.split(':');
      const name = nameParts.join(':');
      const providers = await getModelProviders();
      const provider = providers.find((item) => item.type === type && item.name === name);
      openProviderModal(provider);
      return;
    }

    const testProvider = target.closest('[data-test-provider]');
    if (testProvider) {
      const [type, ...nameParts] = testProvider.dataset.testProvider.split(':');
      const name = nameParts.join(':');
      const resultNode = testProvider.closest('.provider-card')?.querySelector('.provider-test-result');
      const previousText = testProvider.textContent;
      testProvider.disabled = true;
      testProvider.textContent = 'Testing...';
      if (resultNode) resultNode.innerHTML = '<span class="subtle">Verifying saved configuration, please wait...</span>';
      try {
        const result = await request(`/api/user/providers/${encodeURIComponent(type)}/${encodeURIComponent(name)}/test`, { method: 'POST' });
        const success = result?.success === true;
        if (resultNode) {
          resultNode.innerHTML = `<div class="provider-test-message ${success ? 'success' : 'error'}">${escapeHtml(result?.message || (success ? 'Test succeeded' : 'Test failed'))}</div>`;
          const audioData = result?.data?.audio_base64;
          const audioFormat = String(result?.data?.audio_format || '').toLowerCase();
          if (success && typeof audioData === 'string' && /^(wav|mp3|ogg)$/.test(audioFormat)) {
            const player = document.createElement('audio');
            player.className = 'provider-test-audio';
            player.controls = true;
            player.setAttribute('aria-label', 'Speech synthesis test player');
            player.src = `data:audio/${audioFormat};base64,${audioData}`;
            resultNode.append(player);
          }
        }
        toast(result?.message || (success ? 'Test succeeded' : 'Test failed'), success ? 'success' : 'error');
      } catch (error) {
        if (resultNode) resultNode.innerHTML = `<div class="provider-test-message error">${escapeHtml(error?.data?.message || error?.message || 'Test failed')}</div>`;
        showError(error, 'Test failed');
      } finally {
        testProvider.disabled = false;
        testProvider.textContent = previousText;
      }
      return;
    }

    const deleteProvider = target.closest('[data-delete-provider]');
    if (deleteProvider) {
      const [type, ...nameParts] = deleteProvider.dataset.deleteProvider.split(':');
      const name = nameParts.join(':');
      confirmAction('Delete Provider', `Are you sure you want to delete ${type}/${name}?`, async () => {
        await request(`/api/user/providers/${encodeURIComponent(type)}/${encodeURIComponent(name)}`, { method: 'DELETE' });
        toast('Deleted successfully', 'success');
        render();
      });
      return;
    }

    const tab = target.closest('[data-advanced-tab]');
    if (tab) {
      await drawAdvancedTab(tab.dataset.advancedTab);
      return;
    }

    const editCollection = target.closest('[data-edit-collection]');
    if (editCollection) {
      const [endpoint, ...idParts] = editCollection.dataset.editCollection.split(':');
      const id = idParts.join(':');
      const payload = await request(`/api/admin/config/${endpoint}`);
      const rows = normalizeList(payload);
      const row = rows.find((item) => collectionId(item, endpoint) === id);
      const labelMap = { roles: 'Role', 'mcp-functions': 'Local MCP Tool', 'exit-commands': 'Exit Command' };
      const fieldsMap = {
        roles: ['name', 'description', 'enabled'],
        'mcp-functions': ['functionName', 'description', 'enabled'],
        'exit-commands': ['command', 'enabled'],
      };
      openCollectionModal(endpoint, labelMap[endpoint], fieldsMap[endpoint], row);
      return;
    }

    const deleteCollection = target.closest('[data-delete-collection]');
    if (deleteCollection) {
      const [endpoint, ...idParts] = deleteCollection.dataset.deleteCollection.split(':');
      const id = idParts.join(':');
      confirmAction('Delete Item', 'Are you sure you want to delete this configuration item?', async () => {
        await request(`/api/admin/config/${endpoint}/${encodeURIComponent(id)}`, { method: 'DELETE' });
        toast('Deleted successfully', 'success');
        drawAdvancedTab(endpoint);
      });
      return;
    }

    const deleteWhite = target.closest('[data-delete-white]');
    if (deleteWhite) {
      confirmAction('Delete Whitelist Entry', 'Are you sure you want to remove this MAC address?', async () => {
        await request('/api/admin/system/whiteList', {
          method: 'DELETE',
          body: { mac: deleteWhite.dataset.deleteWhite },
        });
        toast('Deleted successfully', 'success');
        render();
      });
    }
  });

  window.addEventListener('popstate', render);
  window.addEventListener('hashchange', render);

  fetch(asset('branding.json'), { cache: 'no-cache' })
    .then((response) => response.ok ? response.json() : null)
    .then((branding) => {
      if (branding) {
        state.branding = {
          ...state.branding,
          ...branding,
          logoUrl: branding.logoUrl || state.branding.logoUrl,
        };
      }
    })
    .catch(() => null)
    .finally(render);
})();
