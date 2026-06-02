# FH6 Telemetry Mirror Hub (Tauri GUI)

Native desktop GUI untuk menduplikasi aliran telemetri UDP Forza Horizon 6 ke beberapa endpoint secara bersamaan.

## 🏗️ Arsitektur

```
┌─────────────────────────────────────────────────────────┐
│              FH6 Telemetry Mirror Hub                   │
│                                                         │
│  ┌──────────────┐       ┌────────────────────────────┐  │
│  │  Svelte UI   │◄─────►│   Rust Backend (Tauri)     │  │
│  │  (WebView2)  │ IPC   │   • UDP Listener (async)   │  │
│  │              │       │   • Packet Router / Dup     │  │
│  │  Dashboard   │◄──────│   • Stats Emitter (1s)     │  │
│  │  Config UI   │events │   • Config Persistence     │  │
│  └──────────────┘       └────────────────────────────┘  │
│                                  │                       │
│                     ┌────────────▼──────────────┐        │
│                     │   UDP 0.0.0.0:20440       │        │
│                     │   (Forza game input)       │        │
│                     └────────────┬──────────────┘        │
│                                  │ duplicate             │
│                    ┌─────────────▼─────────────┐         │
│                    │  Node A  │  Node B  │ ...  │         │
│                    │ SimHub   │ iPhone   │ ...  │         │
│                    └─────────────────────────────        │
└─────────────────────────────────────────────────────────┘
```

## 🚀 Build untuk Windows (via GitHub Actions — Direkomendasikan)

Karena Tauri memerlukan WebView2 SDK dan tools installer (NSIS/WiX) yang **hanya tersedia di Windows**, cara terbaik untuk menghasilkan installer Windows `.exe` adalah via **GitHub Actions**.

### Cara Trigger Build:

**Opsi 1: Tag Git (otomatis release)**
```bash
git tag mirror-v1.0.0
git push origin mirror-v1.0.0
```
→ GitHub Actions akan otomatis build untuk semua platform dan membuat **GitHub Release** dengan semua installer.

**Opsi 2: Manual via GitHub UI**
1. Buka repository di GitHub
2. Klik tab **Actions**
3. Pilih workflow `FH6 Telemetry Mirror Hub - Release`
4. Klik **Run workflow** dan isi versi

### Hasil Build (GitHub Release):

| Platform | File | Ukuran |
|----------|------|--------|
| **Windows x64** | `FH6.Telemetry.Mirror.Hub_1.0.0_x64-setup.exe` | ~5–8 MB |
| **Linux x64** | `fh6-telemetry-mirror-hub_1.0.0_amd64.deb` | ~4–6 MB |
| **macOS ARM** | `FH6.Telemetry.Mirror.Hub_1.0.0_aarch64.dmg` | ~5–7 MB |
| **macOS Intel** | `FH6.Telemetry.Mirror.Hub_1.0.0_x64.dmg` | ~5–7 MB |

---

## 💻 Build Lokal (Native)

### Prerequisites

**Linux (Ubuntu/Debian):**
```bash
sudo apt-get install -y libwebkit2gtk-4.0-dev libssl-dev libgtk-3-dev \
  libayatana-appindicator3-dev librsvg2-dev
```

**macOS:**
```bash
# Xcode Command Line Tools diperlukan
xcode-select --install
```

**Windows:**
- Install [Visual Studio Build Tools 2022](https://visualstudio.microsoft.com/downloads/#build-tools-for-visual-studio-2022)
- Install [WebView2](https://developer.microsoft.com/en-us/microsoft-edge/webview2/) (biasanya sudah ada di Windows 10/11)

### Install Dependencies & Build

```bash
cd fh6-tel-mirror-tauri

# Install Node.js dependencies
npm install

# Dev mode (dengan hot reload)
npm run tauri:dev

# Production build (native platform)
npm run tauri:build
```

Output binary ada di:
- **Linux**: `src-tauri/target/release/bundle/deb/`
- **Windows**: `src-tauri/target/release/bundle/nsis/` atau `msi/`
- **macOS**: `src-tauri/target/release/bundle/dmg/`

---

## ⚙️ Konfigurasi

Konfigurasi disimpan otomatis di `mirror_config.json` di direktori kerja aplikasi:

```json
{
  "bindPort": 20440,
  "destinations": [
    {
      "id": "simhub",
      "name": "SimHub/Vibration Rig",
      "host": "127.0.0.1",
      "port": 20500,
      "rateLimit": "60Hz",
      "enabled": false
    }
  ]
}
```

### Rate Limiting:
| Setting | Paket dikirim | Kasus Penggunaan |
|---------|--------------|-----------------|
| `60Hz` | Semua paket | SimHub, dashboard real-time |
| `30Hz` | 1 dari 2 | Smartphone dashboard |
| `20Hz` | 1 dari 3 | Perangkat IoT, logging |
| `10Hz` | 1 dari 6 | Logging bandwidth rendah |

---

## 🔧 Pengaturan Game (Forza Horizon 6)

Di **Pengaturan** → **HUD dan Gameplay** → **Telemetri Data Out**:
- **Data Out**: `ON`  
- **Data Out IP Address**: `127.0.0.1`  
- **Data Out IP Port**: `20440`  
- **Data Out Packet Format**: `Car Dash`  

---

## 📁 Struktur Proyek

```
fh6-tel-mirror-tauri/
├── src/
│   ├── App.svelte          # UI Dashboard (Svelte 5)
│   └── main.ts             # Entry point
├── src-tauri/
│   ├── src/
│   │   └── main.rs         # Rust backend (UDP engine + Tauri commands)
│   ├── Cargo.toml          # Rust dependencies
│   ├── tauri.conf.json     # App config & bundle settings
│   └── icons/              # App icons (semua platform)
├── index.html
├── vite.config.ts
└── package.json
```
