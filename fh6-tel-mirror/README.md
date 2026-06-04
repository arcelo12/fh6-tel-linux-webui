# 🏁 FH6 Telemetry Mirror CLI/TUI

**Native CLI middleware** untuk menduplikasi aliran telemetri UDP Forza Horizon 6 ke beberapa endpoint secara bersamaan.

Ditulis dalam **Go murni** — menghasilkan binary statik tunggal tanpa dependensi runtime, berjalan di semua platform hanya dengan perintah satu baris.

---

## ✨ Fitur

- **Real-time TUI Dashboard** — statistik RX/TX paket, PPS, dan rata-rata latensi duplikasi
- **Multi-node routing** — salurkan ke SimHub, iPhone dashboard, perangkat IoT, dll secara bersamaan
- **Rate throttling** — pilih 60Hz / 30Hz / 20Hz / 10Hz per node tujuan
- **Config persisten** — pengaturan disimpan otomatis ke `mirror_config.json`
- **Hot-reload port** — ganti port inbound UDP tanpa restart aplikasi
- **Static binary** — tidak perlu runtime Go, .NET, atau library eksternal

---

## 📦 Download Binary (via GitHub Release)

> **Cara termudah**: Unduh binary langsung dari GitHub Releases — tidak perlu build sendiri.

### Cara Trigger Release via GitHub Actions

**Opsi 1: Git Tag (otomatis)**
```bash
git tag v1.0.0
git push origin v1.0.0
```

**Opsi 2: Manual di GitHub UI**
1. Buka repository di GitHub
2. Klik tab **Actions**
3. Pilih **`Release Suite (WebUI, CLI, GUI)`**
4. Klik **Run workflow** → isi nomor versi → **Run workflow**

### Hasil di GitHub Release

| Platform | File | Keterangan |
|----------|------|-----------|
| 🪟 Windows x64 | `fh6-tel-mirror-windows-amd64.exe` | Command Prompt / PowerShell |
| 🪟 Windows ARM | `fh6-tel-mirror-windows-arm64.exe` | Surface Pro X, dll |
| 🐧 Linux x64 | `fh6-tel-mirror-linux-amd64` | Ubuntu, Debian, Arch, dll |
| 🐧 Linux ARM | `fh6-tel-mirror-linux-arm64` | Raspberry Pi 4/5 |
| 🍎 macOS Apple Silicon | `fh6-tel-mirror-darwin-arm64` | M1, M2, M3, M4 |
| 🍎 macOS Intel | `fh6-tel-mirror-darwin-amd64` | Mac Intel |
| 📄 Config default | `mirror_config.json` | Template konfigurasi awal |

> **Catatan**: Semua binary dibangun dari **satu runner Linux** menggunakan cross-compilation Go — tidak butuh runner Windows/macOS.

---

## 🚀 Cara Penggunaan

### Windows

```cmd
:: Download fh6-tel-mirror-windows-amd64.exe dan mirror_config.json
:: Taruh keduanya di folder yang sama, lalu:
fh6-tel-mirror-windows-amd64.exe
```

### Linux / macOS

```bash
# Beri izin eksekusi
chmod +x fh6-tel-mirror-linux-amd64

# Jalankan (pastikan mirror_config.json ada di folder yang sama)
./fh6-tel-mirror-linux-amd64
```

---

## ⌨️ Kontrol TUI

Setelah dijalankan, layar terminal akan menampilkan dashboard real-time:

```
==================================================================
       🏁 FH6 TELEMETRY MIRROR MIDDLEWARE (NATIVE TUI) 🏁
==================================================================
  STATUS: ACTIVE  |  INBOUND UDP PORT: 20440
  TELEMETRY STREAM: OFFLINE (Waiting for Forza UDP stream...)
  INBOUND PAYLOAD:  0   pps  |  AVG DUP LATENCY: 0 µs
  PACKETS - RX: 0       |  TX (MIRRORED): 0
==================================================================
 ACTIVE MIRROR DESTINATIONS:
------------------------------------------------------------------
 #   NAME                   ADDRESS              LIMIT    STATUS
------------------------------------------------------------------
 [1] SimHub/Vibration Rig   127.0.0.1:20500      60Hz     DISABLED
 [2] Local FH6 Tel Server   127.0.0.1:20450      60Hz     ENABLED
==================================================================
 MENU COMMANDS:
 [A] Add Node  |  [T] Toggle Node  |  [D] Delete Node
 [P] Set Inbound UDP Port    |  [Q] Exit App
==================================================================
Enter command:
```

| Perintah | Fungsi |
|---------|--------|
| `A` | Tambah node tujuan baru (nama, IP, port, rate limit) |
| `T` | Toggle node aktif ↔ nonaktif |
| `D` | Hapus node dari routing table |
| `P` | Ubah port UDP inbound (restart listener otomatis) |
| `Q` | Keluar dari aplikasi |

---

## ⚙️ Konfigurasi (`mirror_config.json`)

File ini dibuat otomatis saat pertama kali dijalankan. Taruh di folder yang sama dengan binary.

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
    },
    {
      "id": "local-hub",
      "name": "Local FH6 Telemetry Server",
      "host": "127.0.0.1",
      "port": 20450,
      "rateLimit": "60Hz",
      "enabled": true
    }
  ]
}
```

### Rate Limiting

| Nilai | Paket dikirim | Bandwidth relatif | Kasus Penggunaan |
|-------|--------------|------------------|-----------------|
| `60Hz` | Semua (1/1) | 100% | SimHub, dashboard real-time |
| `30Hz` | 1 dari 2 | 50% | Smartphone dashboard |
| `20Hz` | 1 dari 3 | 33% | Perangkat IoT |
| `10Hz` | 1 dari 6 | 17% | Logger bandwidth rendah |

---

## 🎮 Pengaturan Game Forza Horizon 6

Buka **Settings → HUD and Gameplay → Data Out**:

| Pengaturan | Nilai |
|-----------|-------|
| **Data Out** | `ON` |
| **Data Out IP Address** | `127.0.0.1` (atau IP PC jika dari konsol) |
| **Data Out IP Port** | `20440` |
| **Data Out Packet Format** | `Car Dash` |

---

## 🔨 Build Lokal (Opsional)

Jika ingin build sendiri tanpa menggunakan GitHub Actions:

```bash
cd fh6-tel-mirror

# Build untuk platform lokal saja
make build

# Build semua platform sekaligus
make release

# Hasil ada di dist/
ls -lh dist/
```

### Prasyarat

- [Go 1.21+](https://go.dev/dl/)
- `make` (Linux/macOS) atau gunakan perintah `go build` langsung di Windows

### Build Manual Tanpa Makefile

```bash
# Windows dari Linux/macOS (cross-compile)
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o dist/fh6-tel-mirror-windows-amd64.exe .

# Linux dari mana saja
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o dist/fh6-tel-mirror-linux-amd64 .
```

---

## 📁 Struktur Proyek

```
fh6-tel-mirror/
├── main.go              # TUI dashboard, input loop, UDP router
├── config.go            # Config struct, load/save JSON
├── mirror_config.json   # Konfigurasi default
├── Makefile             # Build scripts
└── go.mod               # Go module definition
```

---

## 🆚 Perbandingan dengan Versi Tauri

| Fitur | CLI/TUI (`fh6-tel-mirror`) | GUI Tauri (`fh6-tel-mirror-tauri`) |
|-------|--------------------------|-----------------------------------|
| **Interface** | Terminal (ANSI) | Native GUI (WebView2) |
| **Ukuran binary** | ~5 MB | ~15–30 MB |
| **Instalasi** | Tidak perlu | Tidak perlu (portable) |
| **Cross-compile** | ✅ Ya (dari Linux) | ❌ Perlu CI per platform |
| **Windows** | ✅ `.exe` langsung | ✅ NSIS installer `.exe` |
| **Server/headless** | ✅ Ya | ❌ Butuh display |
| **Raspberry Pi** | ✅ ARM64 | ❌ Tidak didukung |
