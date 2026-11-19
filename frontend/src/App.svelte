<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import {
    GetStatus,
    GetProviders,
    TestProviders,
    ApplyAutoBest,
    ApplyRoutes,
    ClearRoutes,
    VerifyActive,
    ToggleProxy,
    AddCustomProvider,
    UpdateProvider,
    ToggleProvider,
    RemoveCustomProvider,
    ResetDefaultProviders,
    SyncCatalog
  } from '../wailsjs/go/main/App.js';
  import { EventsOn, EventsOff } from '../wailsjs/runtime/runtime.js';
  import { domain } from '../wailsjs/go/models';
  import logoImg from './assets/images/logo-universal.png';

  type Tab = 'routes' | 'servers' | 'settings';
  let activeTab: Tab = 'routes';

  const ALL_SERVICES = [
    { id: 'gemini', name: 'Google Gemini', desc: 'Gemini Web, AI Studio, Generative Language API' },
    { id: 'claude', name: 'Anthropic Claude', desc: 'Claude AI, Anthropic API, Claude CDN' },
    { id: 'chatgpt', name: 'OpenAI ChatGPT', desc: 'ChatGPT, OpenAI API, Sora, Auth0' },
    { id: 'antigravity', name: 'Antigravity IDE', desc: 'CloudCode PA, JetSki WebChannel, AICode' },
    { id: 'cursor', name: 'Cursor IDE', desc: 'Cursor API, Repo42, Cloudflare Edge' }
  ];

  let status: domain.RouteState = new domain.RouteState({
    is_enabled: false,
    active_mode: 'hosts',
    active_target: 'all',
    active_provider: '',
    active_ip: '',
    latency_ms: 0,
    message: 'Инициализация...',
    routes: {},
    patched_count: 0,
    total_count: 5,
    hosts_path: '',
    proxy_running: false,
    proxy_addr: ''
  });

  let providers: domain.Provider[] = [];
  let probeResults: domain.ProbeResult[] = [];
  let probingProviders = new Set<string>();

  let isScanning: boolean = false;
  let isApplying: boolean = false;
  let isVerifying: boolean = false;
  let isTogglingProxy: boolean = false;
  let isSyncingCatalog: boolean = false;
  let errorMessage: string = '';
  let successNotice: string = '';

  // Filters
  let hideBrokenServers: boolean = true;
  let hideBrokenInSelectors: boolean = true;

  // Local selections per service
  let targetEnabled: Record<string, boolean> = {
    gemini: true,
    claude: true,
    chatgpt: true,
    antigravity: true,
    cursor: true
  };

  let targetProvider: Record<string, string> = {
    gemini: 'auto',
    claude: 'auto',
    chatgpt: 'auto',
    antigravity: 'auto',
    cursor: 'auto'
  };

  // Edit Server Modal state
  let isEditModalOpen: boolean = false;
  let editingProvider: domain.Provider | null = null;
  let editName: string = '';
  let editEndpoint: string = '';
  let editGateways: string = '';
  let editKind: string = 'doh';
  let editEnabled: boolean = true;

  // New Custom provider form
  let newName: string = '';
  let newEndpoint: string = '';
  let newIP: string = '';

  // Svelte 3 reactive computations
  $: probeMatrix = (() => {
    const map: Record<string, domain.ProbeResult> = {};
    for (const r of probeResults) {
      const key = `${r.provider_id}:${r.target}`;
      const existing = map[key];
      if (!existing) {
        map[key] = r;
      } else if (r.is_clean && (!existing.is_clean || r.latency_ms < existing.latency_ms)) {
        map[key] = r;
      }
    }
    return map;
  })();

  $: serverStats = (() => {
    const map: Record<string, {
      workingCount: number;
      workingServices: string[];
      avgLat: number;
      minLat: number;
      tested: boolean;
      clean: boolean;
      reason: string;
    }> = {};

    for (const p of providers) {
      let latSum = 0;
      let workingCount = 0;
      const workingServices: string[] = [];
      let minLat = 999999;
      let firstReason = '';
      let tested = false;

      for (const s of ALL_SERVICES) {
        const r = probeMatrix[`${p.id}:${s.id}`];
        if (r) {
          tested = true;
          if (r.is_clean) {
            workingCount++;
            workingServices.push(s.name);
            latSum += r.latency_ms;
            if (r.latency_ms < minLat) minLat = r.latency_ms;
          } else if (!firstReason) {
            firstReason = r.reason;
          }
        }
      }

      const avgLat = workingCount > 0 ? Math.round(latSum / workingCount) : 0;
      map[p.id] = {
        workingCount,
        workingServices,
        avgLat,
        minLat: minLat === 999999 ? 0 : minLat,
        tested,
        clean: workingCount > 0,
        reason: firstReason || (workingCount > 0 ? 'Рабочий' : 'Не проверен')
      };
    }
    return map;
  })();

  $: sortedProviders = [...providers].sort((a, b) => {
    const sa = serverStats[a.id] || { workingCount: 0, avgLat: 999999 };
    const sb = serverStats[b.id] || { workingCount: 0, avgLat: 999999 };
    if (sb.workingCount !== sa.workingCount) {
      return sb.workingCount - sa.workingCount;
    }
    if (sa.avgLat > 0 && sb.avgLat > 0) {
      return sa.avgLat - sb.avgLat;
    }
    if (sa.avgLat > 0) return -1;
    if (sb.avgLat > 0) return 1;
    return 0;
  });

  $: filteredProviders = hideBrokenServers
    ? sortedProviders.filter(p => {
        const st = serverStats[p.id];
        return !st || !st.tested || st.workingCount > 0;
      })
    : sortedProviders;

  $: hiddenServersCount = sortedProviders.length - filteredProviders.length;

  $: bestGlobalByService = (() => {
    const map: Record<string, domain.ProbeResult | undefined> = {};
    for (const s of ALL_SERVICES) {
      let best: domain.ProbeResult | undefined = undefined;
      for (const p of providers) {
        if (!p.enabled) continue;
        const r = probeMatrix[`${p.id}:${s.id}`];
        if (r && r.is_clean) {
          if (!best || r.latency_ms < best.latency_ms) {
            best = r;
          }
        }
      }
      map[s.id] = best;
    }
    return map;
  })();

  onMount(async () => {
    await refreshState();
    await loadProviders();

    EventsOn('probe:start', (ids: string[]) => {
      probingProviders = new Set(ids);
      isScanning = true;
    });

    EventsOn('probe:provider', (data: { provider_id: string; results: domain.ProbeResult[] }) => {
      probingProviders.delete(data.provider_id);
      probingProviders = new Set(probingProviders);

      const other = probeResults.filter(r => r.provider_id !== data.provider_id);
      probeResults = [...other, ...data.results];
    });

    EventsOn('probe:done', (data: { results: domain.ProbeResult[] }) => {
      probingProviders = new Set();
      isScanning = false;
      if (data && data.results) {
        probeResults = data.results;
      }
    });
  });

  onDestroy(() => {
    EventsOff('probe:start');
    EventsOff('probe:provider');
    EventsOff('probe:done');
  });

  async function refreshState() {
    try {
      status = await GetStatus();
      if (status.routes) {
        for (const s of ALL_SERVICES) {
          const r = status.routes[s.id];
          if (r) {
            targetEnabled[s.id] = r.enabled;
            if (r.provider_id) {
              targetProvider[s.id] = r.provider_id;
            }
          }
        }
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

  function getEnabledTargets(): string[] {
    return ALL_SERVICES.filter(s => targetEnabled[s.id]).map(s => s.id);
  }

  async function handleAutoBest() {
    isApplying = true;
    errorMessage = '';
    successNotice = '';
    try {
      const enabled = getEnabledTargets();
      if (enabled.length === 0) {
        errorMessage = 'Включите хотя бы один сервис для маршрутизации';
        return;
      }
      status = await ApplyAutoBest(enabled);
      successNotice = 'Лучшие шлюзы подобраны и записаны в hosts';
      setTimeout(() => successNotice = '', 4000);
    } catch (err: any) {
      errorMessage = String(err);
    } finally {
      isApplying = false;
    }
  }

  async function handleApplyManual() {
    isApplying = true;
    errorMessage = '';
    successNotice = '';
    try {
      const enabled = getEnabledTargets();
      status = await ApplyRoutes(targetProvider, enabled);
      successNotice = 'Маршруты успешно обновлены в hosts';
      setTimeout(() => successNotice = '', 4000);
    } catch (err: any) {
      errorMessage = String(err);
    } finally {
      isApplying = false;
    }
  }

  async function handleClearHosts() {
    isApplying = true;
    errorMessage = '';
    successNotice = '';
    try {
      status = await ClearRoutes();
      successNotice = 'Файл hosts успешно очищен от правил AIRoute';
      setTimeout(() => successNotice = '', 4000);
    } catch (err: any) {
      errorMessage = String(err);
    } finally {
      isApplying = false;
    }
  }

  async function handleVerifyActive() {
    isVerifying = true;
    errorMessage = '';
    successNotice = '';
    try {
      // Backend updates fresh RTT directly into route state and hosts metadata
      status = await VerifyActive();
      successNotice = 'Пинг активных шлюзов обновлён';
      setTimeout(() => successNotice = '', 3000);
    } catch (err: any) {
      errorMessage = String(err);
    } finally {
      isVerifying = false;
    }
  }

  async function handleTestAll() {
    isScanning = true;
    errorMessage = '';
    try {
      probeResults = await TestProviders('all');
    } catch (err: any) {
      errorMessage = String(err);
    } finally {
      isScanning = false;
    }
  }

  async function handleApplyProviderToAll(provId: string) {
    isApplying = true;
    errorMessage = '';
    successNotice = '';
    try {
      for (const s of ALL_SERVICES) {
        targetProvider[s.id] = provId;
      }
      const enabled = getEnabledTargets();
      status = await ApplyRoutes(targetProvider, enabled);
      successNotice = 'Все сервисы переключены на выбранный DNS';
      setTimeout(() => successNotice = '', 4000);
    } catch (err: any) {
      errorMessage = String(err);
    } finally {
      isApplying = false;
    }
  }

  async function handleToggleProxy() {
    isTogglingProxy = true;
    errorMessage = '';
    try {
      const next = !status.proxy_running;
      status = await ToggleProxy(next, 2080);
    } catch (err: any) {
      errorMessage = String(err);
    } finally {
      isTogglingProxy = false;
    }
  }

  function copyProxyEnv() {
    navigator.clipboard.writeText('http://127.0.0.1:2080');
    successNotice = 'Адрес прокси http://127.0.0.1:2080 скопирован в буфер';
    setTimeout(() => successNotice = '', 3000);
  }

  // Provider Editing
  function openEditModal(prov: domain.Provider) {
    editingProvider = prov;
    editName = prov.name;
    editEndpoint = prov.endpoint;
    editGateways = (prov.gateways || []).join(', ');
    editKind = prov.kind || 'doh';
    editEnabled = prov.enabled;
    isEditModalOpen = true;
  }

  function closeEditModal() {
    isEditModalOpen = false;
    editingProvider = null;
  }

  async function handleSaveEditedProvider() {
    if (!editingProvider || !editName.trim() || !editEndpoint.trim()) {
      errorMessage = 'Укажите название и адрес сервера';
      return;
    }
    try {
      const gws = editGateways.split(',').map(s => s.trim()).filter(Boolean);
      const updated = new domain.Provider({
        id: editingProvider.id,
        name: editName.trim(),
        kind: editKind,
        endpoint: editEndpoint.trim(),
        fallback_ips: editingProvider.fallback_ips || [],
        gateways: gws,
        enabled: editEnabled,
        is_custom: editingProvider.is_custom
      });

      await UpdateProvider(updated);
      await loadProviders();
      closeEditModal();
      successNotice = `Сервер "${updated.name}" сохранен`;
      setTimeout(() => successNotice = '', 3000);
    } catch (err: any) {
      errorMessage = String(err);
    }
  }

  async function handleToggleProvider(id: string, current: boolean) {
    try {
      await ToggleProvider(id, !current);
      await loadProviders();
    } catch (err: any) {
      errorMessage = String(err);
    }
  }

  async function handleResetProviders() {
    if (!confirm('Сбросить все серверы к стандартным заводским настройкам?')) return;
    try {
      await ResetDefaultProviders();
      await loadProviders();
      successNotice = 'Серверы сброшены к значениям по умолчанию';
      setTimeout(() => successNotice = '', 3000);
    } catch (err: any) {
      errorMessage = String(err);
    }
  }

  async function handleSyncCatalog() {
    isSyncingCatalog = true;
    errorMessage = '';
    successNotice = '';
    try {
      const count = await SyncCatalog();
      await loadProviders();
      successNotice = `Синхронизировано ${count} серверов из онлайн-каталога`;
      setTimeout(() => successNotice = '', 4000);
    } catch (err: any) {
      errorMessage = String(err);
    } finally {
      isSyncingCatalog = false;
    }
  }

  function copyProviderEndpoint(prov: domain.Provider) {
    const textToCopy = prov.endpoint || (prov.gateways && prov.gateways[0]) || '';
    if (textToCopy) {
      navigator.clipboard.writeText(textToCopy);
      successNotice = `Адрес "${textToCopy}" скопирован в буфер`;
      setTimeout(() => successNotice = '', 3000);
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
      successNotice = `Сервер "${p.name}" добавлен`;
      setTimeout(() => successNotice = '', 3000);
    } catch (err: any) {
      errorMessage = String(err);
    }
  }

  async function handleRemove(id: string) {
    try {
      await RemoveCustomProvider(id);
      await loadProviders();
      successNotice = 'Сервер удален';
      setTimeout(() => successNotice = '', 3000);
    } catch (err: any) {
      errorMessage = String(err);
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
        <span class="brand-badge">Hosts Patcher</span>
      </div>
    </div>

    <nav class="nav-links">
      <button class="nav-item" class:active={activeTab === 'routes'} on:click={() => activeTab = 'routes'}>
        <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2">
          <polyline points="22 12 18 12 15 21 9 3 6 12 2 12"></polyline>
        </svg>
        <span>Маршруты</span>
      </button>

      <button class="nav-item" class:active={activeTab === 'servers'} on:click={() => activeTab = 'servers'}>
        <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2">
          <rect x="2" y="2" width="20" height="8" rx="2" ry="2"></rect>
          <rect x="2" y="14" width="20" height="8" rx="2" ry="2"></rect>
          <line x1="6" y1="6" x2="6.01" y2="6"></line>
          <line x1="6" y1="18" x2="6.01" y2="18"></line>
        </svg>
        <span>DNS-серверы</span>
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

  <!-- Notifications -->
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

  {#if successNotice}
    <div class="notification-bar success">
      <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2">
        <path d="M22 11.08V12a10 10 0 1 1-5.93-9.14"></path>
        <polyline points="22 4 12 14.01 9 11.01"></polyline>
      </svg>
      <span>{successNotice}</span>
      <button class="icon-close" on:click={() => successNotice = ''}>✕</button>
    </div>
  {/if}

  <!-- Main Content -->
  <main class="view-body">
    <!-- 1. ROUTES TAB (Main smart patcher control) -->
    {#if activeTab === 'routes'}
      <div class="routes-view">
        <!-- Hosts Status Hero Card -->
        <div class="status-hero">
          <div class="hero-left">
            <div class="hero-title-row">
              <span class="status-indicator" class:active={status.patched_count > 0}></span>
              <span class="hero-title">
                {#if status.patched_count > 0}
                  Пропатчено {status.patched_count} из {status.total_count} сервисов
                {:else}
                  Правила не применены — стандартный DNS
                {/if}
              </span>
            </div>
            <div class="hero-path" title={status.hosts_path}>
              <span>hosts:</span> {status.hosts_path || 'Определяется...'}
            </div>
          </div>

          <div class="hero-actions">
            <button class="btn-action primary" disabled={isApplying || isScanning} on:click={handleAutoBest}>
              {#if isApplying}
                <span class="mini-spinner"></span>
                <span>Применение...</span>
              {:else}
                <svg viewBox="0 0 24 24" width="15" height="15" fill="none" stroke="currentColor" stroke-width="2">
                  <polygon points="13 2 3 14 12 14 11 22 21 10 12 10 13 2"></polygon>
                </svg>
                <span>Подобрать лучшие и применить</span>
              {/if}
            </button>

            <button class="btn-action secondary" disabled={isApplying} on:click={handleApplyManual}>
              <span>Применить</span>
            </button>

            {#if status.patched_count > 0}
              <button class="btn-action secondary" disabled={isVerifying} on:click={handleVerifyActive} title="Перепроверить текущие IP в hosts и обновить пинг">
                {#if isVerifying}
                  <span class="mini-spinner"></span>
                  <span>Замер...</span>
                {:else}
                  <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2">
                    <polyline points="20 6 9 17 4 12"></polyline>
                  </svg>
                  <span>Проверить пинг</span>
                {/if}
              </button>

              <button class="btn-action danger" disabled={isApplying} on:click={handleClearHosts}>
                <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2">
                  <polyline points="3 6 5 6 21 6"></polyline>
                  <path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"></path>
                </svg>
                <span>Откатить hosts</span>
              </button>
            {/if}
          </div>
        </div>

        <!-- Filter toggle for selectors -->
        <div class="routes-filter-row">
          <label class="toggle-label">
            <input type="checkbox" bind:checked={hideBrokenInSelectors} />
            <span>Скрывать заведомо нерабочие шлюзы в списках выбора</span>
          </label>
        </div>

        <!-- Service Cards Grid -->
        <div class="service-list">
          {#each ALL_SERVICES as srv}
            <div class="service-card" class:disabled={!targetEnabled[srv.id]}>
              <div class="service-lead">
                <label class="custom-checkbox">
                  <input type="checkbox" bind:checked={targetEnabled[srv.id]} />
                  <span class="checkmark"></span>
                </label>

                <div class="service-info">
                  <div class="service-name-row">
                    <span class="service-name">{srv.name}</span>
                    {#if status.routes && status.routes[srv.id] && status.routes[srv.id].enabled}
                      <span class="badge-active">hosts активен</span>
                    {/if}
                  </div>
                  <span class="service-domains">{srv.desc}</span>

                  <!-- Current Applied Info -->
                  <div class="current-route-info">
                    {#if status.routes && status.routes[srv.id] && status.routes[srv.id].enabled && status.routes[srv.id].ip}
                      <span class="route-ip">{status.routes[srv.id].ip}</span>
                      <span class="sep">•</span>
                      <span class="route-prov">{status.routes[srv.id].provider_name || 'Smart DNS'}</span>
                      {#if status.routes[srv.id].latency_ms > 0}
                        <span class="sep">•</span>
                        <span class="route-lat">{status.routes[srv.id].latency_ms} ms</span>
                      {/if}
                    {:else}
                      <span class="route-inactive">Маршрутизация выключена (прямое соединение)</span>
                    {/if}
                  </div>
                </div>
              </div>

              <!-- Provider Selector Dropdown -->
              <div class="service-selector">
                <span class="sel-label">Шлюз:</span>
                <select bind:value={targetProvider[srv.id]} disabled={!targetEnabled[srv.id]}>
                  <option value="auto">
                    Авто (лучший: {bestGlobalByService[srv.id]?.provider_name ? `${bestGlobalByService[srv.id]?.provider_name} — ${bestGlobalByService[srv.id]?.latency_ms} ms` : 'будет подобран'})
                  </option>

                  <optgroup label="Рабочие серверы">
                    {#each providers as prov}
                      {#if prov.enabled && (!probeMatrix[`${prov.id}:${srv.id}`] || probeMatrix[`${prov.id}:${srv.id}`].is_clean)}
                        <option value={prov.id}>
                          {prov.name}
                          {#if probeMatrix[`${prov.id}:${srv.id}`]}
                            — {probeMatrix[`${prov.id}:${srv.id}`].latency_ms} ms
                          {/if}
                        </option>
                      {/if}
                    {/each}
                  </optgroup>

                  {#if !hideBrokenInSelectors}
                    <optgroup label="Недоступные для этого сервиса">
                      {#each providers as prov}
                        {#if prov.enabled && probeMatrix[`${prov.id}:${srv.id}`] && !probeMatrix[`${prov.id}:${srv.id}`].is_clean}
                          <option value={prov.id} disabled>
                            {prov.name} — недоступен ({probeMatrix[`${prov.id}:${srv.id}`].reason || 'ошибка'})
                          </option>
                        {/if}
                      {/each}
                    </optgroup>
                  {/if}
                </select>
              </div>
            </div>
          {/each}
        </div>

        <!-- Alternative: Local HTTP Proxy Mode Box -->
        <div class="proxy-banner">
          <div class="proxy-info">
            <div class="proxy-header">
              <span class="proxy-title">Режим локального прокси</span>
              <span class="tag-noroot">Без прав root / UAC</span>
            </div>
            <p class="proxy-desc">
              Запускает локальный HTTP CONNECT прокси на <code>127.0.0.1:2080</code>. Маршрутизирует запросы нейросетей через чистые шлюзы без изменения файла hosts.
            </p>
          </div>

          <div class="proxy-actions">
            {#if status.proxy_running}
              <button class="btn-copy" on:click={copyProxyEnv} title="Скопировать адрес">
                <span>127.0.0.1:2080</span>
                <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2">
                  <rect x="9" y="9" width="13" height="13" rx="2" ry="2"></rect>
                  <path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"></path>
                </svg>
              </button>
            {/if}

            <button
              class="btn-toggle-proxy"
              class:active={status.proxy_running}
              disabled={isTogglingProxy}
              on:click={handleToggleProxy}
            >
              {#if isTogglingProxy}
                <span class="mini-spinner"></span>
              {:else if status.proxy_running}
                <span>Остановить прокси</span>
              {:else}
                <span>Запустить прокси</span>
              {/if}
            </button>
          </div>
        </div>
      </div>

    <!-- 2. SERVERS TAB (DNS latency scanner & quick switch) -->
    {:else if activeTab === 'servers'}
      <div class="servers-view">
        <div class="view-toolbar">
          <div class="toolbar-info">
            <h2>Доступные DNS-шлюзы</h2>
            <p>Ранжирование по покрытию сервисов и среднему пингу</p>
          </div>
          <div class="toolbar-controls">
            <label class="toggle-label">
              <input type="checkbox" bind:checked={hideBrokenServers} />
              <span>Скрыть нерабочие{#if hiddenServersCount > 0} ({hiddenServersCount}){/if}</span>
            </label>
            <button class="btn-action secondary" disabled={isSyncingCatalog} on:click={handleSyncCatalog} title="Обновить пул серверов из онлайн-каталога dns.quic.best">
              {#if isSyncingCatalog}
                <span class="mini-spinner"></span>
                <span>Обновление...</span>
              {:else}
                <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2">
                  <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"></path>
                  <polyline points="7 10 12 15 17 10"></polyline>
                  <line x1="12" y1="15" x2="12" y2="3"></line>
                </svg>
                <span>Обновить из сети</span>
              {/if}
            </button>
            <button class="btn-primary" disabled={isScanning} on:click={handleTestAll}>
              <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2" class:rotating={isScanning}>
                <polyline points="23 4 23 10 17 10"></polyline>
                <polyline points="1 20 1 14 7 14"></polyline>
                <path d="M3.51 9a9 9 0 0 1 14.85-3.36L23 10M1 14l4.64 4.36A9 9 0 0 0 20.49 15"></path>
              </svg>
              <span>{isScanning ? 'Замер пинга...' : 'Замерить пинг всех'}</span>
            </button>
          </div>
        </div>

        <div class="provider-list">
          {#each filteredProviders as prov}
            <div class="provider-row" class:is-disabled={!prov.enabled}>
              <div class="provider-lead">
                <div class="provider-title">
                  <span class="provider-name">{prov.name}</span>
                  <span class="tag-kind">{prov.kind.toUpperCase()}</span>
                  {#if prov.is_custom}
                    <span class="tag-custom">Свой</span>
                  {/if}
                  {#if !prov.enabled}
                    <span class="tag-disabled">Отключен</span>
                  {/if}

                  <!-- Coverage Badge -->
                  {#if serverStats[prov.id] && serverStats[prov.id].tested}
                    <span
                      class="coverage-badge"
                      class:full={serverStats[prov.id].workingCount >= 4}
                      class:partial={serverStats[prov.id].workingCount > 0 && serverStats[prov.id].workingCount < 4}
                      class:none={serverStats[prov.id].workingCount === 0}
                    >
                      {serverStats[prov.id].workingCount}/5 сервисов
                    </span>
                  {/if}
                </div>

                <span class="provider-endpoint">{prov.endpoint}</span>

                <!-- Service Badges -->
                <div class="service-badges">
                  {#each ALL_SERVICES as s}
                    <span
                      class="s-badge"
                      class:ok={probeMatrix[`${prov.id}:${s.id}`]?.is_clean}
                      class:fail={probeMatrix[`${prov.id}:${s.id}`] && !probeMatrix[`${prov.id}:${s.id}`]?.is_clean}
                      class:pending={probingProviders.has(prov.id)}
                      title={`${s.name}: ${probeMatrix[`${prov.id}:${s.id}`]?.reason || (probingProviders.has(prov.id) ? 'Замер...' : 'Не проверен')}`}
                    >
                      {s.id.slice(0, 3).toUpperCase()}: 
                      {#if probingProviders.has(prov.id)}
                        ...
                      {:else if probeMatrix[`${prov.id}:${s.id}`]?.is_clean}
                        {probeMatrix[`${prov.id}:${s.id}`].latency_ms}ms
                      {:else if probeMatrix[`${prov.id}:${s.id}`]}
                        ✗
                      {:else}
                        -
                      {/if}
                    </span>
                  {/each}
                </div>
              </div>

              <div class="provider-meta">
                {#if probingProviders.has(prov.id)}
                  <div class="badge-probing">
                    <span class="mini-spinner"></span>
                    <span>Пинг...</span>
                  </div>
                {:else if serverStats[prov.id] && serverStats[prov.id].workingCount > 0}
                  <div class="latency-container" title={`Средний пинг рабочих сервисов: ${serverStats[prov.id].avgLat} ms (мин: ${serverStats[prov.id].minLat} ms)`}>
                    <span
                      class="latency-badge"
                      class:fast={serverStats[prov.id].workingCount >= 3 && serverStats[prov.id].avgLat < 100}
                      class:med={serverStats[prov.id].workingCount >= 3 && serverStats[prov.id].avgLat >= 100}
                      class:warn={serverStats[prov.id].workingCount < 3}
                    >
                      ~{serverStats[prov.id].avgLat} ms
                    </span>
                  </div>
                {:else if serverStats[prov.id] && serverStats[prov.id].tested && serverStats[prov.id].workingCount === 0}
                  <span class="latency-badge bad" title={serverStats[prov.id].reason}>
                    Недоступен
                  </span>
                {/if}

                <button
                  class="btn-apply-all"
                  disabled={isApplying || !prov.enabled}
                  on:click={() => handleApplyProviderToAll(prov.id)}
                  title="Назначить этот сервер для всех активных сервисов и записать hosts"
                >
                  <svg viewBox="0 0 24 24" width="13" height="13" fill="none" stroke="currentColor" stroke-width="2">
                    <polyline points="20 6 9 17 4 12"></polyline>
                  </svg>
                  <span>Применить ко всем</span>
                </button>

                <!-- Copy Endpoint Button -->
                <button class="btn-icon" on:click={() => copyProviderEndpoint(prov)} title="Скопировать DoH / IP адрес сервера">
                  <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2">
                    <rect x="9" y="9" width="13" height="13" rx="2" ry="2"></rect>
                    <path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"></path>
                  </svg>
                </button>

                <!-- Edit Server Button -->
                <button class="btn-icon" on:click={() => openEditModal(prov)} title="Редактировать сервер">
                  <svg viewBox="0 0 24 24" width="15" height="15" fill="none" stroke="currentColor" stroke-width="2">
                    <path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7"></path>
                    <path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z"></path>
                  </svg>
                </button>

                {#if prov.is_custom}
                  <button class="btn-del" on:click={() => handleRemove(prov.id)} title="Удалить">
                    <svg viewBox="0 0 24 24" width="15" height="15" fill="none" stroke="currentColor" stroke-width="2">
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

    <!-- 3. SETTINGS TAB (Custom servers & info) -->
    {:else if activeTab === 'settings'}
      <div class="settings-view">
        <div class="view-toolbar">
          <div class="toolbar-info">
            <h2>Настройки и свои серверы</h2>
            <p>Управление пользовательскими резолверами и системными параметрами</p>
          </div>
          <div class="toolbar-controls">
            <button class="btn-action secondary" disabled={isSyncingCatalog} on:click={handleSyncCatalog}>
              <span>Синхронизировать с онлайн-каталогом</span>
            </button>
            <button class="btn-action danger" on:click={handleResetProviders}>
              <span>Сбросить к заводским</span>
            </button>
          </div>
        </div>

        <div class="settings-form">
          <!-- Add Custom Server Section -->
          <div class="setting-card form-box">
            <div class="setting-desc">
              <span class="setting-title">Добавить свой DNS шлюз</span>
              <span class="setting-sub">Поддержка пользовательских DoH (HTTPS) или IP резолверов</span>
            </div>

            <div class="add-grid">
              <input type="text" placeholder="Название (напр. Мой Zapret DoH)" bind:value={newName} />
              <input type="text" placeholder="Адрес (https://.../dns-query или IP)" bind:value={newEndpoint} />
              <input type="text" placeholder="IP шлюза (опционально)" bind:value={newIP} />
              <button class="btn-primary" on:click={handleAddCustom}>
                <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2">
                  <line x1="12" y1="5" x2="12" y2="19"></line>
                  <line x1="5" y1="12" x2="19" y2="12"></line>
                </svg>
                <span>Добавить</span>
              </button>
            </div>
          </div>

          <!-- Architecture Info -->
          <div class="setting-card info-card">
            <div class="setting-desc">
              <span class="setting-title">О модели работы AIRoute</span>
              <span class="setting-sub">
                В отличие от традиционных VPN, AIRoute работает по принципу <b>Smart Hosts Patcher</b>.
                При нажатии «Применить» приложение находит чистые шлюзы и вносит точечные записи в системный файл <code>hosts</code>.
                После этого приложение не тратит оперативную память и не нагружает процессор (Zero-RAM), а трафик идёт напрямую на максимальной скорости интернет-провайдера.
              </span>
            </div>
          </div>
        </div>
      </div>
    {/if}
  </main>

  <!-- Edit Provider Modal -->
  {#if isEditModalOpen && editingProvider}
    <div class="modal-backdrop" on:click|self={closeEditModal} on:keydown={(e) => e.key === 'Escape' && closeEditModal()}>
      <div class="modal-content">
        <div class="modal-header">
          <span class="modal-title">Редактирование сервера</span>
          <button class="icon-close" on:click={closeEditModal}>✕</button>
        </div>

        <div class="modal-body">
          <div class="form-group">
            <label class="form-label" for="edit-name">Название сервера:</label>
            <input id="edit-name" type="text" bind:value={editName} placeholder="Название сервера" />
          </div>

          <div class="form-group">
            <label class="form-label" for="edit-ep">DoH / DNS эндпоинт:</label>
            <input id="edit-ep" type="text" bind:value={editEndpoint} placeholder="https://domain.tld/dns-query" />
          </div>

          <div class="form-group">
            <label class="form-label" for="edit-gw">Шлюзы / Прямые IP (через запятую):</label>
            <input id="edit-gw" type="text" bind:value={editGateways} placeholder="5.129.221.64, 5.129.221.79" />
          </div>

          <div class="form-row">
            <div class="form-group half">
              <label class="form-label" for="edit-kind">Протокол:</label>
              <select id="edit-kind" bind:value={editKind}>
                <option value="doh">DoH (DNS over HTTPS)</option>
                <option value="udp">UDP Standard DNS</option>
              </select>
            </div>

            <div class="form-group half checkbox-group">
              <label class="custom-checkbox">
                <input type="checkbox" bind:checked={editEnabled} />
                <span class="checkmark"></span>
                <span class="cb-label">Сервер включен</span>
              </label>
            </div>
          </div>
        </div>

        <div class="modal-footer">
          <button class="btn-action secondary" on:click={closeEditModal}>Отмена</button>
          <button class="btn-action primary" on:click={handleSaveEditedProvider}>Сохранить</button>
        </div>
      </div>
    </div>
  {/if}
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
    color: var(--primary);
    background-color: var(--primary-glow);
    padding: 2px 7px;
    border-radius: 4px;
    text-transform: uppercase;
    letter-spacing: 0.5px;
  }

  .nav-links {
    display: flex;
    gap: 6px;
    background-color: var(--bg-app);
    padding: 3px;
    border-radius: 8px;
    border: 1px solid var(--border);
  }

  .nav-item {
    color: var(--text-secondary);
    padding: 6px 14px;
    border-radius: 6px;
    font-size: 13px;
    cursor: pointer;
    background: transparent;
  }

  .nav-item:hover {
    color: var(--text-primary);
    background-color: var(--bg-surface-hover);
  }

  .nav-item.active {
    color: #fff;
    background-color: var(--primary);
  }

  .count-pill {
    background-color: rgba(255, 255, 255, 0.15);
    font-size: 11px;
    padding: 1px 6px;
    border-radius: 10px;
  }

  /* Notifications */
  .notification-bar {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 9px 20px;
    font-size: 13px;
    border-bottom: 1px solid transparent;
  }

  .notification-bar.error {
    background-color: var(--danger-bg);
    color: var(--danger);
    border-bottom-color: rgba(239, 68, 68, 0.2);
  }

  .notification-bar.success {
    background-color: var(--success-bg);
    color: var(--success);
    border-bottom-color: rgba(16, 185, 129, 0.2);
  }

  .icon-close {
    margin-left: auto;
    color: inherit;
    opacity: 0.7;
    font-size: 14px;
  }
  .icon-close:hover { opacity: 1; }

  /* View Body */
  .view-body {
    flex: 1;
    overflow-y: auto;
    padding: 20px 24px;
  }

  /* Status Hero */
  .status-hero {
    background-color: var(--bg-surface);
    border: 1px solid var(--border);
    border-radius: 12px;
    padding: 16px 20px;
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 14px;
    gap: 16px;
  }

  .hero-title-row {
    display: flex;
    align-items: center;
    gap: 10px;
    margin-bottom: 4px;
  }

  .status-indicator {
    width: 10px;
    height: 10px;
    border-radius: 50%;
    background-color: var(--text-muted);
  }

  .status-indicator.active {
    background-color: var(--success);
    box-shadow: 0 0 10px var(--success-glow);
  }

  .hero-title {
    font-size: 15px;
    font-weight: 600;
    color: var(--text-primary);
  }

  .hero-path {
    font-size: 12px;
    color: var(--text-muted);
    font-family: monospace;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    max-width: 380px;
  }

  .hero-path span {
    color: var(--text-secondary);
  }

  .hero-actions {
    display: flex;
    align-items: center;
    gap: 8px;
    flex-wrap: wrap;
  }

  .btn-action {
    padding: 8px 14px;
    font-size: 13px;
    border-radius: 8px;
  }

  .btn-action.primary {
    background-color: var(--primary);
    color: #fff;
  }
  .btn-action.primary:hover { background-color: var(--primary-hover); }

  .btn-action.secondary {
    background-color: var(--bg-card);
    color: var(--text-primary);
    border: 1px solid var(--border);
  }
  .btn-action.secondary:hover { background-color: var(--bg-surface-hover); }

  .btn-action.danger {
    background-color: var(--danger-bg);
    color: var(--danger);
    border: 1px solid rgba(239, 68, 68, 0.2);
  }
  .btn-action.danger:hover { background-color: rgba(239, 68, 68, 0.2); }

  .routes-filter-row {
    margin-bottom: 12px;
    display: flex;
    align-items: center;
    font-size: 12px;
    color: var(--text-muted);
  }

  /* Service Cards */
  .service-list {
    display: flex;
    flex-direction: column;
    gap: 10px;
    margin-bottom: 20px;
  }

  .service-card {
    background-color: var(--bg-card);
    border: 1px solid var(--border);
    border-radius: 10px;
    padding: 14px 18px;
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 16px;
    transition: all 0.15s ease;
  }

  .service-card.disabled {
    opacity: 0.65;
  }

  .service-lead {
    display: flex;
    align-items: flex-start;
    gap: 14px;
    flex: 1;
  }

  /* Custom Checkbox */
  .custom-checkbox {
    position: relative;
    cursor: pointer;
    display: flex;
    align-items: center;
    margin-top: 2px;
    gap: 8px;
  }

  .custom-checkbox input {
    position: absolute;
    opacity: 0;
    cursor: pointer;
    height: 0;
    width: 0;
  }

  .checkmark {
    width: 18px;
    height: 18px;
    background-color: var(--bg-surface);
    border: 1px solid var(--border);
    border-radius: 4px;
    display: inline-block;
    transition: all 0.15s ease;
  }

  .custom-checkbox:hover input ~ .checkmark {
    border-color: var(--primary);
  }

  .custom-checkbox input:checked ~ .checkmark {
    background-color: var(--primary);
    border-color: var(--primary);
  }

  .checkmark:after {
    content: "";
    position: absolute;
    display: none;
  }

  .custom-checkbox input:checked ~ .checkmark:after {
    display: block;
    left: 6px;
    top: 2px;
    width: 4px;
    height: 9px;
    border: solid white;
    border-width: 0 2px 2px 0;
    transform: rotate(45deg);
  }

  .cb-label {
    font-size: 13px;
    color: var(--text-primary);
  }

  .service-info {
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .service-name-row {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .service-name {
    font-size: 14px;
    font-weight: 600;
    color: var(--text-primary);
  }

  .badge-active {
    font-size: 10px;
    font-weight: 600;
    background-color: var(--success-bg);
    color: var(--success);
    padding: 1px 6px;
    border-radius: 4px;
  }

  .service-domains {
    font-size: 12px;
    color: var(--text-muted);
  }

  .current-route-info {
    display: flex;
    align-items: center;
    gap: 6px;
    font-size: 12px;
    margin-top: 3px;
  }

  .route-ip {
    color: var(--primary-hover);
    font-family: monospace;
    font-weight: 500;
  }

  .route-prov {
    color: var(--text-secondary);
  }

  .route-lat {
    color: var(--success);
    font-weight: 600;
    background-color: var(--success-bg);
    padding: 1px 6px;
    border-radius: 4px;
  }

  .route-inactive {
    color: var(--text-muted);
    font-style: italic;
  }

  .sep {
    color: var(--border);
  }

  .service-selector {
    display: flex;
    align-items: center;
    gap: 8px;
    min-width: 270px;
  }

  .sel-label {
    font-size: 12px;
    color: var(--text-secondary);
  }

  .service-selector select {
    flex: 1;
  }

  /* Proxy Banner */
  .proxy-banner {
    background-color: var(--bg-surface);
    border: 1px solid var(--border);
    border-radius: 12px;
    padding: 16px 20px;
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 20px;
  }

  .proxy-header {
    display: flex;
    align-items: center;
    gap: 10px;
    margin-bottom: 4px;
  }

  .proxy-title {
    font-size: 14px;
    font-weight: 600;
    color: var(--text-primary);
  }

  .tag-noroot {
    font-size: 10px;
    font-weight: 700;
    background-color: rgba(245, 158, 11, 0.15);
    color: var(--warning);
    padding: 2px 7px;
    border-radius: 4px;
  }

  .proxy-desc {
    font-size: 12px;
    color: var(--text-secondary);
    line-height: 1.4;
  }

  .proxy-desc code {
    background-color: var(--bg-card);
    padding: 2px 5px;
    border-radius: 4px;
    color: var(--primary-hover);
    font-size: 11px;
  }

  .proxy-actions {
    display: flex;
    align-items: center;
    gap: 10px;
  }

  .btn-copy {
    background-color: var(--bg-card);
    border: 1px solid var(--border);
    color: var(--text-primary);
    padding: 8px 12px;
    font-size: 12px;
    font-family: monospace;
    border-radius: 6px;
  }
  .btn-copy:hover { border-color: var(--primary); }

  .btn-toggle-proxy {
    padding: 8px 16px;
    font-size: 13px;
    background-color: var(--bg-card);
    border: 1px solid var(--border);
    color: var(--text-primary);
    border-radius: 8px;
  }

  .btn-toggle-proxy.active {
    background-color: var(--success-bg);
    border-color: var(--success);
    color: var(--success);
  }

  /* Toolbar */
  .view-toolbar {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 16px;
    gap: 14px;
    flex-wrap: wrap;
  }

  .toolbar-controls {
    display: flex;
    align-items: center;
    gap: 12px;
  }

  .toggle-label {
    display: flex;
    align-items: center;
    gap: 6px;
    font-size: 12px;
    color: var(--text-secondary);
    cursor: pointer;
  }

  .toolbar-info h2 {
    font-size: 16px;
    font-weight: 600;
    color: var(--text-primary);
  }

  .toolbar-info p {
    font-size: 12px;
    color: var(--text-muted);
  }

  .btn-primary {
    background-color: var(--primary);
    color: #fff;
    padding: 8px 16px;
    font-size: 13px;
  }
  .btn-primary:hover { background-color: var(--primary-hover); }

  /* Providers List */
  .provider-list {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .provider-row {
    background-color: var(--bg-card);
    border: 1px solid var(--border);
    border-radius: 10px;
    padding: 12px 18px;
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 16px;
    transition: all 0.15s ease;
  }

  .provider-row.is-disabled {
    opacity: 0.55;
  }

  .provider-lead {
    display: flex;
    flex-direction: column;
    gap: 4px;
    flex: 1;
  }

  .provider-title {
    display: flex;
    align-items: center;
    gap: 8px;
    flex-wrap: wrap;
  }

  .provider-name {
    font-size: 14px;
    font-weight: 600;
    color: var(--text-primary);
  }

  .tag-kind {
    font-size: 10px;
    font-weight: 700;
    color: var(--text-muted);
    background-color: var(--bg-surface);
    padding: 2px 6px;
    border-radius: 4px;
  }

  .tag-custom {
    font-size: 10px;
    font-weight: 700;
    color: var(--warning);
    background-color: var(--warning-bg);
    padding: 2px 6px;
    border-radius: 4px;
  }

  .tag-disabled {
    font-size: 10px;
    font-weight: 700;
    color: var(--danger);
    background-color: var(--danger-bg);
    padding: 2px 6px;
    border-radius: 4px;
  }

  .coverage-badge {
    font-size: 11px;
    font-weight: 600;
    padding: 2px 7px;
    border-radius: 4px;
  }

  .coverage-badge.full {
    background-color: var(--success-bg);
    color: var(--success);
  }

  .coverage-badge.partial {
    background-color: var(--warning-bg);
    color: var(--warning);
  }

  .coverage-badge.none {
    background-color: var(--danger-bg);
    color: var(--danger);
  }

  .provider-endpoint {
    font-size: 12px;
    color: var(--text-muted);
    font-family: monospace;
  }

  .service-badges {
    display: flex;
    align-items: center;
    gap: 6px;
    margin-top: 4px;
    flex-wrap: wrap;
  }

  .s-badge {
    font-size: 10px;
    font-weight: 600;
    padding: 2px 6px;
    border-radius: 4px;
    background-color: var(--bg-surface);
    color: var(--text-muted);
  }

  .s-badge.ok {
    background-color: var(--success-bg);
    color: var(--success);
  }

  .s-badge.fail {
    background-color: var(--danger-bg);
    color: var(--danger);
  }

  .s-badge.pending {
    background-color: var(--primary-glow);
    color: var(--primary);
  }

  .provider-meta {
    display: flex;
    align-items: center;
    gap: 10px;
  }

  .badge-probing {
    display: flex;
    align-items: center;
    gap: 6px;
    font-size: 12px;
    color: var(--primary);
    background-color: var(--primary-glow);
    padding: 4px 8px;
    border-radius: 6px;
  }

  .latency-badge {
    font-size: 12px;
    font-weight: 600;
    padding: 5px 9px;
    border-radius: 6px;
    background-color: var(--bg-surface);
    color: var(--text-secondary);
  }

  .latency-badge.fast {
    background-color: var(--success-bg);
    color: var(--success);
  }

  .latency-badge.med {
    background-color: var(--warning-bg);
    color: var(--warning);
  }

  .latency-badge.warn {
    background-color: rgba(245, 158, 11, 0.15);
    color: var(--warning);
    border: 1px solid rgba(245, 158, 11, 0.3);
  }

  .latency-badge.bad {
    background-color: var(--danger-bg);
    color: var(--danger);
  }

  .btn-apply-all {
    background-color: var(--bg-surface);
    border: 1px solid var(--border);
    color: var(--text-primary);
    padding: 6px 12px;
    font-size: 12px;
  }
  .btn-apply-all:hover {
    border-color: var(--primary);
    background-color: var(--primary-glow);
    color: var(--primary);
  }

  .btn-icon {
    color: var(--text-secondary);
    padding: 6px;
    border-radius: 6px;
    background-color: var(--bg-surface);
  }
  .btn-icon:hover { color: var(--primary); background-color: var(--bg-surface-hover); }

  .btn-del {
    color: var(--text-muted);
    padding: 6px;
    border-radius: 6px;
    background-color: var(--bg-surface);
  }
  .btn-del:hover { color: var(--danger); background-color: var(--bg-surface-hover); }

  /* Settings Form */
  .settings-form {
    display: flex;
    flex-direction: column;
    gap: 16px;
  }

  .setting-card {
    background-color: var(--bg-card);
    border: 1px solid var(--border);
    border-radius: 10px;
    padding: 16px 20px;
  }

  .setting-desc {
    display: flex;
    flex-direction: column;
    gap: 4px;
    margin-bottom: 12px;
  }

  .setting-title {
    font-size: 14px;
    font-weight: 600;
    color: var(--text-primary);
  }

  .setting-sub {
    font-size: 12px;
    color: var(--text-muted);
    line-height: 1.5;
  }

  .add-grid {
    display: grid;
    grid-template-columns: 1fr 1.5fr 1fr auto;
    gap: 10px;
  }

  /* Form inside modal */
  .form-group {
    display: flex;
    flex-direction: column;
    gap: 6px;
  }

  .form-label {
    font-size: 12px;
    color: var(--text-secondary);
  }

  .form-row {
    display: flex;
    gap: 12px;
    align-items: center;
  }

  .form-group.half {
    flex: 1;
  }

  .checkbox-group {
    display: flex;
    align-items: flex-start;
    padding-top: 18px;
  }

  @media (max-width: 680px) {
    .add-grid {
      grid-template-columns: 1fr;
    }
    .status-hero {
      flex-direction: column;
      align-items: flex-start;
    }
  }
</style>
