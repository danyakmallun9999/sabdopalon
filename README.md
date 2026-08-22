<div align="center">
  <img src="images/logo-text.png" alt="Sabdopalon" width="340">
  <p><strong>Lingkungan development lokal yang siap pakai — PHP, database, dan tools dalam satu aplikasi.</strong></p>
  <p>Gratis selamanya (MIT) · Windows, macOS, Linux</p>
</div>

---

Sabdopalon adalah aplikasi untuk membuat website dan aplikasi PHP di komputer
kamu sendiri (local development). Semua yang kamu butuhkan — PHP, database,
mail catcher, dan lainnya — dipasang dan dikelola otomatis di dalam satu
folder, tanpa perlu menginstal atau mengonfigurasi apa pun secara manual.

Cukup jalankan `sabdopalon`, lalu:

- **Dashboard di browser** — buat situs baru, mulai/hentikan situs, install
  PHP atau MariaDB, aktifkan HTTPS, backup database, semua lewat klik.
- **Situs langsung jalan** — taruh folder proyek di `sites/`, langsung bisa
  diakses di `http://namasitus.localhost` tanpa perlu Apache/Nginx.
- **Aman & bersih** — semua tersimpan di dalam folder Sabdopalon, tidak
  mengubah sistem operasi kamu.

## Fitur utama

| Fitur | Keterangan |
|---|---|
| 🖥️ Dashboard web | Semua pengaturan dari browser — tanpa perlu edit file config |
| 🌐 Banyak situs | Setiap folder di `sites/` otomatis jadi situs |
| 🐘 Multi-PHP | PHP 8.1 – 8.5, bisa pakai versi berbeda per situs |
| 🗄️ Database | SQLite (tanpa setup) atau MariaDB (otomatis dikelola) |
| 🔒 HTTPS lokal | Sertifikat lokal untuk situs kamu dalam beberapa klik |
| 📧 Mailpit | Tangkap email lokal supaya tidak terkirim ke luar |
| 📦 Tools tambahan | PostgreSQL, Redis, MinIO, Meilisearch — opsional, satu klik |
| 💾 Backup database | Sekali klik, otomatis menyimpan riwayat |
| 🪟 Aplikasi desktop | Versi native dengan tray icon, autostart, dan wizard instalasi |
| ⌨️ Terminal bawaan | Jalankan composer/artisan langsung dari dashboard |

## Status

`v0.7.2` — terminal utuh + revamp halaman situs. Saat pertama
dijalankan, muncul wizard
instalasi interaktif (di terminal maupun di aplikasi desktop) yang menyiapkan
semua yang dibutuhkan. Ada juga installer satu perintah
(`install.sh` / `install.ps1`), aplikasi desktop native (tray icon +
autostart + wizard GUI), dan terminal bawaan di dashboard.

## Prasyarat

**Tidak ada.** Sabdopalon sudah lengkap sendiri — tidak perlu menginstal Go,
PHP, atau MariaDB secara manual. Cukup unduh dan jalankan:

- **PHP** — otomatis terpasang lewat wizard instalasi (8.4, ~8MB, 30+ ekstensi)
- **MariaDB** — otomatis terpasang lewat wizard (pilihan bawaan)
- **PostgreSQL** — opsional, satu klik di wizard
- **Go** — hanya diperlukan kalau mau membangun dari kode sumber sendiri

> Kalau komputer kamu sudah punya PHP, Sabdopalon otomatis memakainya.
> Unduhan otomatis hanya terjadi kalau PHP tidak ditemukan.

## Instalasi

### Cara A: Aplikasi desktop (tanpa terminal — disarankan untuk Windows)

Unduh installer desktop dari halaman
[Releases](https://github.com/danyakmallun9999/sabdopalon/releases):

| Platform | File |
|---|---|
| Windows x86_64 | `Sabdopalon_0.7.2_x64-setup.exe` (NSIS) |
| macOS (Apple Silicon) | `Sabdopalon.app.tar.gz` / `.dmg` |
| Linux | `.deb` / `.AppImage` |

Klik dua kali → wizard instalasi GUI standar → selesai. **Sepanjang instalasi
dan pemakaian kamu tidak perlu menyentuh terminal sama sekali** — setup
pertama berjalan sebagai wizard di dalam jendela aplikasi, PHP/MariaDB/
phpMyAdmin sudah ikut ter-bundle di dalam installer, dan tidak ada jendela
konsol yang muncul saat proses berjalan.

### Cara B: Installer satu perintah (via terminal)

```bash
# Linux / macOS:
curl -sSL https://github.com/danyakmallun9999/sabdopalon/releases/latest/download/install.sh | bash

# Windows (PowerShell) — opsional, untuk yang terbiasa dengan terminal:
irm https://github.com/danyakmallun9999/sabdopalon/releases/latest/download/install.ps1 | iex
```

Installer akan mengunduh paket untuk sistem kamu, mengekstraknya ke
`~/sabdopalon` (`%USERPROFILE%\sabdopalon` di Windows), menambahkan ke PATH,
lalu menjalankan **wizard instalasi** — kamu tinggal jawab beberapa
pertanyaan (PHP + MariaDB sebagai bawaan, PostgreSQL opsional, port, situs
contoh).

### Cara C: Unduh binary langsung (via terminal)

Unduh rilis terbaru untuk sistem operasi kamu dari halaman
[Releases](https://github.com/danyakmallun9999/sabdopalon/releases).

| Platform | File |
|---|---|
| Linux x86_64 (Intel/AMD) | `sabdopalon-linux-x86_64.tar.gz` |
| Linux aarch64 (ARM64) | `sabdopalon-linux-aarch64.tar.gz` |
| macOS x86_64 (Intel) | `sabdopalon-macos-x86_64.tar.gz` |
| macOS aarch64 (Apple Silicon) | `sabdopalon-macos-aarch64.tar.gz` |
| Windows x86_64 | `sabdopalon-windows-x86_64.zip` |

Setiap arsip berisi **paket lengkap**: binary + config bawaan + daftar paket +
folder data + installer. Ekstrak di mana saja, lalu jalankan `./sabdopalon` —
wizard instalasi muncul otomatis saat pertama kali dijalankan.

```bash
# Contoh (Linux x86_64):
curl -L https://github.com/danyakmallun9999/sabdopalon/releases/latest/download/sabdopalon-linux-x86_64.tar.gz | tar xz
chmod +x sabdopalon
./sabdopalon version

# macOS (Apple Silicon):
curl -L https://github.com/danyakmallun9999/sabdopalon/releases/latest/download/sabdopalon-macos-aarch64.tar.gz | tar xz
chmod +x sabdopalon
./sabdopalon version

# Windows (PowerShell):
# Unduh sabdopalon-windows-x86_64.zip, ekstrak, lalu:
.\sabdopalon.exe version
```

### Cara D: Membangun dari kode sumber (butuh Go 1.22+)

```bash
# 1. Clone repository
git clone https://github.com/danyakmallun9999/sabdopalon.git
cd sabdopalon

# 2. Build binary
#    Linux / macOS:
go build -o sabdopalon ./cmd/sabdopalon

#    Windows (PowerShell / CMD):
go build -o sabdopalon.exe .\cmd\sabdopalon

# 3. Verifikasi
./sabdopalon version    # Linux/macOS
.\sabdopalon.exe version # Windows
```

### Supaya bisa dijalankan dari mana saja (opsional)

```bash
# Linux / macOS: symlink ke /usr/local/bin
sudo ln -s "$(pwd)/sabdopalon" /usr/local/bin/sabdopalon
# Sekarang 'sabdopalon' bisa dijalankan dari folder mana pun

# Windows: tambahkan folder ke PATH, atau salin sabdopalon.exe ke folder PATH
```

## Mulai cepat

Setelah terpasang, **wizard instalasi muncul otomatis** saat pertama kali
dijalankan. Kamu juga bisa menjalankannya kapan saja:

```bash
./sabdopalon setup        # wizard interaktif (PHP + MariaDB sebagai bawaan)
./sabdopalon              # mulai normal — dashboard + database + situs
./sabdopalon doctor       # cek kesehatan: PHP, port, database, SSL
```

## Menjalankan situs

```bash
# Mulai Sabdopalon (tekan Ctrl+C untuk berhenti)
./sabdopalon

# Lalu buka di browser:
#   http://localhost:9900/              ← dashboard interaktif
#   http://example-app.localhost:8080/  ← situs kamu (HTTP)
#   https://example-app.localhost:8443/ ← situs kamu (HTTPS, kalau sertifikat sudah dibuat)
```

### Menambah situs baru

```bash
# Cara 1: dari template
./sabdopalon new blank myblog
# → membuat sites/myblog/public/index.php
# → buka http://myblog.localhost:8080/

# Cara 2: manual — cukup buat folder
mkdir -p sites/myapp/public
echo '<?php echo "Hello!";' > sites/myapp/public/index.php
# → buka http://myapp.localhost:8080/ (tanpa perlu restart)
```

Bisa juga lewat dashboard: tombol **New Site** di halaman Sites.

### Yang diunduh otomatis

| Komponen | Kapan | Ukuran |
|---|---|---|
| **PHP 8.4** | Saat pertama kali dijalankan jika PHP tidak ditemukan | ~8 MB |
| **MariaDB 11.4** | Lewat `sabdopalon add mariadb` atau wizard | ~250 MB |

Semua file diverifikasi (SHA-256) dan disimpan di dalam folder `bin/` — tidak
ada instalasi ke sistem, tidak mengotori OS, semuanya portabel.

## Layanan tambahan (opsional)

Layanan lokal bisa diaktifkan lewat halaman **Services** di dashboard
(berlaku langsung dan tersimpan otomatis), atau lewat file config:

```toml
[services]
mailpit = false      # penangkap email lokal — SMTP :1025, antarmuka web :8025
redis = false        # cache & antrian — :6379
minio = false        # penyimpanan S3-compatible — API :9000, konsol :9001
meilisearch = false  # mesin pencari instan — :7700
```

| Layanan | Cara pasang | Port | Yang didapat PHP |
|---|---|---|---|
| **Mailpit** | `add mailpit` | SMTP 1025 · UI 8025 | `SABDOPALON_MAIL_SMTP`, `SABDOPALON_MAIL_UI` |
| **Redis** | `add redis` (Windows) atau `redis-server` sistem (Linux/macOS) | 6379 | `SABDOPALON_REDIS_HOST`, `SABDOPALON_REDIS_PORT` |
| **MinIO** | `add minio` | API 9000 · Konsol 9001 | `SABDOPALON_S3_ENDPOINT/KEY/SECRET/BUCKET` |
| **Meilisearch** | `add meilisearch` | 7700 | `SABDOPALON_MEILI_HOST` |

Cuplikan `.env` Laravel untuk tiap layanan yang berjalan otomatis muncul di
halaman Services (Redis cache/queue, MinIO S3 filesystem, Meilisearch Scout)
lengkap dengan tombol salin.

### Mesin database

- **SQLite** (bawaan) — tanpa setup, tersimpan di `data/sabdopalon.db`.
- **MariaDB** — `sabdopalon add mariadb`, lalu set `engine = "mariadb"` di
  `config/engine.toml`. Dijalankan otomatis di `:3306`.
- **PostgreSQL** — `sabdopalon add postgresql` (Linux/macOS; Windows perlu
  instalasi sistem), lalu set `engine = "postgresql"`. Dijalankan otomatis di
  `:5432`, user `sabdopalon` dengan akses lokal.

Kredensial root mengikuti kebiasaan umum di lokal (XAMPP): **user root tanpa
password**, hanya bisa diakses dari komputer sendiri (`127.0.0.1`) — cocok
untuk dipakai phpMyAdmin atau halaman Adminer bawaan (`add adminer` →
`http://adminer.localhost`).

### Catatan platform

| | Linux | macOS | Windows |
|---|---|---|---|
| `*.localhost` langsung jalan | ✅ otomatis | ✅ otomatis | ✅ otomatis (Win 10+) |
| Perlu edit `/etc/hosts` | Tidak | Tidak | Tidak |
| Nama binary | `sabdopalon` | `sabdopalon` | `sabdopalon.exe` |

> **`.localhost` berlaku di semua platform**: Linux, macOS, dan Windows 10+
> modern otomatis mengarahkan `*.localhost` ke `127.0.0.1` — tanpa edit
> `/etc/hosts`, tanpa perlu akses admin.

## Cara kerja

```
                 ┌──────────────────────────────────────────┐
                 │   sabdopalon (program Go)                │
                 │                                          │
   browser ─────▶│   HTTP proxy :8080  +  HTTPS :8443       │
                 │     diarahkan dari nama host             │
                 │                                          │
                 │   Dashboard :9900 (antarmuka + API)      │
                 │                                          │
                 │  Host: example-app.localhost             │
                 │     ▼  (dinyalakan saat diakses)         │
                 │   ReverseProxy ───────▶   php -S :9001 -t sites/example-app/public
                 └──────────────┬───────────────────────────┘
                                 │
                 ┌──────────────▼───────────────────────────┐
                 │  MariaDB 11.4.12 (dikelola otomatis)     │
                 │  port 3306, auto-backup, socket in data/ │
                 └──────────────────────────────────────────┘
```

## Perintah

Semua perintah di bawah juga bisa dilakukan lewat dashboard — CLI sifatnya
opsional.

| Perintah | Kegunaan |
|---|---|
| `sabdopalon` (atau `serve`) | Menjalankan semuanya dan membuka dashboard |
| `sabdopalon doctor` | Cek kesehatan: PHP, port, database, SSL |
| `sabdopalon sites` | Daftar situs + URL-nya |
| `sabdopalon new <template> <nama>` | Membuat proyek (blank, laravel, wordpress, codeigniter) |
| `sabdopalon add <paket>` | Pasang paket: `mariadb`, `mailpit`, `php@8.2` … |
| `sabdopalon pkg:list` | Daftar paket yang tersedia + status |
| `sabdopalon php:list` | Versi PHP yang terpasang (8.1–8.5) |
| `sabdopalon ssl:ca` / `ssl:wildcard` / `ssl:issue <host>` / `ssl:trust` | HTTPS lokal dalam empat langkah |
| `sabdopalon enable-ports` | Izinkan URL bersih di :80/:443 |
| `sabdopalon backup` / `backup:list` | Backup database |
| `sabdopalon profile:create/list/delete` | Profil lingkungan |
| `sabdopalon setup` | Menjalankan ulang wizard instalasi |
| `sabdopalon version` / `help` | Info versi / bantuan |

### Pengaturan per situs (`.sabdopalon.yml`)

Taruh file ini di folder situs untuk pengaturan khusus proyek itu:

```yaml
php: "8.3"          # versi PHP dari `add php@8.3`
docroot: public     # folder utama situs (relatif ke folder situs)
aliases:            # domain tambahan yang diarahkan ke situs ini
  - www.myapp.test
env:
  APP_ENV: local
```

## Dashboard

Dashboard dibangun dengan React (tampilan modern, tema gelap) dan tersimpan
di dalam binary — tidak perlu Node.js saat dipakai. Dashboard hanya bisa
diakses dari komputer sendiri (`127.0.0.1`).

| Halaman | Fungsinya |
|---|---|
| 🌐 Sites | Buat situs dari template, buka, mulai/hentikan/restart, hapus |
| 🗄️ Database | Status engine, backup sekali klik + riwayat otomatis |
| 🧩 Services | Aktifkan Mailpit/Redis/MinIO/Meilisearch, cuplikan .env |
| 📦 Packages | Pasang MariaDB, PostgreSQL, Mailpit, PHP 8.1–8.5 dengan progres |
| 🔒 SSL | Wizard CA → wildcard → trust; deteksi sertifikat lama |
| ⚙️ Settings | TLD, port, engine database, buka otomatis; terapkan profil |
| 🖥️ Terminal | Terminal bawaan (xterm) — jalankan php, mysql, composer |
| 📜 Logs | Log PHP per situs, database, dan Mailpit |

> **Pertama kali dijalankan / aplikasi desktop:** selama belum ada pengaturan,
> dashboard masuk **mode setup** dan menampilkan wizard di `/setup` (juga
> halaman awal aplikasi desktop).

## Aplikasi desktop

Sabdopalon punya aplikasi desktop native — jendela OS sungguhan (tanpa bilah
URL), tray icon, autostart saat login, dan **wizard instalasi bergaya GUI**
pada saat pertama dijalankan (tanpa perlu menyentuh terminal).

**Jaminan bebas-konsol di Windows:** sidecar Go dibangun sebagai aplikasi
GUI (`-H windowsgui`) dan seluruh proses anak (PHP, MariaDB, certutil, dll.)
dijalankan dengan flag `CREATE_NO_WINDOW`, jadi tidak ada jendela konsol
hitam yang muncul atau berkedip — pengguna Windows sepenuhnya berinteraksi
lewat GUI saja.

- **Windows** — installer NSIS (per-user, tanpa admin), shortcut Start Menu.
- **macOS** — `.dmg`, tinggal seret ke Applications.
- **Linux** — `.deb` + `.AppImage`.

Data aplikasi disimpan di folder data milik OS (mis. `%LOCALAPPDATA%\Sabdopalon`
di Windows, `~/Library/Application Support/Sabdopalon` di macOS,
`~/.local/share/sabdopalon` di Linux) — aplikasinya sendiri terpasang read-only.

Membangun dari kode sumber:

```bash
cd desktop
npm install
bash scripts/build-sidecar.sh   # membangun sidecar Go untuk platform kamu
npm run dev                     # tauri dev (butuh toolchain Rust)
```

> **Catatan tanda tangan:** build macOS/Windows yang tidak ditandatangani
> menampilkan peringatan saat pertama dibuka (klik kanan → buka / "More
> info"). Penandatanganan butuh akun developer berbayar — di luar cakupan
> untuk sekarang.

### URL bersih (`https://myapp.localhost` — tanpa port)

Sabdopalon otomatis mencoba port 80/443 dulu. Tanpa hak akses, otomatis
mundur ke 8080/8443. Untuk mengizinkan port rendah secara permanen:

```bash
./sabdopalon enable-ports   # Linux: sudo setcap cap_net_bind_service=+ep <binary>
```

## Pengaturan PHP

Semua proses PHP (untuk semua situs) memakai `config/php.ini`, yang dibuat
otomatis saat pertama kali dijalankan:

```ini
memory_limit = 256M
upload_max_filesize = 64M
post_max_size = 64M
max_execution_time = 120
date.timezone = UTC
```

Edit file itu lalu mulai ulang Sabdopalon (atau situsnya saja) untuk
menerapkan. Pengaturan per situs ada di `.sabdopalon.yml` (versi PHP,
docroot, variabel lingkungan).

## Variabel lingkungan (tersedia di PHP)

| Variabel | Contoh |
|---|---|
| `SABDOPALON` | `1` |
| `SABDOPALON_DB_ENGINE` | `sqlite` / `mariadb` / `mysql` / `postgresql` |
| `SABDOPALON_DB_PATH` | `/path/to/sabdopalon.db` (khusus sqlite) |
| `SABDOPALON_PG_HOST` / `PORT` / `USER` / `DB` | `127.0.0.1` / `5432` / `sabdopalon` / `postgres` (engine = postgresql) |
| `SABDOPALON_MAIL_SMTP` / `SABDOPALON_MAIL_UI` | Mailpit SMTP + UI (saat berjalan) |
| `SABDOPALON_REDIS_HOST` / `SABDOPALON_REDIS_PORT` | Redis (saat berjalan) |
| `SABDOPALON_S3_ENDPOINT` / `KEY` / `SECRET` / `BUCKET` | MinIO S3 (saat berjalan) |
| `SABDOPALON_MEILI_HOST` | Meilisearch (saat berjalan) |

## Struktur folder

```
sabdopalon/
├── sabdopalon                 # program utamanya
├── config/engine.toml         # pengaturan global
├── config/profiles/           # profil pengaturan
├── sites/                     # web root: 1 folder = 1 situs
├── bin/mariadb/               # MariaDB yang diunduh (tidak ikut git)
├── data/                      # folder data database (tidak ikut git)
├── logs/                      # log PHP per situs + database
├── backups/                   # backup database
└── certs/                     # sertifikat SSL
```

## Lisensi

MIT — lihat [LICENSE](LICENSE). Sabdopalon mengelola komponen open-source
(PHP, MariaDB) yang diunduh lewat sistem paket.
