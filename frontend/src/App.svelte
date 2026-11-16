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

  type Tab = 'home' | 'servers' | 'custom';
  let activeTab: Tab = 'home';

  let status: domain.RouteState = new domain.RouteState({
    is_enabled: false,
    active_mode: 'hosts',
    active_target: 'all',
    active_provider: '',
    active_ip: '',
    latency_ms: 0,
    message: 'Загрузка статуса...'
  });

  let selectedTarget: string = 'all';
  let selectedProviderId: string = 'dns-ai';
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
    } catch (err: any) {
      errorMessage = String(err);
    }
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
    if (!newName || !newEndpoint) {
      errorMessage = 'Укажите название и адрес (DoH URL или IP)';
      return;
    }
    try {
      const p = new domain.Provider({
        id: 'custom-' + Date.now(),
        name: newName,
        kind: newEndpoint.startsWith('http') ? 'doh' : 'udp',
        endpoint: newEndpoint,
        fallback_ips: newIP ? [newIP] : [],
        gateways: newIP ? [newIP] : [],
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

  function getLatencyBadgeClass(ms: number): string {
    if (ms <= 0 || ms > 900) return 'badge-fail';
    if (ms < 80) return 'badge-fast';
    if (ms < 180) return 'badge-med';
    return 'badge-slow';
  }
</script>

<div class="window">
  <!-- Top Title Bar -->
  <header class="header">
    <div class="brand">
      <span class="logo-icon">⚡</span>
      <div class="brand-text">
        <span class="app-name">AIRoute</span>
        <span class="app-sub">AI Smart DNS & Hosts Optimizer</span>
      </div>
    </div>

    <nav class="nav">
      <button class="nav-btn" class:active={activeTab === 'home'} on:click={() => activeTab = 'home'}>Главная</button>
      <button class="nav-btn" class:active={activeTab === 'servers'} on:click={() => activeTab = 'servers'}>Серверы ({providers.length})</button>
      <button class="nav-btn" class:active={activeTab === 'custom'} on:click={() => activeTab = 'custom'}>+ Добавить DNS</button>
    </nav>
  </header>

  <!-- Error banner if any -->
  {#if errorMessage}
    <div class="error-banner">
      <span>⚠️ {errorMessage}</span>
      <button class="close-err" on:click={() => errorMessage = ''}>✕</button>
    </div>
  {/if}

  <!-- Content Views -->
  <main class="content">
    {#if activeTab === 'home'}
      <div class="home-view">
        <!-- Big Toggle Switch -->
        <div class="toggle-card">
          <button
            class="power-btn"
            class:active={status.is_enabled}
            disabled={isToggling}
            on:click={handleToggle}
          >
            <div class="power-icon">⏻</div>
            <div class="power-label">
              {#if isToggling}
                Применение...
              {:else if status.is_enabled}
                МАРШРУТИЗАЦИЯ ВКЛЮЧЕНА
              {:else}
                ВКЛЮЧИТЬ AI ROUTE
              {/if}
            </div>
            <div class="power-sub">
              {status.is_enabled ? 'Все запросы идут через чистый шлюз' : 'Нажмите для мгновенного прямого доступа'}
            </div>
          </button>
        </div>

        <!-- Target Selector & Info -->
        <div class="controls-row">
          <div class="form-group">
            <label for="target-select">Маршрутизировать для:</label>
            <select id="target-select" bind:value={selectedTarget} disabled={status.is_enabled}>
              <option value="all">🌟 Все нейросети сразу (Gemini, Claude, ChatGPT, Antigravity, Cursor)</option>
              <option value="gemini">💎 Только Google Gemini & AI Studio</option>
              <option value="claude">🧠 Только Claude AI & Anthropic</option>
              <option value="chatgpt">⚡ Только ChatGPT & OpenAI</option>
              <option value="antigravity">🛸 Только Antigravity IDE (CloudCode)</option>
              <option value="cursor">💻 Только Cursor IDE</option>
            </select>
          </div>

          <div class="form-group">
            <label for="provider-select">Основной провайдер:</label>
            <select id="provider-select" bind:value={selectedProviderId} disabled={status.is_enabled}>
              {#each providers as prov}
                <option value={prov.id}>{prov.name}</option>
              {/each}
            </select>
          </div>
        </div>

        <!-- Status Summary Bar -->
        <div class="status-summary">
          <div class="status-item">
            <span class="label">Режим:</span>
            <span class="val">Hosts File (Zero-RAM, без фоновых служб)</span>
          </div>
          <div class="status-item">
            <span class="label">Активный IP:</span>
            <span class="val">{status.active_ip || '—'}</span>
          </div>
          <div class="status-item">
            <span class="label">Провайдер:</span>
            <span class="val">{status.active_provider || 'Стандартный DNS провайдера'}</span>
          </div>
        </div>

        <!-- Ecosystem Grid -->
        <div class="ecosystem-grid">
          <div class="eco-card" class:active={status.is_enabled}>
            <div class="eco-header">
              <span class="eco-name">Google Gemini</span>
              <span class="eco-dot"></span>
            </div>
            <span class="eco-sub">gemini.google.com, aistudio</span>
          </div>
          <div class="eco-card" class:active={status.is_enabled}>
            <div class="eco-header">
              <span class="eco-name">Claude AI</span>
              <span class="eco-dot"></span>
            </div>
            <span class="eco-sub">claude.ai, api.anthropic.com</span>
          </div>
          <div class="eco-card" class:active={status.is_enabled}>
            <div class="eco-header">
              <span class="eco-name">ChatGPT & OpenAI</span>
              <span class="eco-dot"></span>
            </div>
            <span class="eco-sub">chatgpt.com, api.openai.com</span>
          </div>
          <div class="eco-card" class:active={status.is_enabled}>
            <div class="eco-header">
              <span class="eco-name">Antigravity IDE</span>
              <span class="eco-dot"></span>
            </div>
            <span class="eco-sub">cloudcode-pa, aicode.googleapis</span>
          </div>
        </div>
      </div>

    {:else if activeTab === 'servers'}
      <div class="servers-view">
        <div class="servers-toolbar">
          <span class="toolbar-title">Проверенные Smart DNS и DoH шлюзы</span>
          <button class="test-btn" disabled={isTesting} on:click={handleTestAll}>
            {isTesting ? '🔄 Тестирование TLS и пинга...' : '⚡ Замерить пинг всех серверов'}
          </button>
        </div>

        <div class="server-list">
          {#each providers as prov}
            <div class="server-card">
              <div class="server-main">
                <div class="server-title-row">
                  <span class="server-name">{prov.name}</span>
                  <span class="server-kind">{prov.kind.toUpperCase()}</span>
                  {#if prov.is_custom}
                    <span class="custom-badge">Пользовательский</span>
                  {/if}
                </div>
                <div class="server-endpoint">{prov.endpoint}</div>
                {#if prov.fallback_ips && prov.fallback_ips.length > 0}
                  <div class="server-ips">IP: {prov.fallback_ips.join(', ')}</div>
                {/if}
              </div>

              <div class="server-actions">
                {#if probeResults.some(r => r.provider_id === prov.id)}
                  {@const pResult = probeResults.find(r => r.provider_id === prov.id)}
                  <span class="ping-badge {getLatencyBadgeClass(pResult ? pResult.latency_ms : 0)}">
                    {pResult && pResult.is_clean ? `${pResult.latency_ms} ms` : 'Ошибка'}
                  </span>
                {/if}
                <button
                  class="select-btn"
                  class:selected={selectedProviderId === prov.id}
                  disabled={status.is_enabled}
                  on:click={() => selectedProviderId = prov.id}
                >
                  {selectedProviderId === prov.id ? '✓ Выбран' : 'Выбрать'}
                </button>
                {#if prov.is_custom}
                  <button class="del-btn" on:click={() => handleRemove(prov.id)}>🗑</button>
                {/if}
              </div>
            </div>
          {/each}
        </div>
      </div>

    {:else if activeTab === 'custom'}
      <div class="custom-view">
        <h3>Добавить пользовательский Smart DNS / DoH</h3>
        <p class="custom-hint">
          Вы можете добавить любой приватный DoH резолвер или IP-шлюз. Он сохранится в настройках и будет участвовать в бенчмарках.
        </p>

        <div class="custom-form">
          <div class="form-group">
            <label for="custom-name">Название:</label>
            <input id="custom-name" type="text" placeholder="Мой DoH Шлюз" bind:value={newName} />
          </div>

          <div class="form-group">
            <label for="custom-endpoint">Адрес (DoH URL или IP):</label>
            <input id="custom-endpoint" type="text" placeholder="https://example.com/dns-query" bind:value={newEndpoint} />
          </div>

          <div class="form-group">
            <label for="custom-ip">Резервный IP шлюза (опционально):</label>
            <input id="custom-ip" type="text" placeholder="192.168.1.1" bind:value={newIP} />
          </div>

          <button class="add-btn" on:click={handleAddCustom}>Сохранить DNS</button>
        </div>
      </div>
    {/if}
  </main>
</div>

<style>
  .window {
    display: flex;
    flex-direction: column;
    height: 100vh;
    background-color: var(--bg-main);
  }

  .header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 12px 20px;
    background-color: var(--bg-card);
    border-bottom: 1px solid var(--border);
  }

  .brand {
    display: flex;
    align-items: center;
    gap: 10px;
  }

  .logo-icon {
    font-size: 24px;
  }

  .brand-text {
    display: flex;
    flex-direction: column;
    text-align: left;
  }

  .app-name {
    font-size: 18px;
    font-weight: 700;
    color: var(--primary);
    letter-spacing: 0.5px;
  }

  .app-sub {
    font-size: 11px;
    color: var(--text-muted);
  }

  .nav {
    display: flex;
    gap: 8px;
  }

  .nav-btn {
    padding: 6px 14px;
    font-size: 13px;
    background-color: transparent;
    border: 1px solid transparent;
    color: var(--text-muted);
  }

  .nav-btn.active {
    background-color: var(--bg-card-hover);
    border-color: var(--border);
    color: var(--primary);
    font-weight: 600;
  }

  .error-banner {
    background-color: rgba(248, 81, 73, 0.15);
    border: 1px solid var(--accent-red);
    color: #ff7b72;
    padding: 8px 16px;
    display: flex;
    justify-content: space-between;
    align-items: center;
    font-size: 13px;
  }

  .close-err {
    background: none;
    border: none;
    color: inherit;
    font-size: 14px;
  }

  .content {
    flex: 1;
    overflow-y: auto;
    padding: 20px;
  }

  /* Home View */
  .home-view {
    display: flex;
    flex-direction: column;
    gap: 20px;
    max-width: 680px;
    margin: 0 auto;
  }

  .toggle-card {
    display: flex;
    justify-content: center;
    padding: 10px 0;
  }

  .power-btn {
    width: 240px;
    height: 240px;
    border-radius: 50%;
    background: linear-gradient(145deg, #161b22, #1c2129);
    border: 3px solid var(--border);
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 8px;
    cursor: pointer;
    box-shadow: 0 10px 30px rgba(0, 0, 0, 0.4);
    transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
  }

  .power-btn:hover:not(:disabled) {
    transform: scale(1.02);
    border-color: var(--primary);
  }

  .power-btn.active {
    border-color: var(--accent-green);
    box-shadow: 0 0 40px var(--accent-green-glow);
    background: linear-gradient(145deg, #102618, #161b22);
  }

  .power-icon {
    font-size: 52px;
    color: var(--text-muted);
    transition: color 0.3s;
  }

  .power-btn.active .power-icon {
    color: #3fb950;
  }

  .power-label {
    font-size: 14px;
    font-weight: 700;
    letter-spacing: 0.5px;
  }

  .power-btn.active .power-label {
    color: #3fb950;
  }

  .power-sub {
    font-size: 11px;
    color: var(--text-muted);
    max-width: 170px;
    text-align: center;
  }

  .controls-row {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 16px;
  }

  .form-group {
    display: flex;
    flex-direction: column;
    gap: 6px;
    text-align: left;
  }

  .form-group label {
    font-size: 12px;
    color: var(--text-muted);
  }

  .form-group select, .form-group input {
    padding: 8px 12px;
    font-size: 13px;
  }

  .status-summary {
    display: flex;
    justify-content: space-between;
    background-color: var(--bg-card);
    border: 1px solid var(--border);
    border-radius: 8px;
    padding: 12px 16px;
    font-size: 12px;
  }

  .status-item {
    display: flex;
    flex-direction: column;
    gap: 4px;
    text-align: left;
  }

  .status-item .label {
    color: var(--text-muted);
  }

  .status-item .val {
    font-weight: 600;
    color: var(--text-main);
  }

  .ecosystem-grid {
    display: grid;
    grid-template-columns: repeat(4, 1fr);
    gap: 12px;
  }

  .eco-card {
    background-color: var(--bg-card);
    border: 1px solid var(--border);
    border-radius: 8px;
    padding: 12px;
    text-align: left;
    transition: border-color 0.2s;
  }

  .eco-card.active {
    border-color: #3fb950;
  }

  .eco-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 4px;
  }

  .eco-name {
    font-size: 12px;
    font-weight: 600;
  }

  .eco-dot {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    background-color: #484f58;
  }

  .eco-card.active .eco-dot {
    background-color: #3fb950;
    box-shadow: 0 0 8px rgba(63, 185, 80, 0.6);
  }

  .eco-sub {
    font-size: 10px;
    color: var(--text-muted);
    display: block;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  /* Servers View */
  .servers-view {
    max-width: 680px;
    margin: 0 auto;
    display: flex;
    flex-direction: column;
    gap: 16px;
  }

  .servers-toolbar {
    display: flex;
    justify-content: space-between;
    align-items: center;
  }

  .toolbar-title {
    font-size: 14px;
    font-weight: 600;
    color: var(--text-muted);
  }

  .test-btn {
    padding: 8px 16px;
    background-color: var(--primary);
    color: #0d1117;
    font-weight: 700;
    font-size: 13px;
    border: none;
  }

  .server-list {
    display: flex;
    flex-direction: column;
    gap: 10px;
  }

  .server-card {
    background-color: var(--bg-card);
    border: 1px solid var(--border);
    border-radius: 8px;
    padding: 12px 16px;
    display: flex;
    justify-content: space-between;
    align-items: center;
    text-align: left;
  }

  .server-title-row {
    display: flex;
    align-items: center;
    gap: 8px;
    margin-bottom: 4px;
  }

  .server-name {
    font-weight: 600;
    font-size: 14px;
  }

  .server-kind {
    font-size: 10px;
    background-color: var(--border);
    padding: 2px 6px;
    border-radius: 4px;
    font-weight: 700;
  }

  .custom-badge {
    font-size: 10px;
    background-color: rgba(88, 166, 255, 0.2);
    color: var(--primary);
    padding: 2px 6px;
    border-radius: 4px;
  }

  .server-endpoint {
    font-size: 12px;
    color: var(--text-muted);
    font-family: monospace;
  }

  .server-ips {
    font-size: 11px;
    color: #6e7681;
    margin-top: 2px;
  }

  .server-actions {
    display: flex;
    align-items: center;
    gap: 10px;
  }

  .ping-badge {
    font-size: 12px;
    font-weight: 700;
    padding: 4px 8px;
    border-radius: 4px;
  }

  .badge-fast {
    background-color: rgba(46, 160, 67, 0.2);
    color: #3fb950;
  }

  .badge-med {
    background-color: rgba(210, 153, 34, 0.2);
    color: #d29922;
  }

  .badge-slow {
    background-color: rgba(248, 81, 73, 0.2);
    color: #ff7b72;
  }

  .badge-fail {
    background-color: #21262d;
    color: #8b949e;
  }

  .select-btn {
    padding: 6px 12px;
    font-size: 12px;
    background-color: var(--bg-card-hover);
    color: var(--text-main);
  }

  .select-btn.selected {
    background-color: #238636;
    border-color: #2ea043;
    font-weight: 600;
  }

  .del-btn {
    background: none;
    border: none;
    padding: 4px;
    font-size: 14px;
  }

  /* Custom View */
  .custom-view {
    max-width: 500px;
    margin: 20px auto;
    text-align: left;
  }

  .custom-hint {
    font-size: 13px;
    color: var(--text-muted);
    margin-bottom: 20px;
    line-height: 1.5;
  }

  .custom-form {
    display: flex;
    flex-direction: column;
    gap: 16px;
  }

  .add-btn {
    padding: 10px;
    background-color: var(--primary);
    color: #0d1117;
    font-weight: 700;
    border: none;
    margin-top: 8px;
  }
</style>
