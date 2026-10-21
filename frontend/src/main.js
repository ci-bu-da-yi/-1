import './style.css';
import './app.css';
import { EventsOn } from '../wailsjs/runtime/runtime';
import { Quit } from '../wailsjs/runtime/runtime';

const root = document.querySelector('#app');
const previewMode = import.meta.env.DEV
  ? new URLSearchParams(window.location.search).get('preview')
  : null;

const state = {
  user: null,
  authMode: 'login',
  form: {
    username: '',
    password: '',
    confirmPassword: '',
  },
  loading: false,
  authError: '',
  monitorError: '',
  backgroundMode: false,
  collapsed: false,
  messages: [],
  toasts: [],
  receivedCount: 0,
};

const iconPaths = {
  alert: '<path d="m21.73 18-8-14a2 2 0 0 0-3.48 0l-8 14A2 2 0 0 0 4 21h16a2 2 0 0 0 1.73-3Z"/><path d="M12 9v4"/><path d="M12 17h.01"/>',
  bell: '<path d="M10.268 21a2 2 0 0 0 3.464 0"/><path d="M3.262 15.326A1 1 0 0 0 4 17h16a1 1 0 0 0 .74-1.673C19.41 13.956 18 12.499 18 8A6 6 0 0 0 6 8c0 4.499-1.411 5.956-2.738 7.326"/>',
  check: '<path d="M20 6 9 17l-5-5"/>',
  chevronUp: '<path d="m18 15-6-6-6 6"/>',
  clock: '<circle cx="12" cy="12" r="10"/><polyline points="12 6 12 12 16 14"/>',
  eye: '<path d="M2.062 12.348a1 1 0 0 1 0-.696 10.75 10.75 0 0 1 19.876 0 1 1 0 0 1 0 .696 10.75 10.75 0 0 1-19.876 0"/><circle cx="12" cy="12" r="3"/>',
  eyeOff: '<path d="M10.733 5.076a10.744 10.744 0 0 1 11.205 6.575 1 1 0 0 1 0 .696 10.747 10.747 0 0 1-1.444 2.49"/><path d="M14.084 14.158a3 3 0 0 1-4.242-4.242"/><path d="M17.479 17.499a10.75 10.75 0 0 1-15.417-5.151 1 1 0 0 1 0-.696 10.75 10.75 0 0 1 4.446-5.143"/><path d="m2 2 20 20"/>',
  lock: '<rect width="18" height="11" x="3" y="11" rx="2" ry="2"/><path d="M7 11V7a5 5 0 0 1 10 0v4"/>',
  logout: '<path d="M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4"/><polyline points="16 17 21 12 16 7"/><line x1="21" x2="9" y1="12" y2="12"/>',
  message: '<path d="M7.9 20A9 9 0 1 0 4 16.1L2 22Z"/><path d="M8 12h.01"/><path d="M12 12h.01"/><path d="M16 12h.01"/>',
  panelClose: '<rect width="18" height="18" x="3" y="3" rx="2"/><path d="M15 3v18"/><path d="m8 9 3 3-3 3"/>',
  panelOpen: '<rect width="18" height="18" x="3" y="3" rx="2"/><path d="M15 3v18"/><path d="m10 15-3-3 3-3"/>',
  power: '<path d="M12 2v10"/><path d="M18.4 6.6a9 9 0 1 1-12.77.04"/>',
  radio: '<path d="M4.9 19.1C1 15.2 1 8.8 4.9 4.9"/><path d="M7.8 16.2c-2.3-2.3-2.3-6.1 0-8.5"/><circle cx="12" cy="12" r="2"/><path d="M16.2 7.8c2.3 2.3 2.3 6.1 0 8.5"/><path d="M19.1 4.9C23 8.8 23 15.1 19.1 19"/>',
  trash: '<path d="M3 6h18"/><path d="M19 6v14c0 1-1 2-2 2H7c-1 0-2-1-2-2V6"/><path d="M8 6V4c0-1 1-2 2-2h4c1 0 2 1 2 2v2"/><line x1="10" x2="10" y1="11" y2="17"/><line x1="14" x2="14" y1="11" y2="17"/>',
  user: '<path d="M19 21v-2a4 4 0 0 0-4-4H9a4 4 0 0 0-4 4v2"/><circle cx="12" cy="7" r="4"/>',
  wifiOff: '<path d="M12 20h.01"/><path d="M8.5 16.429a5 5 0 0 1 7 0"/><path d="M5 12.859a10 10 0 0 1 5.17-2.69"/><path d="M19 12.859a10 10 0 0 0-2.007-1.523"/><path d="M2 8.82a15 15 0 0 1 4.177-2.643"/><path d="M22 8.82a15 15 0 0 0-11.288-3.764"/><path d="m2 2 20 20"/>',
  x: '<path d="M18 6 6 18"/><path d="m6 6 12 12"/>',
};

function icon(name, className = '') {
  return `<svg class="icon ${className}" viewBox="0 0 24 24" aria-hidden="true" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">${iconPaths[name] || ''}</svg>`;
}

function escapeHtml(value) {
  return String(value ?? '')
    .replaceAll('&', '&amp;')
    .replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;')
    .replaceAll('"', '&quot;')
    .replaceAll("'", '&#039;');
}

function hasBackend() {
  return Boolean(window.go?.main?.App?.Login);
}

function callBackend(method, ...args) {
  const fn = window.go?.main?.App?.[method];
  if (typeof fn !== 'function') {
    return Promise.reject(new Error('未连接到应用后端'));
  }
  return fn(...args);
}

function createPreviewBridge() {
  return {
    login: async (username) => ({
      success: true,
      message: '登录成功!',
      user: { id: 1, user: username || '演示用户' },
    }),
    register: async (username) => ({
      success: true,
      message: '注册成功!',
      user: { id: 1, user: username || '演示用户' },
    }),
    logout: async () => ({ success: true, message: '已退出登录' }),
    setInteractiveArea: () => Promise.resolve(),
    setBackgroundMode: () => Promise.resolve(),
  };
}

const bridge = previewMode
  ? createPreviewBridge()
  : {
      login: (username, password) => callBackend('Login', username, password),
      register: (username, password) => callBackend('Register', username, password),
      logout: () => callBackend('Logout'),
      setInteractiveArea: (x, y, width, height, visible) =>
        callBackend('SetOverlayInteractiveArea', x, y, width, height, visible),
      setBackgroundMode: (enabled) => callBackend('SetOverlayBackgroundMode', enabled),
    };

function timeLabel(value) {
  const match = String(value || '').match(/(\d{1,2}:\d{2})/);
  return match ? match[1] : '--:--';
}

function fullTimeLabel(value) {
  return String(value || '').trim() || '暂无时间';
}

function initialOf(name) {
  return Array.from(String(name || 'Q').trim())[0]?.toUpperCase() || 'Q';
}

function normalizeMessage(raw) {
  if (!raw || typeof raw !== 'object') return null;
  const title = String(raw.title ?? raw.Title ?? 'QQ 消息').trim();
  const body = String(raw.body ?? raw.Body ?? '').trim();
  if (!title && !body) return null;
  return {
    key: `${raw.notification_id ?? raw.NotificationID ?? raw.ID ?? Date.now()}-${Math.random()}`,
    notificationId: raw.notification_id ?? raw.NotificationID ?? '',
    time: raw.time ?? raw.Time ?? '',
    title: title || 'QQ 消息',
    body,
  };
}

function addIncomingMessage(raw) {
  const message = normalizeMessage(raw);
  if (!message) return;

  if (
    message.notificationId &&
    state.messages.some((item) => item.notificationId === message.notificationId)
  ) {
    return;
  }

  state.messages.unshift(message);
  state.messages = state.messages.slice(0, 60);
  state.receivedCount += 1;
  state.monitorError = '';

  const toast = { ...message, toastKey: `toast-${Date.now()}-${Math.random()}` };
  state.toasts.unshift(toast);
  state.toasts = state.toasts.slice(0, 4);
  render();

  window.setTimeout(() => {
    state.toasts = state.toasts.filter((item) => item.toastKey !== toast.toastKey);
    render();
  }, 9000);
}

function installBackendEvents() {
  if (previewMode || !window.runtime?.EventsOn) return;

  try {
    EventsOn('qq:new-message', (payload) => addIncomingMessage(payload));
    EventsOn('qq:monitor-error', (message) => {
      state.monitorError = String(message || 'QQ 通知监听暂时不可用');
      render();
    });
  } catch (error) {
    state.monitorError = error?.message || '事件监听初始化失败';
    render();
  }
}

function brandMarkup(compact = false) {
  return `
    <div class="brand ${compact ? 'brand--compact' : ''}">
      <span class="brand-glyph" aria-hidden="true">Q</span>
      ${compact ? '' : '<span class="brand-copy"><strong>QQ 弹幕</strong><small>桌面通知</small></span>'}
    </div>
  `;
}

function errorBannerMarkup(message, action = 'dismiss-error') {
  if (!message) return '';
  return `
    <div class="alert-bar" role="alert">
      ${icon('alert')}
      <span>${escapeHtml(message)}</span>
      <button class="icon-button icon-button--small" type="button" data-action="${action}" title="关闭提示" aria-label="关闭提示">
        ${icon('x')}
      </button>
    </div>
  `;
}

function authView() {
  const isRegister = state.authMode === 'register';
  const backendMissing = !previewMode && !hasBackend();

  return `
    <main class="overlay-layer auth-layer">
      <section class="control-panel auth-panel" id="control-panel" aria-label="登录面板">
        <header class="panel-header">
          ${brandMarkup()}
          <button class="icon-button" type="button" data-action="quit" title="退出程序" aria-label="退出程序">
            ${icon('power')}
          </button>
        </header>

        <div class="auth-heading">
          <span class="eyebrow">${isRegister ? '创建账户' : '欢迎回来'}</span>
          <h1>${isRegister ? '创建新账户' : '登录 QQ 弹幕'}</h1>
        </div>

        <div class="segmented-control" role="tablist" aria-label="账户操作">
          <button class="${!isRegister ? 'is-active' : ''}" type="button" role="tab" aria-selected="${!isRegister}" data-action="auth-mode" data-mode="login">登录</button>
          <button class="${isRegister ? 'is-active' : ''}" type="button" role="tab" aria-selected="${isRegister}" data-action="auth-mode" data-mode="register">注册</button>
        </div>

        ${errorBannerMarkup(state.authError)}
        ${backendMissing ? errorBannerMarkup('正在等待应用后端连接，请通过桌面应用启动。') : ''}

        <form class="auth-form" data-form="auth" novalidate>
          <label class="field">
            <span class="field-label">用户名</span>
            <span class="input-shell">
              ${icon('user')}
              <input
                name="username"
                type="text"
                value="${escapeHtml(state.form.username)}"
                autocomplete="username"
                placeholder="输入用户名"
                ${state.loading ? 'disabled' : ''}
              />
            </span>
          </label>

          <label class="field">
            <span class="field-label">密码</span>
            <span class="input-shell">
              ${icon('lock')}
              <input
                name="password"
                type="password"
                value="${escapeHtml(state.form.password)}"
                autocomplete="${isRegister ? 'new-password' : 'current-password'}"
                placeholder="${isRegister ? '8-20 位密码' : '输入密码'}"
                ${state.loading ? 'disabled' : ''}
              />
              <button class="reveal-button" type="button" data-action="toggle-password" data-target="password" title="显示密码" aria-label="显示密码">
                ${icon('eye')}
              </button>
            </span>
          </label>

          ${
            isRegister
              ? `
                <label class="field">
                  <span class="field-label">确认密码</span>
                  <span class="input-shell">
                    ${icon('lock')}
                    <input
                      name="confirmPassword"
                      type="password"
                      value="${escapeHtml(state.form.confirmPassword)}"
                      autocomplete="new-password"
                      placeholder="再次输入密码"
                      ${state.loading ? 'disabled' : ''}
                    />
                    <button class="reveal-button" type="button" data-action="toggle-password" data-target="confirmPassword" title="显示密码" aria-label="显示密码">
                      ${icon('eye')}
                    </button>
                  </span>
                </label>
                <p class="field-note">包含大写、小写字母、数字和特殊字符，长度 8-20 位。</p>
              `
              : ''
          }

          <button class="primary-button" type="submit" ${state.loading ? 'disabled' : ''}>
            ${state.loading ? '<span class="spinner" aria-hidden="true"></span>' : icon(isRegister ? 'user' : 'check')}
            <span>${state.loading ? '正在处理' : isRegister ? '创建账户' : '进入弹幕'}</span>
          </button>
        </form>

      </section>
    </main>
  `;
}

function messageItemMarkup(message) {
  return `
    <article class="message-item">
      <div class="message-meta">
        <span class="message-source">${escapeHtml(message.title)}</span>
        <time title="${escapeHtml(fullTimeLabel(message.time))}">${escapeHtml(timeLabel(message.time))}</time>
      </div>
      <p>${escapeHtml(message.body || '收到一条新通知')}</p>
    </article>
  `;
}

function toastMarkup(message) {
  return `
    <article class="message-toast" data-toast="${escapeHtml(message.toastKey)}">
      <span class="toast-accent" aria-hidden="true"></span>
      <div class="toast-body">
        <div class="toast-meta">
          <span>${escapeHtml(message.title)}</span>
          <time>${escapeHtml(timeLabel(message.time))}</time>
        </div>
        <p>${escapeHtml(message.body || '收到一条新通知')}</p>
      </div>
    </article>
  `;
}

function miniPanelMarkup() {
  return `
    <section class="mini-panel" id="control-panel" aria-label="QQ 弹幕控制台">
      <span class="brand-glyph brand-glyph--mini" aria-hidden="true">Q</span>
      <span class="mini-status"><span class="status-dot"></span>监听中</span>
      <span class="mini-count">${state.receivedCount}</span>
      <button class="icon-button icon-button--small" type="button" data-action="toggle-collapse" title="展开控制台" aria-label="展开控制台">
        ${icon('panelOpen')}
      </button>
    </section>
  `;
}

function dashboardView() {
  const userName = state.user?.name || state.form.username || '用户';
  const hasMessages = state.messages.length > 0;

  return `
    <main class="overlay-layer dashboard-layer">
      ${
        state.collapsed
          ? miniPanelMarkup()
          : `
            <section class="control-panel dashboard-panel" id="control-panel" aria-label="QQ 弹幕控制台">
              <header class="panel-header">
                ${brandMarkup()}
                <div class="header-actions">
                  <button class="icon-button" type="button" data-action="toggle-collapse" title="收起控制台" aria-label="收起控制台">
                    ${icon('panelClose')}
                  </button>
                  <button class="icon-button" type="button" data-action="quit" title="退出程序" aria-label="退出程序">
                    ${icon('power')}
                  </button>
                </div>
              </header>

              <section class="account-row">
                <span class="avatar" aria-hidden="true">${escapeHtml(initialOf(userName))}</span>
                <span class="account-copy">
                  <strong>${escapeHtml(userName)}</strong>
                  <small><span class="status-dot"></span>正在接收 QQ 通知</small>
                </span>
                <span class="live-badge"><span class="live-pulse"></span>LIVE</span>
              </section>

              ${errorBannerMarkup(state.monitorError, 'dismiss-monitor-error')}

              <section class="metrics-row" aria-label="本次会话统计">
                <div class="metric">
                  <span class="metric-label">本次消息</span>
                  <strong>${state.receivedCount}</strong>
                </div>
                <div class="metric">
                  <span class="metric-label">当前状态</span>
                  <strong class="metric-status">监听中</strong>
                </div>
              </section>

              <div class="feed-heading">
                <div>
                  <span class="eyebrow">消息流</span>
                  <h2>实时通知</h2>
                </div>
                <button class="icon-button" type="button" data-action="clear-messages" title="清空本次消息" aria-label="清空本次消息" ${hasMessages ? '' : 'disabled'}>
                  ${icon('trash')}
                </button>
              </div>

              <section class="message-list" aria-live="polite">
                ${
                  hasMessages
                    ? state.messages.map(messageItemMarkup).join('')
                    : `
                      <div class="empty-state">
                        <span class="empty-icon">${icon('radio')}</span>
                        <strong>等待新消息</strong>
                      </div>
                    `
                }
              </section>

              <footer class="dashboard-footer">
                <button class="setting-toggle" type="button" role="switch" aria-checked="${state.backgroundMode}" data-action="toggle-background">
                  <span class="switch-track" aria-hidden="true"><span class="switch-thumb"></span></span>
                  <span>
                    <strong>后台悬浮</strong>
                    <small>${state.backgroundMode ? '已隐藏任务栏入口' : '显示在任务栏中'}</small>
                  </span>
                </button>
                <button class="text-button" type="button" data-action="logout">
                  ${icon('logout')}
                  <span>退出登录</span>
                </button>
              </footer>
            </section>
          `
      }

      <aside class="toast-stack" aria-live="polite">
        ${state.toasts.map(toastMarkup).join('')}
      </aside>
    </main>
  `;
}

function render() {
  root.innerHTML = state.user ? dashboardView() : authView();
  syncOverlayArea();
}

let areaFrame = 0;
function syncOverlayArea() {
  window.cancelAnimationFrame(areaFrame);
  areaFrame = window.requestAnimationFrame(() => {
    const panel = document.querySelector('#control-panel');
    if (!panel) {
      bridge.setInteractiveArea(0, 0, 0, 0, false).catch(() => {});
      return;
    }

    const rect = panel.getBoundingClientRect();
    const padding = 6;
    const scale = window.devicePixelRatio || 1;
    const x = Math.max(0, Math.round((rect.left - padding) * scale));
    const y = Math.max(0, Math.round((rect.top - padding) * scale));
    const width = Math.round((rect.width + padding * 2) * scale);
    const height = Math.round((rect.height + padding * 2) * scale);
    bridge.setInteractiveArea(x, y, width, height, true).catch(() => {});
  });
}

function setAuthMode(mode) {
  state.authMode = mode;
  state.authError = '';
  state.form.password = '';
  state.form.confirmPassword = '';
  render();
  window.requestAnimationFrame(() => {
    root.querySelector('input[name="username"]')?.focus();
  });
}

function validateAuth() {
  const username = state.form.username.trim();
  const password = state.form.password;

  if (!username) return '请输入用户名';
  if (!password) return '请输入密码';

  if (state.authMode === 'register') {
    if (password.length < 8 || password.length > 20) {
      return '密码长度必须在 8-20 位之间';
    }
    if (!/[a-z]/.test(password) || !/[A-Z]/.test(password) || !/\d/.test(password)) {
      return '密码必须包含大写字母、小写字母和数字';
    }
    if (!/[^A-Za-z0-9]/.test(password)) {
      return '密码必须包含至少一个特殊字符';
    }
    if (password !== state.form.confirmPassword) {
      return '两次输入的密码不一致';
    }
  }

  return '';
}

async function submitAuth() {
  if (state.loading) return;

  const validationError = validateAuth();
  if (validationError) {
    state.authError = validationError;
    render();
    return;
  }

  state.loading = true;
  state.authError = '';
  render();

  try {
    const username = state.form.username.trim();
    const result =
      state.authMode === 'register'
        ? await bridge.register(username, state.form.password)
        : await bridge.login(username, state.form.password);

    if (!result?.success) {
      state.authError = result?.message || '操作未完成，请重试';
      return;
    }

    state.user = {
      id: result.user?.id ?? null,
      name: result.user?.user || username,
    };
    state.messages = [];
    state.toasts = [];
    state.receivedCount = 0;
    state.monitorError = '';
    state.collapsed = false;
    state.form.password = '';
    state.form.confirmPassword = '';
  } catch (error) {
    state.authError = error?.message || '无法连接应用后端';
  } finally {
    state.loading = false;
    render();
  }
}

async function logout() {
  try {
    await bridge.logout();
  } catch {
    // Local session cleanup still matters when the monitor has already stopped.
  }

  state.user = null;
  state.messages = [];
  state.toasts = [];
  state.receivedCount = 0;
  state.monitorError = '';
  state.collapsed = false;
  state.authMode = 'login';
  state.authError = '';
  state.form.username = '';
  state.form.password = '';
  state.form.confirmPassword = '';
  render();
}

async function toggleBackgroundMode() {
  const nextValue = !state.backgroundMode;
  try {
    await bridge.setBackgroundMode(nextValue);
    state.backgroundMode = nextValue;
    state.monitorError = '';
  } catch (error) {
    state.monitorError = error?.message || '无法切换后台模式';
  }
  render();
}

root.addEventListener('input', (event) => {
  const field = event.target.closest('input[name]');
  if (!field) return;
  state.form[field.name] = field.value;
});

root.addEventListener('submit', (event) => {
  if (!event.target.matches('[data-form="auth"]')) return;
  event.preventDefault();
  submitAuth();
});

root.addEventListener('click', (event) => {
  const target = event.target.closest('[data-action]');
  if (!target) return;

  const action = target.dataset.action;
  if (action === 'auth-mode') {
    setAuthMode(target.dataset.mode);
    return;
  }

  if (action === 'toggle-password') {
    const input = root.querySelector(`input[name="${target.dataset.target}"]`);
    if (!input) return;
    const showing = input.type === 'text';
    input.type = showing ? 'password' : 'text';
    target.innerHTML = icon(showing ? 'eye' : 'eyeOff');
    target.title = showing ? '显示密码' : '隐藏密码';
    target.setAttribute('aria-label', target.title);
    return;
  }

  if (action === 'dismiss-error') {
    state.authError = '';
    render();
    return;
  }

  if (action === 'dismiss-monitor-error') {
    state.monitorError = '';
    render();
    return;
  }

  if (action === 'clear-messages') {
    state.messages = [];
    state.receivedCount = 0;
    render();
    return;
  }

  if (action === 'toggle-collapse') {
    state.collapsed = !state.collapsed;
    render();
    return;
  }

  if (action === 'toggle-background') {
    toggleBackgroundMode();
    return;
  }

  if (action === 'logout') {
    logout();
    return;
  }

  if (action === 'quit') {
    if (previewMode) {
      state.user = null;
      render();
      return;
    }
    try {
      Quit();
    } catch (error) {
      console.error(error);
    }
  }
});

window.addEventListener('resize', syncOverlayArea);
window.visualViewport?.addEventListener('resize', syncOverlayArea);
document.fonts?.ready.then(syncOverlayArea);

if (previewMode) {
  document.body.classList.add('preview-surface');
  if (previewMode === 'dashboard') {
    state.user = { id: 1, name: '林知夏' };
    state.receivedCount = 18;
    state.messages = [
      {
        key: 'preview-1',
        notificationId: 'preview-1',
        title: '产品设计组',
        time: '2026-10-21 18:42:18',
        body: '新版弹幕样式已经更新，今晚一起看下实际效果。',
      },
      {
        key: 'preview-2',
        notificationId: 'preview-2',
        title: '陈默',
        time: '2026-10-21 18:39:06',
        body: '收到，我把演示视频放到共享盘了。',
      },
      {
        key: 'preview-3',
        notificationId: 'preview-3',
        title: '项目协作群',
        time: '2026-10-21 18:31:44',
        body: '本周评审改到周四下午三点，请提前准备材料。',
      },
    ];
    state.toasts = [{ ...state.messages[0], toastKey: 'preview-toast' }];
  }
}

installBackendEvents();
render();
