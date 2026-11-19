# AIRoute

Universal Smart DNS and hosts router designed for seamless, zero-overhead connectivity to AI models (Google Gemini, Claude AI, ChatGPT, Antigravity IDE, Cursor) without persistent proxy daemons, VPN tunnels, or RAM consumption.

<details open>
<summary><b>English</b></summary>

## Overview

**AIRoute** (binary `air`) decouples network routing from heavy background proxy daemons. It operates at the operating system hosts level with atomic updates and automated DNS cache flushing, providing instant direct connectivity to blocked or rate-limited AI endpoints.

## Features

- **Zero-RAM Hosts Routing**: Manipulates system hosts entries directly within marked comment boundaries (`# === AIROUTE START ===`). When idle, consumes 0 MB RAM and 0% CPU.
- **AI Ecosystem Registries**: Pre-configured target profiles for:
  - **Google Gemini & Google AI Studio** (`gemini.google.com`, `aistudio.google.com`, `generativelanguage.googleapis.com`)
  - **Claude AI & Anthropic** (`claude.ai`, `api.anthropic.com`, `cdn.claude.ai`)
  - **ChatGPT & OpenAI** (`chatgpt.com`, `api.openai.com`, `oaistatic.com`)
  - **Antigravity IDE** (`cloudcode-pa.googleapis.com`, `aicode.googleapis.com`)
  - **Cursor IDE** (`cursor.com`, `api2.cursor.sh`)
- **Built-in Gateway Pool**: Includes verified, low-latency Smart DNS providers:
  - `DNS-AI.RU` (Encrypted DoH/DoT/DoQ, GeoIP bypass)
  - `GeoHide`
  - `MagicDNS`
  - `Bezmezhau`
  - `Zapret`
- **Real-Time Latency Probing**: TLS handshake verification and RTT latency measuring against Google, Cloudflare, and OpenAI to filter captchas and stale proxies.
- **Custom Resolvers**: Add and persist custom DoH (DNS-over-HTTPS) or standard UDP DNS endpoints.
- **Modern Minimalist Desktop UI**: Built with Wails v2, Go, Svelte, and TypeScript. Dark theme, zero horizontal overflow, and vector SVG iconography.

## Installation

### Windows (10 / 11)
- **Installer**: Download `AIRoute_Windows_Installer.exe` from [Releases](https://github.com/CogniDevs/AIRoute/releases) and run setup.
- **Portable**: Download and run `AIRoute_Windows_Portable.exe` directly.

### Linux (Ubuntu 22.04+ / Ubuntu 24.04+ / Debian 12+)
- **Ubuntu / Debian (.deb)**:
  ```bash
  sudo apt update
  sudo apt install ./AIRoute_Linux_amd64.deb
  ```
- **Portable Archive (.tar.gz)**:
  ```bash
  # Ensure runtime dependencies are installed:
  sudo apt install -y libgtk-3-0 libwebkit2gtk-4.1-0
  mkdir -p airoute && tar -xzf AIRoute_Linux_amd64.tar.gz -C airoute
  ./airoute/bin/airoute
  ```

## Development & Build

Requires Go 1.23+, Node.js 20+, and Wails CLI v2:

```bash
# Clone
git clone https://github.com/CogniDevs/AIRoute.git
cd AIRoute

# Install frontend dependencies
cd frontend && npm install && cd ..

# Live dev with hot-reload
wails dev

# Production build
wails build -platform windows/amd64
```

## License

MIT License (c) 2026 CogniDevs.
</details>

<details>
<summary><b>Русский</b></summary>

## Обзор

**AIRoute** — универсальный легковесный Smart DNS и hosts-маршрутизатор для прямого доступа к зарубежным нейросетям (Google Gemini, Claude AI, ChatGPT, Antigravity IDE, Cursor) без использования VPN, сторонних прокси-клиентов и фоновых служб.

## Возможности

- **Маршрутизация без фоновых служб (Zero-RAM)**: управление системным файлом hosts с атомарным обновлением внутри изолированных маркеров (`# === AIROUTE START ===`). В пассивном режиме утилита потребляет 0 МБ RAM.
- **Готовые профили экосистем**:
  - **Google Gemini и AI Studio** (`gemini.google.com`, `aistudio.google.com`)
  - **Claude AI и Anthropic API** (`claude.ai`, `api.anthropic.com`)
  - **ChatGPT и OpenAI API** (`chatgpt.com`, `api.openai.com`)
  - **Antigravity IDE** (`cloudcode-pa.googleapis.com`, `aicode.googleapis.com`)
  - **Cursor IDE** (`cursor.com`, `api2.cursor.sh`)
- **Проверенный пул шлюзов**: интеграция с DNS-AI.RU, GeoHide, MagicDNS, Bezmezhau и Zapret с автоматическим замером задержки (RTT / пинг).
- **Валидация TLS**: проверка подлинности сертификатов Google, Cloudflare и OpenAI с фильтрацией капчи и заглушек.
- **Пользовательские DNS**: поддержка добавления и сохранения собственных DoH (DNS-over-HTTPS) и стандартных DNS-серверов.
- **Минималистичный темный интерфейс**: быстрое включение в один клик, нативные векторные иконки и разделение параметров по вкладкам.

## Установка

### Windows (10 / 11)
- **Установщик**: скачайте `AIRoute_Windows_Installer.exe` из раздела [Releases](https://github.com/CogniDevs/AIRoute/releases) и следуйте указаниям мастера.
- **Портативная версия**: скачайте и запустите `AIRoute_Windows_Portable.exe` без установки.

### Linux (Ubuntu 22.04+ / Ubuntu 24.04+ / Debian 12+)
- **Ubuntu / Debian (.deb)**:
  ```bash
  sudo apt update
  sudo apt install ./AIRoute_Linux_amd64.deb
  ```
- **Портативный архив (.tar.gz)**:
  ```bash
  # Установка рантайм-зависимостей WebKit и GTK:
  sudo apt install -y libgtk-3-0 libwebkit2gtk-4.1-0
  mkdir -p airoute && tar -xzf AIRoute_Linux_amd64.tar.gz -C airoute
  ./airoute/bin/airoute
  ```

## Сборка из исходников

Требуются Go 1.23+, Node.js 20+ и Wails CLI v2:

```bash
# Клонирование
git clone https://github.com/CogniDevs/AIRoute.git
cd AIRoute

# Установка зависимостей интерфейса
cd frontend && npm install && cd ..

# Режим разработки
wails dev

# Релизная сборка
wails build -platform windows/amd64
```

## Лицензия

MIT License (c) 2026 CogniDevs.
</details>
