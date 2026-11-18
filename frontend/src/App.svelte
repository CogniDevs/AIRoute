<script lang="ts">
  import { onMount } from 'svelte';
  import {
    GetStatus,
    ToggleRoute,
    TestProviders,
    GetProviders,
    AddCustomProvider,
    RemoveCustomProvider
  } from '../wailsjs/go/main/App.js';
  import { domain } from '../wailsjs/go/models';
  import logoImg from './assets/images/logo-universal.png';

  type Tab = 'connect' | 'servers' | 'settings';
  let activeTab: Tab = 'connect';

  let status: domain.RouteState = new domain.RouteState({
    is_enabled: false,
    active_mode: 'hosts',
    active_target: 'all',
    active_provider: '',
    active_ip: '',
    latency_ms: 0,
    message: 'Инициализация...'
  });

  let selectedTarget: string = 'all';
  let selectedProviderId: string = 'dns-ai-cf';
  let providers: domain.Provider[] = [];
  let probeResults: domain.ProbeResult[] = [];
  let isTesting: boolean = false;
  let isToggling: boolean = false;
  let errorMessage: string = '';

  // Custom provider form
  let newName: string = '';
  let newEndpoint: string = '';
  let newIP: string = '';

  onMount(async () => {
    await refreshState();
    await loadProviders();
  });

  async function refreshState() {
    try {
      status = await GetStatus();
      if (status.active_target) {
        selectedTarget = status.active_target;
      }
    } catch (err: any) {
      errorMessage = String(err);
    }
  }

  async function loadProviders() {
    try {
      providers = await GetProviders();
      if (providers.length > 0 && !providers.some(p => p.id === selectedProviderId)) {
        selectedProviderId = providers[0].id;
      }
    } catch (err: any) {
      errorMessage = String(err);
    }
  }

  function getBestProbe(providerId: string): domain.ProbeResult | undefined {
    const list = probeResults.filter(r => r.provider_id === providerId);
    if (list.length === 0) return undefined;
    const cleanList = list.filter(r => r.is_clean);
    if (cleanList.length > 0) {
      cleanList.sort((a, b) => (a.latency_ms || 0) - (b.latency_ms || 0));
      return cleanList[0];
    }
    return list[0];
  }

  async function handleToggle() {
    isToggling = true;
    errorMessage = '';
    try {
      const willEnable = !status.is_enabled;
      status = await ToggleRoute(willEnable, selectedTarget, selectedProviderId);
    } catch (err: any) {
      errorMessage = String(err);
    } finally {
      isToggling = false;
    }
  }

  async function handleTestAll() {
    isTesting = true;
    errorMessage = '';
    try {
      probeResults = await TestProviders(selectedTarget);
    } catch (err: any) {
      errorMessage = String(err);
    } finally {
      isTesting = false;
    }
  }

  async function handleAddCustom() {
    if (!newName.trim() || !newEndpoint.trim()) {
      errorMessage = 'Введите название и адрес сервера';
      return;
    }
    try {
      const ep = newEndpoint.trim();
      const p = new domain.Provider({
        id: 'custom-' + Date.now(),
        name: newName.trim(),
        kind: ep.startsWith('http') ? 'doh' : 'udp',
        endpoint: ep,
        fallback_ips: newIP.trim() ? [newIP.trim()] : [],
        gateways: newIP.trim() ? [newIP.trim()] : [],
        enabled: true,
        is_custom: true
      });
      await AddCustomProvider(p);
      newName = '';
      newEndpoint = '';
      newIP = '';
      await loadProviders();
      activeTab = 'servers';
    } catch (err: any) {
      errorMessage = String(err);
    }
  }

  async function handleRemove(id: string) {
    try {
      await RemoveCustomProvider(id);
      await loadProviders();
    } catch (err: any) {
      errorMessage = String(err);
    }
  }

  function getTargetLabel(target: string): string {
    switch (target) {
      case 'all': return 'Все нейросети (Gemini, Claude, ChatGPT, Antigravity, Cursor)';
      case 'gemini': return 'Google Gemini & AI Studio';
      case 'claude': return 'Claude AI & Anthropic';
      case 'chatgpt': return 'ChatGPT & OpenAI';
      case 'antigravity': return 'Antigravity IDE (CloudCode)';
      case 'cursor': return 'Cursor IDE';
      default: return 'Все нейросети';
    }
  }
</script>

<div class="app-layout">
  <!-- Minimalist Header -->
  <header class="navbar">
    <div class="brand">
      <img class="brand-logo" src={logoImg} alt="AIRoute Logo" />
      <div class="brand-meta">
        <span class="brand-title">AIRoute</span>
        <span class="brand-badge">PRO</span>
      </div>
    </div>

    <nav class="nav-links">
      <button class="nav-item" class:active={activeTab === 'connect'} on:click={() => activeTab = 'connect'}>
        <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2">
          <path d="M18.36 6.64a9 9 0 1 1-12.73 0"></path>
          <line x1="12" y1="2" x2="12" y2="12"></line>
        </svg>
        <span>Подключение</span>
      </button>

      <button class="nav-item" class:active={activeTab === 'servers'} on:click={() => activeTab = 'servers'}>
        <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2">
          <rect x="2" y="2" width="20" height="8" rx="2" ry="2"></rect>
          <rect x="2" y="14" width="20" height="8" rx="2" ry="2"></rect>
          <line x1="6" y1="6" x2="6.01" y2="6"></line>
          <line x1="6" y1="18" x2="6.01" y2="18"></line>
        </svg>
        <span>Серверы</span>
        <span class="count-pill">{providers.length}</span>
      </button>

      <button class="nav-item" class:active={activeTab === 'settings'} on:click={() => activeTab = 'settings'}>
        <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2">
          <circle cx="12" cy="12" r="3"></circle>
          <path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1 0 2.83 2 2 0 0 1-2.83 0l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-2 2 2 2 0 0 1-2-2v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83 0 2 2 0 0 1 0-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1-2-2 2 2 0 0 1 2-2h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 0-2.83 2 2 0 0 1 2.83 0l.06.06a1.65 1.65 0 0 0 1.82.33H9a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 2-2 2 2 0 0 1 2 2v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 0 2 2 0 0 1 0 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 2 2 2 2 0 0 1-2 2h-.09a1.65 1.65 0 0 0-1.51 1z"></path>
        </svg>
        <span>Настройки</span>
      </button>
    </nav>
  </header>

  <!-- Notification Bar -->
  {#if errorMessage}
    <div class="notification-bar error">
      <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2">
        <circle cx="12" cy="12" r="10"></circle>
        <line x1="12" y1="8" x2="12" y2="12"></line>
        <line x1="12" y1="16" x2="12.01" y2="16"></line>
      </svg>
      <span>{errorMessage}</span>
      <button class="icon-close" on:click={() => errorMessage = ''}>✕</button>
    </div>
  {/if}

  <!-- Viewport Container -->
  <main class="view-body">
    <!-- 1. Connection View (Minimal, Beautiful) -->
    {#if activeTab === 'connect'}
      <div class="connect-view">
        <!-- Power Button -->
        <div class="power-container">
          <button
            class="main-toggle"
            class:active={status.is_enabled}
            disabled={isToggling}
            on:click={handleToggle}
          >
            <div class="toggle-glow"></div>
            <svg class="toggle-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5">
              <path d="M18.36 6.64a9 9 0 1 1-12.73 0"></path>
              <line x1="12" y1="2" x2="12" y2="12"></line>
            </svg>
            <span class="toggle-state">
              {#if isToggling}
                Применение...
              {:else if status.is_enabled}
                ПОДКЛЮЧЕНО
              {:else}
                ПОДКЛЮЧИТЬ
              {/if}
            </span>
          </button>
        </div>

        <!-- Connection Badge -->
        <div class="connection-status">
          <span class="status-dot" class:online={status.is_enabled}></span>
          <div class="status-text">
            {#if status.is_enabled}
              <div class="status-main">Маршрутизация активна через {status.active_provider || 'Smart DNS'}</div>
              <div class="status-sub">Шлюз: {status.active_ip || 'Активен'}{status.latency_ms > 0 ? ` (${status.latency_ms} мс)` : ''} • Режим: Hosts (Zero-RAM)</div>
            {:else}
              <div class="status-main">Прямое подключение к сети</div>
              <div class="status-sub">Нажмите кнопку выше для мгновенного обхода ограничений</div>
            {/if}
          </div>
        </div>

        <!-- Scope Card -->
        <div class="scope-card">
          <div class="scope-info">
            <span class="scope-label">Охват сервисов</span>
            <span class="scope-val">{getTargetLabel(selectedTarget)}</span>
          </div>
          <button class="scope-btn" on:click={() => activeTab = 'settings'}>
            Изменить
          </button>
        </div>
      </div>

    <!-- 2. Servers View -->
    {:else if activeTab === 'servers'}
      <div class="servers-view">
        <div class="view-toolbar">
          <div class="toolbar-info">
            <h2>Доступные шлюзы</h2>
            <p>Пул проверенных Smart DNS и DoH серверов</p>
          </div>
          <button class="btn-primary" disabled={isTesting} on:click={handleTestAll}>
            <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2" class:rotating={isTesting}>
              <polyline points="23 4 23 10 17 10"></polyline>
              <polyline points="1 20 1 14 7 14"></polyline>
              <path d="M3.51 9a9 9 0 0 1 14.85-3.36L23 10M1 14l4.64 4.36A9 9 0 0 0 20.49 15"></path>
            </svg>
            <span>{isTesting ? 'Проверка...' : 'Замерить пинг'}</span>
          </button>
        </div>

        <div class="provider-list">
          {#each providers as prov}
            {@const res = getBestProbe(prov.id)}
            <div class="provider-row" class:is-selected={selectedProviderId === prov.id}>
              <div class="provider-lead">
                <div class="provider-title">
                  <span class="provider-name">{prov.name}</span>
                  <span class="tag-kind">{prov.kind.toUpperCase()}</span>
                  {#if prov.is_custom}
                    <span class="tag-custom">Свой</span>
                  {/if}
                </div>
                <span class="provider-endpoint">{prov.endpoint}</span>
                {#if prov.fallback_ips && prov.fallback_ips.length > 0}
                  <span class="provider-ip-hint">IP: {prov.fallback_ips.join(', ')}</span>
                {/if}
              </div>

              <div class="provider-meta">
                {#if res}
                  <span
                    class="latency-badge"
                    class:fast={res.is_clean && res.latency_ms < 80}
                    class:med={res.is_clean && res.latency_ms >= 80}
                    class:bad={!res.is_clean}
                    title={`${res.ip ? res.ip + ' — ' : ''}${res.reason || ''}`}
                  >
                    {res.is_clean ? `${res.latency_ms} ms` : 'Ошибка'}
                  </span>
                {/if}

                <button
                  class="btn-select"
                  class:active={selectedProviderId === prov.id}
                  disabled={status.is_enabled}
                  on:click={() => selectedProviderId = prov.id}
                >
                  {#if selectedProviderId === prov.id}
                    <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2.5">
                      <polyline points="20 6 9 17 4 12"></polyline>
                    </svg>
                    <span>Выбран</span>
                  {:else}
                    <span>Выбрать</span>
                  {/if}
                </button>

                {#if prov.is_custom}
                  <button class="btn-del" on:click={() => handleRemove(prov.id)} title="Удалить">
                    <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2">
                      <polyline points="3 6 5 6 21 6"></polyline>
                      <path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"></path>
                    </svg>
                  </button>
                {/if}
              </div>
            </div>
          {/each}
        </div>
      </div>

    <!-- 3. Settings View -->
    {:else if activeTab === 'settings'}
      <div class="settings-view">
        <div class="view-toolbar">
          <div class="toolbar-info">
            <h2>Параметры маршрутизации</h2>
            <p>Тонкая настройка работы Smart DNS и шлюзов</p>
          </div>
        </div>

        <div class="settings-form">
          <!-- Target scope -->
          <div class="setting-card">
            <div class="setting-desc">
              <span class="setting-title">Целевые сервисы</span>
              <span class="setting-sub">Выберите, для каких нейросетей перенаправлять запросы</span>
            </div>
            <select bind:value={selectedTarget} disabled={status.is_enabled}>
              <option value="all">Все нейросети (Gemini, Claude, ChatGPT, Antigravity, Cursor)</option>
              <option value="gemini">Google Gemini & AI Studio</option>
              <option value="claude">Claude AI & Anthropic</option>
              <option value="chatgpt">ChatGPT & OpenAI</option>
              <option value="antigravity">Antigravity IDE (CloudCode)</option>
              <option value="cursor">Cursor IDE</option>
            </select>
          </div>

          <!-- DNS Provider -->
          <div class="setting-card">
            <div class="setting-desc">
              <span class="setting-title">Предпочтительный сервер</span>
              <span class="setting-sub">Шлюз по умолчанию при включении</span>
            </div>
            <select bind:value={selectedProviderId} disabled={status.is_enabled}>
              {#each providers as prov}
                <option value={prov.id}>{prov.name}</option>
              {/each}
            </select>
          </div>

          <!-- Add Custom Server Section -->
          <div class="setting-card form-box">
            <div class="setting-desc">
              <span class="setting-title">Добавить свой DNS шлюз</span>
              <span class="setting-sub">Поддержка пользовательских DoH (HTTPS) или IP резолверов</span>
            </div>

            <div class="add-grid">
              <input type="text" placeholder="Название (напр. Мой DoH)" bind:value={newName} />
              <input type="text" placeholder="Адрес (https://ru.dns-ai.ru/...-&#123;?dns&#125; или IP)" bind:value={newEndpoint} />
              <input type="text" placeholder="Резервный IP (опционально)" bind:value={newIP} />
              <button class="btn-primary" on:click={handleAddCustom}>
                <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2">
                  <line x1="12" y1="5" x2="12" y2="19"></line>
                  <line x1="5" y1="12" x2="19" y2="12"></line>
                </svg>
                <span>Добавить</span>
              </button>
            </div>
          </div>
        </div>
      </div>
    {/if}
  </main>
</div>

<style>
  .app-layout {
    display: flex;
    flex-direction: column;
    height: 100vh;
    width: 100%;
    overflow: hidden;
  }

  /* Header */
  .navbar {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 12px 24px;
    background-color: var(--bg-surface);
    border-bottom: 1px solid var(--border);
  }

  .brand {
    display: flex;
    align-items: center;
    gap: 10px;
  }

  .brand-logo {
    width: 26px;
    height: 26px;
    border-radius: 6px;
    object-fit: contain;
  }

  .brand-meta {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .brand-title {
    font-size: 17px;
    font-weight: 700;
    color: var(--text-primary);
    letter-spacing: -0.3px;
  }

  .brand-badge {
    font-size: 10px;
    font-weight: 700;
    padding: 2px 6px;
    background-color: var(--primary-glow);
    color: var(--primary);
    border-radius: 4px;
    border: 1px solid rgba(59, 130, 246, 0.3);
  }

  .nav-links {
    display: flex;
    gap: 6px;
    background-color: var(--bg-app);
    padding: 4px;
    border-radius: 10px;
    border: 1px solid var(--border);
  }

  .nav-item {
    padding: 6px 14px;
    font-size: 13px;
    color: var(--text-secondary);
    border-radius: 6px;
  }

  .nav-item:hover {
    color: var(--text-primary);
  }

  .nav-item.active {
    background-color: var(--bg-surface);
    color: var(--primary);
    box-shadow: 0 1px 3px rgba(0, 0, 0, 0.2);
  }

  .count-pill {
    font-size: 11px;
    background: var(--bg-card);
    padding: 1px 6px;
    border-radius: 10px;
    color: var(--text-muted);
  }

  /* Notifications */
  .notification-bar {
    padding: 10px 20px;
    display: flex;
    align-items: center;
    gap: 10px;
    font-size: 13px;
  }

  .notification-bar.error {
    background-color: var(--danger-bg);
    color: #fca5a5;
    border-bottom: 1px solid rgba(239, 68, 68, 0.3);
  }

  .icon-close {
    margin-left: auto;
    color: inherit;
    font-size: 14px;
  }

  /* Body */
  .view-body {
    flex: 1;
    overflow-y: auto;
    overflow-x: hidden;
    padding: 24px;
  }

  /* Connect View */
  .connect-view {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    max-width: 520px;
    margin: 20px auto 0;
    gap: 28px;
  }

  .power-container {
    position: relative;
    display: flex;
    justify-content: center;
    align-items: center;
  }

  .main-toggle {
    position: relative;
    width: 210px;
    height: 210px;
    border-radius: 50%;
    background: radial-gradient(circle at 30% 30%, #1e293b, #0f172a);
    border: 3px solid var(--border);
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 12px;
    box-shadow: 0 12px 36px rgba(0, 0, 0, 0.4);
    transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
  }

  .main-toggle:hover:not(:disabled) {
    transform: scale(1.02);
    border-color: var(--primary);
  }

  .main-toggle.active {
    border-color: var(--success);
    background: radial-gradient(circle at 30% 30%, #064e3b, #022c22);
    box-shadow: 0 0 50px var(--success-glow);
  }

  .toggle-icon {
    width: 44px;
    height: 44px;
    color: var(--text-muted);
    transition: color 0.3s;
  }

  .main-toggle.active .toggle-icon {
    color: #34d399;
  }

  .toggle-state {
    font-size: 13px;
    font-weight: 700;
    letter-spacing: 1px;
    color: var(--text-secondary);
  }

  .main-toggle.active .toggle-state {
    color: #34d399;
  }

  .connection-status {
    display: flex;
    align-items: center;
    gap: 14px;
    background-color: var(--bg-surface);
    border: 1px solid var(--border);
    padding: 14px 20px;
    border-radius: 12px;
    width: 100%;
  }

  .status-dot {
    width: 10px;
    height: 10px;
    border-radius: 50%;
    background-color: var(--text-muted);
    flex-shrink: 0;
  }

  .status-dot.online {
    background-color: var(--success);
    box-shadow: 0 0 10px var(--success);
  }

  .status-text {
    display: flex;
    flex-direction: column;
    gap: 2px;
    text-align: left;
  }

  .status-main {
    font-size: 14px;
    font-weight: 600;
    color: var(--text-primary);
  }

  .status-sub {
    font-size: 12px;
    color: var(--text-secondary);
  }

  .scope-card {
    display: flex;
    justify-content: space-between;
    align-items: center;
    background-color: var(--bg-card);
    border: 1px solid var(--border);
    padding: 12px 18px;
    border-radius: 10px;
    width: 100%;
  }

  .scope-info {
    display: flex;
    flex-direction: column;
    gap: 2px;
    text-align: left;
    max-width: 360px;
  }

  .scope-label {
    font-size: 11px;
    color: var(--text-muted);
    text-transform: uppercase;
    font-weight: 600;
  }

  .scope-val {
    font-size: 13px;
    color: var(--text-primary);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .scope-btn {
    padding: 6px 14px;
    font-size: 12px;
    color: var(--primary);
    background-color: var(--primary-glow);
    border: 1px solid rgba(59, 130, 246, 0.3);
  }

  /* Common Toolbar */
  .view-toolbar {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 20px;
  }

  .toolbar-info h2 {
    font-size: 18px;
    font-weight: 700;
    margin-bottom: 4px;
    text-align: left;
  }

  .toolbar-info p {
    font-size: 13px;
    color: var(--text-secondary);
    text-align: left;
  }

  .btn-primary {
    background-color: var(--primary);
    color: #ffffff;
    padding: 9px 18px;
    font-size: 13px;
  }

  .btn-primary:hover:not(:disabled) {
    background-color: var(--primary-hover);
  }

  .rotating {
    animation: spin 1s linear infinite;
  }

  @keyframes spin {
    100% { transform: rotate(360deg); }
  }

  /* Servers List */
  .servers-view, .settings-view {
    max-width: 680px;
    margin: 0 auto;
  }

  .provider-list {
    display: flex;
    flex-direction: column;
    gap: 10px;
  }

  .provider-row {
    background-color: var(--bg-surface);
    border: 1px solid var(--border);
    border-radius: 10px;
    padding: 14px 18px;
    display: flex;
    justify-content: space-between;
    align-items: center;
    transition: border-color 0.15s ease;
  }

  .provider-row.is-selected {
    border-color: var(--primary);
    background-color: var(--bg-surface-hover);
  }

  .provider-lead {
    display: flex;
    flex-direction: column;
    gap: 4px;
    text-align: left;
  }

  .provider-title {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .provider-name {
    font-size: 14px;
    font-weight: 600;
  }

  .tag-kind {
    font-size: 10px;
    font-weight: 700;
    padding: 2px 6px;
    background-color: var(--bg-card);
    border: 1px solid var(--border);
    border-radius: 4px;
    color: var(--text-muted);
  }

  .tag-custom {
    font-size: 10px;
    font-weight: 600;
    padding: 2px 6px;
    background-color: var(--primary-glow);
    border: 1px solid rgba(59, 130, 246, 0.3);
    color: var(--primary);
    border-radius: 4px;
  }

  .provider-endpoint {
    font-size: 12px;
    color: var(--text-muted);
    font-family: monospace;
  }

  .provider-ip-hint {
    font-size: 11px;
    color: var(--text-muted);
  }

  .provider-meta {
    display: flex;
    align-items: center;
    gap: 12px;
  }

  .latency-badge {
    font-size: 12px;
    font-weight: 700;
    padding: 4px 8px;
    border-radius: 6px;
  }

  .latency-badge.fast {
    background-color: var(--success-bg);
    color: var(--success);
  }

  .latency-badge.med {
    background-color: var(--warning-bg);
    color: var(--warning);
  }

  .latency-badge.bad {
    background-color: var(--danger-bg);
    color: var(--danger);
  }

  .btn-select {
    padding: 6px 14px;
    font-size: 12px;
    background-color: var(--bg-card);
    border: 1px solid var(--border);
    color: var(--text-primary);
  }

  .btn-select.active {
    background-color: var(--success-bg);
    border-color: var(--success);
    color: var(--success);
  }

  .btn-del {
    color: var(--text-muted);
    padding: 6px;
  }

  .btn-del:hover {
    color: var(--danger);
  }

  /* Settings Form */
  .settings-form {
    display: flex;
    flex-direction: column;
    gap: 16px;
  }

  .setting-card {
    background-color: var(--bg-surface);
    border: 1px solid var(--border);
    border-radius: 12px;
    padding: 18px 20px;
    display: flex;
    flex-direction: column;
    gap: 12px;
    text-align: left;
  }

  .setting-desc {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .setting-title {
    font-size: 14px;
    font-weight: 600;
  }

  .setting-sub {
    font-size: 12px;
    color: var(--text-secondary);
  }

  .add-grid {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 10px;
  }

  .add-grid input:first-child {
    grid-column: span 2;
  }
</style>
