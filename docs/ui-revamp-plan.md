# Sabdopalon UI/UX Revamp Plan

**Status:** planning  
**Referensi gaya:** `untitled.pen` (12 frame, pen.dev)  
**Ruang lingkup:** revamp visual dan UX konsistensi. Bukan penambahan atau pengurangan fitur.  
**Target:** seluruh dashboard, setup wizard, shell desktop, dialog, state, dan responsive behavior.

---

## 1. Tujuan

1. Menjadikan desain di `untitled.pen` sebagai **preferensi gaya**, bukan spesifikasi fitur.
2. Menerapkan bahasa visual yang sama ke **seluruh sistem**, termasuk bagian yang belum didesain.
3. Mempertahankan informasi, aksi, alur, dan data yang sudah ada.
4. Menghasilkan UI yang lebih tenang, padat, konsisten, dan nyaman dipakai di window default
   Sabdopalon (1280 × 800) maupun di 1440 px.

---

## 2. Prinsip dan Guardrails

### 2.1 Yang termasuk revamp

- Warna, tipografi, spacing, radius, border, elevation, ikon, dan density.
- Shell: title bar, sidebar, top bar, page header, dan pola scroll.
- Semua halaman, tab, dialog, sheet, dropdown, form, tabel, chart, empty state, loading, dan error.
- Responsive behavior dan aksesibilitas.
- Pemindahan judul halaman dari top bar ke page header (struktur visual, bukan fitur).
- Penyesuaian copy hanya untuk label/header yang memang dibutuhkan oleh pola baru, tanpa
  mengubah makna atau mengklaim fitur yang tidak ada.

### 2.2 Yang tidak termasuk

- Route, endpoint API, aksi, field, tab, atau fitur baru.
- Global search, command palette, notifications, avatar, akun, atau settings baru.
- Quick-install service dari Dashboard (desain menampilkan chip `+`; tidak dipakai).
- Tab atau aksi yang tidak ada di aplikasi sekarang.
- Perubahan TLD, port, data mock, versi, atau perilaku backend.
- Light theme. Aplikasi saat ini dark-only (`<html class="dark">`), dan desain juga dark-only.
- Mass translasi copy. Bahasa existing per halaman dipertahankan.
- Perubahan pada `untitled.pen`.

### 2.3 Aturan ekstrapolasi

Jika suatu surface tidak ada di `.pen`:

1. Ambil pola terdekat dari desain.
2. Pertahankan struktur informasi dan aksi yang sudah ada.
3. Jangan menambah elemen dekoratif yang menyiratkan fitur baru.
4. Gunakan token dan komponen yang sama dengan halaman lain.
5. Verifikasi di 1280 × 800 dan 960 × 640, bukan hanya 1440 px.

### 2.4 Penyimpangan yang disengaja dari mockup

| Mockup | Keputusan | Alasan |
|---|---|---|
| Top bar berisi search, notifications, avatar | Tidak dipakai | Fitur tidak ada. Top bar hanya memuat status live yang sudah ada. |
| Additional services berupa chip `+` | Tidak dipakai | Aksi install dari Dashboard tidak ada. Pertahankan link ke Packages. |
| Service status berupa baris meter | Chart historis dipertahankan | Meter hanya menunjukkan state saat ini dan menghilangkan histori running vs enabled. |
| Traffic chart berupa bar | Boleh diubah ke bar | Data per menit sama; tooltip dan informasi tetap. |
| TLD `.wowo`, port 80/443 | Gunakan nilai live | Default aplikasi `.localhost`, port 8080/8443. |
| Tipografi body 10–12 px | Naikkan floor body/meta ke 12 px | Keterbacaan di window 1280 dan scaling Windows. |
| Tombol `Open` di site detail | Tidak ditambahkan | URL site sudah menjadi link. |
| `Open file`, `Check status`, `Firefox guide` | Tidak ditambahkan | Tidak ada aksi tersebut di aplikasi. |
| `Clear` di halaman Terminal | Tidak ditambahkan | Hanya ada di terminal dock Sites, bukan di halaman Terminal. |
| Ring ganda pada semua kontrol | Ring untuk Card dan CTA; flat untuk kontrol rapat | Menghindari tabrakan shadow di toolbar/tabel. |

---

## 3. Bahasa Visual yang Diadopsi

### 3.1 Warna

Token baru menjadi sumber kebenaran. Nilai lama dipertahankan sebagai alias agar komponen
yang belum disentuh tidak rusak.

| Token | Nilai | Pemakaian |
|---|---|---|
| `--bg-app` | `#0A0A0A` | Background aplikasi, sidebar, title bar |
| `--bg-surface` | `#171717` | Kartu, panel, dialog |
| `--bg-inset` | `#121212` | Input, code block, terminal, log |
| `--bg-elevated` | `#262626` | Nav aktif, tombol sekunder, pill, hover |
| `--border` | `#2A2A2A` | Border kartu, input, dialog |
| `--border-soft` | `#222222` | Divider, sidebar, top bar |
| `--ring-gap` | `#0A0A0A` | Jarak 3 px pada ring kartu |
| `--ring-outer` | `#2E2E2E` | Border luar ring kartu/tombol |
| `--ring-inner` | `#232323` | Border dalam kartu/tombol/input |
| `--text-primary` | `#FAFAFA` | Judul dan teks utama |
| `--text-secondary` | `#A1A1A1` | Deskripsi dan body |
| `--text-muted` | `#8C8C8C` | Label, meta, placeholder |
| `--brand` | `#E07B28` | Link, ikon aktif, accent text |
| `--brand-soft` | `#EDAB4A20` | Selected/active tint |
| `--brand-fill` | `#973C00` | Tombol primer, switch on, progress |
| `--brand-fg` | `#FFFBEB` | Teks/ikon di atas `brand-fill` |
| `--success` | `#00D492` | Running/online |
| `--success-soft` | `#00D4921A` | Pill running |
| `--warning` | `#F5B544` | Connecting/pending/stale (turunan; desain belum punya) |
| `--danger` | `#FF6B6B` | Error/failed |
| `--bar-idle` | `#2A2A2A` | Bar chart idle |

**Penting:** `--accent` milik shadcn tetap berarti hover surface (`#262626`), bukan warna brand.
Warna brand memakai nama `--brand*` agar tidak bertabrakan dengan `bg-accent` yang sudah dipakai.

#### Pemetaan token lama

| Token lama | Nilai baru |
|---|---|
| `--background` | `#0A0A0A` |
| `--foreground` | `#FAFAFA` |
| `--card` / `--popover` | `#171717` |
| `--card-foreground` / `--popover-foreground` | `#FAFAFA` |
| `--primary` | `#E07B28` (link, ikon, focus) |
| `--primary-foreground` | `#0A0A0A` |
| `--secondary` | `#262626` |
| `--secondary-foreground` | `#FAFAFA` |
| `--muted` | `#1A1A1A` |
| `--muted-foreground` | `#8C8C8C` |
| `--accent` | `#262626` |
| `--accent-foreground` | `#FAFAFA` |
| `--border` / `--input` | `#2A2A2A` |
| `--ring` | `#E07B28` |
| `--success` | `#00D492` |
| `--warning` | `#F5B544` |
| `--destructive` | `#FF6B6B` |
| `--sidebar` | `#0A0A0A` |
| `--sidebar-accent` | `#262626` |
| `--sidebar-border` | `#222222` |
| `--radius` | `10px` |
| `--chart-1` | `#E07B28` |
| `--chart-2` | `#00D492` |
| `--chart-3` | `#8C8C8C` |
| `--chart-4` | `#A1A1A1` |
| `--chart-5` | `#2A2A2A` |

Chart code yang sekarang memakai `hsl(var(--chart-*))` harus diganti ke `var(--chart-*)`
atau hex langsung, karena token baru bukan format HSL.

### 3.2 Tipografi

- Font UI: **DM Sans** untuk body dan heading (menggantikan Public Sans pada heading).
- Font mono: **JetBrains Mono** untuk path, port, code, log, terminal, versi.
- Bobot: 400 body, 500 label/tombol, 600 judul dan nilai metric.

| Peran | Ukuran | Line height | Warna |
|---|---:|---:|---|
| Page title | 26 px | 1.2 | `text-primary` |
| Page subtitle | 12.5 px | 1.45 | `text-secondary` |
| Site/detail title | 22 px | 1.2 | `text-primary` |
| Card title | 14.5 px | 1.3 | `text-primary` |
| Card description | 12 px | 1.45 | `text-secondary` |
| Body | 12.5–13 px | 1.5 | `text-secondary` |
| Meta | 11.5 px | 1.4 | `text-muted` |
| Section label | 10.5 px, uppercase, tracking 0.08em | 1.2 | `text-muted` |
| Metric value | 26 px | 1.1 | `text-primary` |
| Button | 12.5 px | 1.0 | sesuai varian |
| Mono | 11.5–12 px | 1.55 | `text-secondary` |

Aturan: tidak ada teks body di bawah 12 px. 10–10.5 px hanya untuk label uppercase
non-esensial dan badge kecil.

### 3.3 Spacing, Radius, dan Elevation

| Elemen | Nilai |
|---|---|
| Sidebar width | 264 px |
| Top bar height | 60 px |
| Title bar desktop | 36 px (dipertahankan) |
| Page padding | 28 px; 24 px di <1280; 16 px di <768 |
| Page gap | 20 px |
| Card padding | 18 px vertikal, 22 px horizontal; compact 14/18 |
| Card header gap | 5 px |
| Card content gap | 14 px |
| Control height | sm 30 px, default 34 px, lg 38 px |
| Sidebar nav item | 33 px, radius 8, padding 8/10, gap 10 |
| Card outer radius | 17 px |
| Card inner radius | 14 px |
| Button radius | 10 px luar, 8 px dalam |
| Input radius | 8 px |
| Dialog radius | 16 px |
| Pill | full |

**Card ring:** border luar `1px --ring-outer`, padding 3 px dengan `--ring-gap`, lalu panel
dalam `1px --ring-inner` + `--bg-surface`. Implementasi boleh memakai wrapper internal
`Card` (aman terhadap `overflow`), bukan box-shadow yang bisa terpotong scroll container.
Sediakan varian `flat` untuk panel di dalam card, tabel, dan area kompak.

**Elevation:** hindari shadow besar. Gunakan border dan perbedaan surface. Shadow hanya untuk
dialog, dropdown, sheet, dan toast.

### 3.4 Ikon

- Lucide, ukuran 13–16 px untuk inline, 18–20 px untuk empty state.
- Warna ikon mengikuti teks; ikon aksen hanya untuk state aktif atau status.
- Jangan memakai ikon di dalam kotak berwarna kecuali identitas produk, setup, atau About.

---

## 4. Pola Komponen

### 4.1 Card / Panel

- Header: judul 14.5 px, deskripsi 12 px, aksi di kanan.
- Body: 12.5–13 px.
- Footer aksi: border-top `--border-soft`, padding atas 14 px.
- Card ring untuk section utama. `flat` untuk card di dalam card, tabel, code panel.
- Hindari nested card berlapis. Gunakan divider dan row.

### 4.2 Button

- Primary: `--brand-fill` + `--brand-fg`, label 600.
- Secondary: `--bg-elevated`, border `--ring-inner`, teks primary.
- Outline: transparent/inset, border `--border`, hover elevated.
- Ghost: hover `--bg-elevated`, teks secondary → primary.
- Destructive: tint danger, teks danger; bukan solid merah.
- Link: `--brand`, underline saat hover.
- Busy: spinner menggantikan ikon kiri, lebar tidak berubah.
- Ring luar 2 px hanya untuk CTA yang punya ruang; icon button dan aksi tabel memakai flat.

### 4.3 Badge / Status

- Pill tinggi 20–24 px, padding 3/9, gap 6, dot 5–6 px.
- Running: `success-soft` + `success`.
- Stopped/disabled: `bg-elevated` + `text-secondary`/`muted`.
- Starting/pending: warning tint.
- Error: danger tint.
- `ToneBadge` tetap menjadi satu-satunya definisi status.

### 4.4 Tabs

- Container: `bg-elevated`, border `ring-inner`, radius 14, padding 3, gap 4.
- Active: `#333333` + border `#3D3D3D`, radius 8, padding 6/12.
- Inactive: transparan, teks muted.
- Untuk tab halaman (Database, Site detail) gunakan segmented. Untuk source tabs yang panjang
  (Logs) tetap horizontal scroll.

### 4.5 Form

- Label 11.5–12 px, secondary.
- Input/select/textarea: `bg-inset`, border `ring-inner`, radius 8, padding 8/10, teks 12.5.
- Field row: gap 14, dua kolom pada layar lebar.
- Switch: 36 × 20, padding 3; on `brand-fill` + knob `brand-fg`; off `bg-elevated` +
  knob `text-muted`.
- Checkbox: 16 px, radius 4, checked `brand-fill`.
- Toggle row: border-top soft, padding 10/0, label + description di kiri.
- Error text: danger 11.5 px, tidak mengganti border seluruh form.

### 4.6 Table

- Selalu di dalam Card/Panel.
- Header: 10 px uppercase tracking, muted, height 32, border-bottom soft.
- Row: 44 px, border-bottom `ring-inner`, hover `bg-elevated/50`.
- Cell: padding 10/12, teks 12.5.
- Aksi kanan: gap 6, tombol sm + icon.
- Kolom kanan boleh punya lebar tetap; body boleh horizontal scroll.
- Empty row: tinggi 120–160, ikon + judul + deskripsi + aksi existing.

### 4.7 Dialog, Sheet, Dropdown, Toast

- Overlay: hitam 72% + blur 2 px.
- Dialog: `bg-surface`, border `--border`, radius 16, padding 22–24, title 18, desc 12.5.
- Dialog besar (New Site) maksimal 840 px, tinggi 90vh, body scroll.
- Footer: aksi sekunder kiri/kanan, primary kanan, gap 8.
- Sheet terminal: `bg-inset`, border kiri, header 44 px, panel terminal mengisi sisa.
- Dropdown: `bg-surface`, border, radius 10, padding 4, item 30 px, 12.5, hover elevated.
- Toast: `bg-surface`, border, radius 10, teks 12.5, ikon tone, bottom-right.
- Semua overlay mempertahankan focus trap dan Escape existing.

### 4.8 Code, Log, dan Terminal

- Panel: `bg-inset`, border `ring-inner`, radius 14.
- Header panel: path/file 11.5 mono, status pill, aksi di kanan.
- Body: mono 11.5–12, line-height 1.55, padding 16/20.
- Xterm: background `#121212`, foreground `#A1A1A1`, cursor `#E07B28`,
  selection `#E07B2833`, font 13 JetBrains Mono.
- Log level coloring tetap; jangan menambah parsing baru.

### 4.9 Chart

- Grid horizontal `--border-soft`, tanpa garis vertikal.
- Axis tick 10.5 px `--text-muted`.
- Tooltip surface gelap, border, teks 12.
- Traffic: brand untuk data aktif, `bar-idle` untuk bar kosong.
- Service status: success dan muted; garis enabled putus-putus.
- Jangan menghapus tooltip, legend data, atau rentang waktu.

### 4.10 Empty, Loading, Error

- Empty: panel surface + border `ring-inner`, radius 14, padding 40/22, ikon 18–20,
  judul 13, deskripsi 11.5, aksi existing.
- Loading: skeleton dengan radius dan tinggi final; tidak menggeser layout.
- Error: panel `danger` 10% + border 30%, ikon 16, judul 13, detail 11.5, retry existing.
- Busy state tidak boleh mengubah ukuran tombol.

---

## 5. Pemetaan Seluruh Surface

| Surface | Referensi `.pen` | Perlakuan |
|---|---|---|
| Title bar desktop | Tidak ada (canvas mulai di konten) | Palet baru, border-bottom soft, kontrol window 40 px. Tinggi 36 px dipertahankan. |
| Sidebar | Sidebar semua frame | 264 px, group label, item 33 px, active elevated, proxy card di footer. Collapse tetap. |
| Top bar | Top bar semua frame | 60 px, border-bottom, status DB/PHP/services/version/connection. Tanpa search/notif/avatar. |
| Page header | Page Header semua frame | Judul 26, subtitle 12.5, aksi kanan. Tambah komponen `PageHeader`. |
| Dashboard | Dashboard | Metric, server, daemon, service, additional services, chart, table, log dialog. |
| Sites | Sites | Toolbar, filter, table/card mobile, dialog create/configure/delete, terminal dock. |
| Site Overview | Site myapp — Overview | Header site, tabs, 4 tile, URL, public access, devtools summary. |
| Site Config | Site myapp — Config | Bento PHP/ini/document root/aliases/env/apply. |
| Site Logs | Logs | Source tabs, auto-refresh, path, log panel. |
| Site DevTools | Dashboard service card + Logs | Grid tool card, status, port/PID, start/stop, log panel. |
| Site Terminal | Terminal | Panel inset, status header, xterm theme baru. |
| Database — Daemons | Database | Segmented tabs, 2 daemon cards, SQLite flat card, note, port form. |
| Database — Backups | Database + tabel | Card header + Back Up Now, table/empty. |
| Database — Terminal | Terminal | Engine selector + status + terminal. |
| Services | Services + daemon card | Hint, empty, service cards, auto-start, env snippet. |
| Packages | Packages | System PHP/tools, registry error, grid, install progress/log. |
| Terminal | Terminal | Page header, shell tabs, new shell, terminal panel. |
| Logs | Logs | Page header, source tabs, auto-refresh, log panel. |
| SSL / HTTPS | SSL / HTTPS | 3 step cards, Firefox alert, status table, elevation dialog. |
| Settings | Settings | General form, PHP ini, profiles table, 2 kolom desktop. |
| About | About | Identity, updates, components, credits, paths, privacy, diagnostics. |
| Setup wizard | Tidak ada | Ekstrapolasi penuh dari card/form/button/log/setup pattern. |
| New Site dialog | Tidak ada | Dialog surface + field/select/checkbox + progress log. |
| Configure/Delete dialogs | Tidak ada | Dialog/alert dialog pattern. |
| Terminal dock (Sites) | Terminal | Sheet/panel inset, resizable tetap. |
| Toasts | Tidak ada | Surface/border/toast pattern. |
| Loading/error/empty | Sebagian | Empty/loading/error pattern di atas. |

---

## 6. Bagian yang Tidak Tercover dan Cara Menurunkannya

### 6.1 Setup wizard

- Layout: full window, tanpa sidebar/top bar; brand header centered max-width 1024.
- Card: ring panel; core components, settings, additional tools.
- Form: advanced TLD/ports memakai input/field pattern.
- Tool row: checkbox + icon + label + description; selected memakai `brand-soft` +
  border `brand`/30.
- Footer sticky: border-top, ringkasan pilihan, CTA primary.
- Install panel: status card + progress + log panel `bg-inset` + error panel.
- Tidak menambah langkah, pilihan, atau tool baru.

### 6.2 Site Logs, DevTools, Terminal

- Logs: ikuti halaman Logs; source tabs, auto-refresh, file path, panel log.
- DevTools: card per tool mengikuti daemon card; status pill, PID/port mono, start/stop,
  error warning, log panel hanya saat running.
- Terminal: header tipis + status koneksi + panel xterm; tinggi mengisi tab.

### 6.3 Site Config

- Bento 2 kolom; kiri PHP version, document root, aliases; kanan custom php.ini;
  bawah environment variables + apply.
- Semua field existing dipertahankan: PHP version, document root, aliases,
  environment variables, custom php.ini, apply.
- `PhpIniEditor` memakai textarea mono `bg-inset`.

### 6.4 Sites dialogs dan dock

- New Site: dialog lebar 720–840; section name/template/database/testing; log pembuatan
  memakai panel inset; tombol Cancel/Close/Create.
- Configure: field PHP version, php.ini, document root, aliases, env.
- Delete: alert dialog danger, nama site, Cancel/Delete.
- Terminal dock: resizable, panel terminal, tombol hide/restart/clear.
- Semua aksi dan validasi existing dipertahankan.

### 6.5 Database backups dan terminal

- Backups: card dengan header + CTA `Back Up Now`; table Name/Size/Time; empty state.
- Terminal: segmented engine selector; status pill; xterm panel.
- Jangan menambah restore/delete backup.

### 6.6 Services dan Packages

- Service card: title, port mono, status pill, Open UI (jika ada), Start/Stop,
  auto-start toggle, error. Mengikuti daemon card.
- Env snippet: card + tabs (jika >1) + code panel + copy button.
- Package card: label, version/license, status pill, install button. Grid 3/2/1.
- Install job: card dengan status, progress, log panel, close.
- System PHP/tools: card ring/flat dengan row; tetap read-only.

### 6.7 SSL, Settings, About

- SSL step cards: nomor/ikon, judul, deskripsi, CTA, status pill.
- Alert Firefox: callout netral/inset, bukan kartu besar.
- Status table: mengikuti table pattern.
- Elevation dialog: dialog + code block + copy.
- Settings: 2 kolom; General form, PHP ini, Profiles. Save tetap di footer General.
- Profiles: table + Apply; tidak menambah create/delete profile.
- About: identity/update/components/credits/paths/privacy/diagnostics; update progress
  memakai progress + log/pre; copy rows memakai mono + copy icon.
- Semua tombol/aksi existing dipertahankan.

### 6.8 State dan overlay

- Loading: skeleton per section; tidak memakai spinner halaman penuh kecuali detail site
  yang sekarang.
- Error: inline panel + retry existing.
- Empty: panel centered + aksi existing.
- Dialog/sheet/dropdown/toast: pola di 4.7.
- Fokus keyboard: ring 2–3 px `brand`/40, offset 1–2 px.
- `prefers-reduced-motion`: durasi 0–100 ms untuk transisi non-esensial.

---

## 7. Rencana Implementasi

### Fase 0 — Baseline dan guardrails

**Tujuan:** memastikan tidak ada fitur yang hilang.

- [ ] Inventaris fitur per halaman (checklist di Lampiran B).
- [ ] Simpan screenshot baseline 1280 × 800 dan 1440 × 900.
- [ ] Konfirmasi daftar penyimpangan di 2.4.
- [ ] Tetapkan `PageHeader` dan struktur scroll.
- [ ] Jangan mengubah `untitled.pen`.

**Exit:** daftar surface dan fitur disetujui.

### Fase 1 — Token dan primitives

**File:**
- `internal/dashboard/ui/src/index.css`
- `src/components/ui/button.tsx`
- `src/components/ui/card.tsx`
- `src/components/ui/badge.tsx`
- `src/components/ui/tabs.tsx`
- `src/components/ui/input.tsx`
- `src/components/ui/textarea.tsx`
- `src/components/ui/select.tsx`
- `src/components/ui/switch.tsx`
- `src/components/ui/checkbox.tsx`
- `src/components/ui/table.tsx`
- `src/components/ui/dialog.tsx`
- `src/components/ui/alert-dialog.tsx`
- `src/components/ui/sheet.tsx`
- `src/components/ui/dropdown-menu.tsx`
- `src/components/ui/alert.tsx`
- `src/components/ui/progress.tsx`
- `src/components/ui/empty.tsx`
- `src/components/ui/skeleton.tsx`
- `src/components/ui/sonner.tsx`
- `src/components/status.tsx`
- `src/components/icon-button.tsx`
- `src/components/error-boundary.tsx`

**Tugas:**
- [ ] Ganti token warna, chart, radius, font.
- [ ] Tetapkan type scale dan `font-heading` = DM Sans.
- [ ] Audit semua pemakaian langsung `bg-primary` / `text-primary` agar sesuai pemisahan
      `--brand`, `--brand-fill`, dan `--accent` (termasuk error boundary dan drag handle).
- [ ] Implementasikan Card ring/flat.
- [ ] Implementasikan Button variant/size baru dan state busy.
- [ ] Implementasikan segmented Tabs.
- [ ] Implementasikan Input/Select/Switch/Checkbox/Field.
- [ ] Implementasikan Table density.
- [ ] Implementasikan Dialog/Sheet/Dropdown/Toast.
- [ ] Implementasikan Empty/Error/Skeleton.
- [ ] Perbaiki chart color format.
- [ ] Pastikan focus ring dan kontras.

**Exit:** komponen dasar tampil konsisten di halaman existing; build/typecheck hijau.

### Fase 2 — Shell

**File:**
- `src/App.tsx`
- `src/components/app-titlebar.tsx`
- `src/components/app-sidebar.tsx`
- `src/components/site-header.tsx`
- `src/components/ui/sidebar.tsx` (jika perlu)
- `src/index.css`
- baru: `src/components/page-header.tsx`
- baru: `src/components/page-shell.tsx` (opsional)

**Tugas:**
- [ ] Title bar 36 px dengan palet baru.
- [ ] Sidebar 264 px, group label, active item, proxy card.
- [ ] Top bar 60 px dengan status live existing.
- [ ] Pindahkan judul halaman ke `PageHeader`; top bar tidak lagi memuat title.
- [ ] Pertahankan breadcrumb site di site detail.
- [ ] Pertahankan sidebar collapse dan window controls.
- [ ] Tetapkan padding/gap halaman.
- [ ] Update PageSkeleton.

**Exit:** semua halaman memakai shell dan page header yang sama; tidak ada fitur shell hilang.

### Fase 3 — Dashboard

**File:** `src/pages/dashboard.tsx`

- [ ] Page header + Refresh.
- [ ] 4 metric cards; database card tetap clickable.
- [ ] Error banner.
- [ ] Server card + Start/Stop All.
- [ ] MariaDB/PostgreSQL cards dengan Start/Stop/Manage.
- [ ] Service card per installed service + logs.
- [ ] Additional services callout + link Packages (tanpa chip install).
- [ ] Traffic chart (bar opsional) + Service status chart (area/step).
- [ ] Installed services table + empty row.
- [ ] Log dialog.
- [ ] Responsive 4→2→1.

**Exit:** seluruh aksi dan data dashboard lama masih ada; visual sesuai pola.

### Fase 4 — Sites dan Site Detail

**File:** `src/pages/sites.tsx`, `src/pages/site-detail.tsx`

Sites:
- [ ] Page header + toolbar (filter, Terminal, New Site).
- [ ] Table desktop + card mobile.
- [ ] URL/aliases/tunnel badge.
- [ ] Row menu: view, restart, configure, terminal, share, logs, delete.
- [ ] New Site dialog + progress log.
- [ ] Configure dialog.
- [ ] Delete alert.
- [ ] Terminal dock/resizable sheet.

Site detail:
- [ ] Site header: name, status, URLs, actions existing.
- [ ] Segmented tabs (Overview, Config, Logs, Dev Tools, Terminal).
- [ ] Overview: 4 tile, URLs, Public access, running devtools.
- [ ] Config: PHP version, document root, aliases, php.ini, env, apply.
- [ ] Logs: source tabs + auto-refresh + panel.
- [ ] DevTools: tool cards + logs.
- [ ] Terminal: inset panel + status.

**Exit:** 5 tab berfungsi; semua field/aksi lama ada; dialog tetap accessible.

### Fase 5 — Database, Services, Packages

**File:** `src/pages/database.tsx`, `src/pages/services.tsx`, `src/pages/packages.tsx`

- [ ] Database: segmented tabs Daemons/Backups/Terminal.
- [ ] Daemon cards: enable switch, state, port input/save, start/stop/restart, installed warning.
- [ ] SQLite flat card + note.
- [ ] Backups table + empty + Back Up Now.
- [ ] Database terminal engine selector.
- [ ] Services: hint, empty, service cards, auto-start, Open UI, error.
- [ ] Env snippet + copy.
- [ ] Packages: system PHP/tools, registry error, grid, install job/progress/log.
- [ ] Responsive grid.

**Exit:** semua aksi database/service/package sama; tidak ada restore/delete/install baru.

### Fase 6 — Terminal, Logs, SSL, Settings, About

**File:** `src/pages/terminal.tsx`, `logs.tsx`, `ssl.tsx`, `settings.tsx`, `about.tsx`, `components/terminal-panel.tsx`

- [ ] Terminal: page header, shell tabs, new shell, xterm theme.
- [ ] Logs: page header, source tabs, auto-refresh, file path, log panel.
- [ ] SSL: 3 step cards, Firefox callout, status table, elevation dialog.
- [ ] Settings: 2 kolom, General, PHP ini, Profiles table.
- [ ] About: identity, updates/progress, components, credits, paths, privacy, diagnostics.
- [ ] Table/copy row/code panel patterns.

**Exit:** semua fitur system pages utuh; tidak ada aksi baru.

### Fase 7 — Setup wizard dan overlays lintas halaman

**File:** `src/pages/setup.tsx`, `sites.tsx`, `site-detail.tsx`, `ssl.tsx`, `packages.tsx`

- [ ] Setup wizard full-window: core, settings, additional tools, footer, install panel.
- [ ] Semua dialog memakai surface/field/button baru.
- [ ] Terminal dock sheet.
- [ ] Dropdown/select/checkbox/toast.
- [ ] Empty/loading/error di semua halaman.
- [ ] Pastikan tidak ada dialog yang kehilangan focus trap/escape.
- [ ] Pastikan tidak ada form yang kehilangan label/error.

**Exit:** first-run flow dan semua overlay konsisten; tidak ada fitur baru.

### Fase 8 — QA dan handoff

> Hasil lengkap ada di [Lampiran E](#lampiran-e--laporan-qa-fase-8).

- [x] Visual QA 1440 × 900, 1280 × 800, 1024 × 768, 960 × 640, 390 px web.
- [x] Semua halaman: populated, empty, loading, error.
- [x] Running/stopped/starting/error states.
- [x] Dialog create/configure/delete/elevation/install.
- [x] Keyboard/focus/contrast.
- [x] Tidak ada clipped text/overflow.
- [x] `npm run typecheck`, `npm run lint`, `npm run build`.
- [x] Smoke test `go test ./...` bila relevan.
- [x] Update screenshot `images/ui/*` setelah implementasi (opsional, bukan fitur).
- [x] Update dokumen ini dengan keputusan final.

**Exit:** definition of done terpenuhi.

---

## 8. Responsive Rules

| Width | Perilaku |
|---|---|
| ≥1440 | Layout acuan; 4 metric, 3 package column, 2 chart, 2 settings column. |
| 1280–1439 | Padding 24, gap 20; metric tetap 4 bila container cukup, jika tidak 2. |
| 1024–1279 | Sidebar tetap/offcanvas sesuai shell; metric 2; package 2; chart stack bila sempit. |
| 960–1023 | Window minimum desktop; sidebar offcanvas; table horizontal scroll; dialog 90vw. |
| <768 (web) | Site list menjadi card (existing); tab horizontal scroll; form 1 kolom; aksi wrap. |

Aturan:
- Gunakan container query existing (`@xl/main`, `@5xl/main`) agar layout mengikuti area konten,
  bukan viewport.
- Jangan memakai lebar persen pada komponen layout; gunakan `fill_container`/grid.
- Jangan memaksa 4 kolom di 960 px.
- Top bar boleh menyembunyikan versi/uptime di layar sempit; status DB/PHP tetap.
- Sidebar trigger harus tetap terlihat.

---

## 9. Aksesibilitas dan State

- Kontras teks body minimal 4.5:1. Palet baru:
  - `text-secondary` pada `bg-surface` ≈ 6.9:1.
  - `text-muted` pada `bg-surface` ≈ 5.3:1.
  - brand pada `bg-surface` ≈ 6.0:1.
  - `brand-fg` pada `brand-fill` ≈ 6.8:1.
- Jangan mengandalkan warna saja: dot selalu dengan label.
- Focus visible 2–3 px, jangan dihapus.
- Target klik minimal 28 × 28 px; icon button 28.
- Dialog: focus trap, Escape, restore focus.
- Tab: arrow key, `aria-selected`.
- Tabel: header semantik, action button punya label.
- Loading: `aria-busy`/skeleton; error: role alert.
- Tidak ada layout shift saat status berubah (lebar pill/angka tabular).

---

## 10. QA Checklist

### Visual
- [ ] Card ring konsisten; tidak ada ring terpotong.
- [ ] Radius, border, spacing sesuai token.
- [ ] Tidak ada teks terpotong/overflow.
- [ ] Tidak ada card di dalam card yang berlebihan.
- [ ] Icon dan teks sejajar.
- [ ] Chart tetap terbaca dan tooltip berfungsi.
- [ ] Table header/row rapi di semua lebar.
- [ ] Dialog tidak melebihi viewport.

### Fungsional
- [ ] Semua checklist Lampiran B lulus.
- [ ] Tidak ada API/route/field/action baru.
- [ ] Semua dialog existing masih bisa dibuka/ditutup.
- [ ] Terminal tetap connect/reconnect.
- [ ] Log auto-refresh tetap berjalan.
- [ ] Public access/tunnel tetap berfungsi.
- [ ] Backups/profiles/update flow tetap berfungsi.

### Build
- [ ] `npm run typecheck`
- [ ] `npm run lint`
- [ ] `npm run build`
- [ ] `go test ./...` (jika relevan)

---

## 11. Risiko dan Mitigasi

| Risiko | Mitigasi |
|---|---|
| Teks 10–12 px terlalu kecil di 1280 | Floor 12 px body/meta; 10.5 hanya label uppercase; uji di 1280 dan scaling 125%. |
| Ring card bertabrakan/terpotong | Wrapper internal Card, padding halaman cukup; varian flat untuk area rapat. |
| Double ring tombol di toolbar | Ring hanya untuk CTA; icon/table action memakai flat. |
| Pindah judul ke body merusak breadcrumb | `PageHeader` mendukung breadcrumb; site detail tetap punya nama + link Sites. |
| Chart token invalid | Hapus `hsl(var(--chart-*))`; gunakan `var(--chart-*)`/hex. |
| Scroll container full-bleed rusak | Jangan mengubah pemilik scroll di fase visual; audit per halaman. |
| Setup wizard tidak punya referensi | Gunakan pola card/form/log yang sama; review terpisah. |
| Scope creep ke fitur | Guardrail 2.2 dan checklist fitur. |
| Light theme tidak ada | Tetap dark-only; jangan menambah theme toggle. |
| Copy mock ikut masuk | Gunakan data live; jangan salin `.wowo`, port 80, atau angka mock. |

---

## 12. Definition of Done

1. Seluruh halaman memakai token, tipografi, spacing, card, button, tabs, table, form,
   dialog, dan toast yang sama.
2. Tidak ada fitur/aksi/field/tab/route/API yang hilang atau bertambah.
3. Setup wizard, site logs/devtools/terminal, dan semua dialog sudah mengikuti pola baru.
4. Semua state (loading, empty, error, running, stopped, starting) tampil konsisten.
5. Layout berfungsi di 1440, 1280, 1024, dan 960 px; tidak ada overflow yang tidak disengaja.
6. Kontras dan focus state lulus; keyboard navigation tidak rusak.
7. Build, typecheck, dan lint hijau.
8. Screenshot baseline/after tersedia untuk review.

---

## Lampiran A — Ringkasan File yang Disentuh

**Foundation**
- `src/index.css`
- `src/components/ui/*` (button, card, badge, tabs, input, textarea, select, switch,
  checkbox, table, dialog, alert-dialog, sheet, dropdown-menu, alert, progress, empty,
  skeleton, sonner)
- `src/components/status.tsx`
- `src/components/icon-button.tsx`
- baru: `src/components/page-header.tsx`
- baru: `src/components/page-shell.tsx` (opsional)

**Shell**
- `src/App.tsx`
- `src/components/app-titlebar.tsx`
- `src/components/app-sidebar.tsx`
- `src/components/site-header.tsx`
- `src/components/ui/sidebar.tsx` (jika perlu)

**Pages**
- `src/pages/dashboard.tsx`
- `src/pages/sites.tsx`
- `src/pages/site-detail.tsx`
- `src/pages/database.tsx`
- `src/pages/services.tsx`
- `src/pages/packages.tsx`
- `src/pages/terminal.tsx`
- `src/pages/logs.tsx`
- `src/pages/ssl.tsx`
- `src/pages/settings.tsx`
- `src/pages/about.tsx`
- `src/pages/setup.tsx`
- `src/components/terminal-panel.tsx`

**Tidak disentuh**
- Backend Go, API, config, database, proxy, routes, dan `untitled.pen`.

---

## Lampiran B — Checklist Fitur yang Wajib Dipertahankan

### Shell
- [ ] Window minimize/maximize/close.
- [ ] Sidebar collapse/offcanvas.
- [ ] Navigasi 10 halaman.
- [ ] Status DB, PHP, active services, versi, uptime, connection dot.
- [ ] Proxy/clean URL indicator.

### Dashboard
- [ ] Refresh.
- [ ] Start All / Stop All.
- [ ] Start/Stop/Restart MariaDB dan PostgreSQL.
- [ ] Manage database link.
- [ ] Start/Stop service, auto-start, logs.
- [ ] Additional services link ke Packages.
- [ ] Traffic chart + service history chart.
- [ ] Installed services table.
- [ ] Log dialog.
- [ ] Error banner.

### Sites
- [ ] Filter.
- [ ] Terminal dock show/hide/resize/restart/clear.
- [ ] New Site: nama, template, database, testing, progress log.
- [ ] Start/Stop.
- [ ] Row menu: view, restart, configure, terminal, share, logs, delete.
- [ ] Configure: PHP, php.ini, document root, aliases, env.
- [ ] Delete confirmation.
- [ ] URL dan aliases.
- [ ] Tunnel/public badge.

### Site detail
- [ ] Start/Stop/Restart.
- [ ] Overview: framework, PHP, database, directory, URLs, public access, devtools.
- [ ] Config: PHP version, document root, aliases, php.ini, env, apply.
- [ ] Logs: source tabs, auto-refresh, file, panel.
- [ ] Dev Tools: start/stop, PID/port, error, log.
- [ ] Terminal: connect/reconnect, status.

### Database
- [ ] Enable/disable MariaDB/PostgreSQL.
- [ ] Port edit + save.
- [ ] Start/Stop/Restart.
- [ ] Installed warning/error.
- [ ] SQLite info.
- [ ] Backups list + Back Up Now.
- [ ] Terminal engine selector.
- [ ] Note env vars.

### Services
- [ ] Start/Stop.
- [ ] Auto-start switch.
- [ ] Open UI.
- [ ] Last error.
- [ ] Env snippet tabs + copy.
- [ ] Empty state + Packages link.

### Packages
- [ ] System PHP detection.
- [ ] System tools.
- [ ] Registry error.
- [ ] Install.
- [ ] Progress + log.
- [ ] Status active/installed/not installed.

### Terminal
- [ ] Multiple shell tabs.
- [ ] New shell.
- [ ] Close tab.
- [ ] Reconnect/replay.
- [ ] Clear/restart hanya di terminal dock Sites, sesuai UI yang sudah ada.

### Logs
- [ ] Source tabs (sites/database/mailpit).
- [ ] Query param `?site=`.
- [ ] Auto-refresh.
- [ ] File path.
- [ ] Error/no logs.

### SSL
- [ ] Generate CA.
- [ ] Issue wildcard.
- [ ] Trust CA.
- [ ] Status table.
- [ ] Elevation dialog + copy command.
- [ ] Firefox note.

### Settings
- [ ] Save general (TLD, ports, dashboard port, auto-open, LAN).
- [ ] PHP ini load/save.
- [ ] Profiles table + Apply.

### About
- [ ] Refresh.
- [ ] Check updates.
- [ ] Update now + progress + kind.
- [ ] Channel stable/beta.
- [ ] Components.
- [ ] Credits/links.
- [ ] Paths + copy.
- [ ] Privacy hosts.
- [ ] Diagnostics + copy.

### Setup
- [ ] Core components.
- [ ] Sample site.
- [ ] Advanced TLD/ports.
- [ ] Additional tools selection.
- [ ] Finish setup.
- [ ] Install progress/log.
- [ ] Error reload.

---

## Lampiran C — Keputusan yang Perlu Dikonfirmasi Sebelum Fase 2

> **Status: sudah dikonfirmasi** (dipakai sebagai acuan implementasi di branch `revamp/ui-ux`).

1. **Page title:** pindah ke body (rekomendasi, mengikuti desain) atau tetap di top bar?
   → **Dipindah ke body.** Setiap halaman punya `PageHeader` (judul + deskripsi + aksi); top bar menjadi strip status global. Breadcrumb "Sites › <nama>" tetap ada di Site Detail supaya navigasi tidak hilang.
2. **Traffic chart:** ubah ke bar (rekomendasi) atau pertahankan area?
   → **Diubah ke bar chart** (batang brand, radius 3px). Grafik status service tetap area/step karena itu data historis, bukan volume.
3. **Ring card:** semua card (rekomendasi: page card ring, card dalam flat)?
   → **Ring untuk page card, flat/inset untuk panel ringkas.** `Card` punya `variant="ring" | "flat" | "inset"`; panel nested di dalam page memakai `border-ring-inner` + `bg-surface-inset` (setara flat).
4. **Tipografi:** floor 12 px body/meta (rekomendasi) atau 1:1 dengan desain 10–12 px?
   → **Floor 12 px** untuk body/meta; label tabel dan badge boleh 10–11 px sebagai elemen dekoratif. Body default 13 px.
5. **Copy:** pertahankan bahasa existing (rekomendasi) atau samakan ke Inggris?
   → **Bahasa existing dipertahankan.** Hanya label baru yang mengikuti pola teks yang sudah ada.

---

## Lampiran D — Catatan Implementasi (branch `revamp/ui-ux`)

### Yang sudah dikerjakan

| Fase | Isi | Status |
|---|---|---|
| 1 | Token (`index.css`), tipografi, `Card` (ring/flat/inset), `Button`, `Badge`, `Tabs`, `Input`, `Textarea`, `Select`, `Switch`, `Checkbox`, `Table`, `Dialog`, `AlertDialog`, `Sheet`, `DropdownMenu`, `Alert`, `Progress`, `Empty`, `Skeleton`, `Tooltip`, `Sonner`, `Status`/`ToneBadge` | Selesai |
| 2 | Shell: title bar, sidebar (264 px, grup + footer proxy), top bar (60 px, hanya status), `PageHeader` baru, layout `PageLayout`/`FullBleedLayout` | Selesai |
| 3 | Dashboard: metrik ring card, kartu server/DB/service, traffic jadi bar chart, status service tetap area | Selesai |
| 4 | Sites: `PageHeader`, toolbar, tabel ring, kartu mobile, dock terminal; Site Detail: breadcrumb, header deskriptif, tab Config jadi bento 2 kolom, Logs/DevTools/Terminal | Selesai |
| 5 | Database (daemon/backups/terminal), Services, Packages | Selesai |
| 6 | Terminal (multi-shell), Logs (tab per sumber), SSL (3 langkah + status), Settings (General/php.ini/Profiles), About | Selesai |
| 7 | Setup wizard: kartu ring, baris komponen, panel log, panel install | Selesai |
| 8 | QA | typecheck, lint, dan build hijau; checklist fungsional di Lampiran B tetap perlu diverifikasi manual di aplikasi berjalan |

### Token baru yang dipakai di seluruh app

`bg-surface`, `bg-surface-raised`, `bg-surface-inset`, `border-border-soft`, `border-ring-inner`, `border-ring-outer`,
`bg-brand`, `bg-brand-fill`, `bg-brand-soft`, `text-brand`, `text-brand-fg`, `bg-success-soft`, `bg-warning-soft`,
`bg-danger-soft`, `text-text-primary`, `text-text-secondary`, `text-text-muted`.

### Catatan teknis

- `Card` kini membungkus panel dalam wrapper ber-ring; `className` pada `Card` menempel ke wrapper luar, panel dalam punya `data-slot="card-panel"` dan variabel `--card-pad-x/y`, `--card-gap`.
- `index.css` hanya memuat satu palet (dark-only). `html` sudah `class="dark"`, jadi varian `dark:` tetap berlaku.
- Font: DM Sans (UI + heading) dan JetBrains Mono (kode). Public Sans dihapus dari import CSS.
- Scrollbar tetap disembunyikan (gaya desktop app); halaman yang panjang digulir lewat `PageLayout` (`overflow-y-auto`) atau kolom internalnya.
- Tidak ada perubahan backend, route, tab, field, atau aksi. `untitled.pen` tidak dimodifikasi.

---

## Lampiran E — Laporan QA (Fase 8)

### Metode

- Backend nyata dijalankan dari repo (`SABDOPALON_DIR=<repo> go run ./cmd/sabdopalon`, dashboard di `:9900`), UI dijalankan lewat Vite dev server (`:5199`, proxy `/api`).
- Screenshot diambil dengan Chrome headless yang dikendalikan lewat Chrome DevTools Protocol, jadi tab yang sama dipakai untuk semua rute: SSE tetap hidup, jadi snapshot live tidak berubah antar-halaman.
- Interaksi (buka dialog, klik menu baris, start/stop site, tekan Tab) dijalankan lewat `Runtime.evaluate`/`Input.dispatchKeyEvent` pada halaman yang sama.

### Cakupan

| Area | Yang diuji | Hasil |
|---|---|---|
| Lebar | 1440×900, 1280×800, 1024×768, 960×640, 390×844 | Tidak ada overflow horizontal; kartu metrik 4→2 kolom ikut lebar kontainer (container query), tabel jadi kartu di <768, tab dan header wrap rapi |
| Halaman | 12 route + setup wizard | Semua memakai `PageHeader`, ring card, dan token yang sama |
| State | populated, loading (skeleton), empty (Services/Packages), error (backend mati → titik merah + "DB —"), running, stopped | Konsisten; status selalu_dot + label, tidak pernah warna saja |
| Overlay | New Site, Configure, Delete (AlertDialog), dropdown baris, tab, segmented control | Dialog tidak melewati viewport; fokus terperangkap; overlay blur + surface baru |
| Keyboard | Tab walk dari atas | Urutan fokus masuk ke sidebar lalu konten; focus ring brand terlihat (orange) |
| Build | `npm run typecheck`, `npm run build`, `npm run lint`, `go test ./...` | Hijau; lint 12 error (semua pola lama: `react-refresh`, `set-state-in-effect`, `refs`) — 2 lebih sedikit dari baseline repo (14) |

### Kontras (WCAG AA, rasio dihitung dari token final)

| Pasangan | Rasio |
|---|---|
| `text-primary` #FAFAFA di `surface` | 17.18:1 |
| `text-secondary` #A1A1A1 di `surface` | 6.94:1 |
| `text-muted` #8C8C8C di `surface` | 5.33:1 |
| `text-muted` #8C8C8C di app background | 5.89:1 |
| `brand` #E07B28 di `surface` | 6.00:1 |
| `brand-fg` #FFFBEB di `brand-fill` #973C00 | 6.84:1 |
| `success` #00D492 di `success-soft` | 7.81:1 |
| `warning` #F5B544 di `warning-soft` | 8.16:1 |
| `danger` #FF6B6B di `danger-soft` | 5.68:1 |

Semua ≥ 4.5:1.

### Temuan dan perbaikan saat QA

- **Versi Composer tampil dengan kode ANSI.** `composer --version` mengembalikan string berwarna (`\x1b[32m…`), yang tidak terlihat di terminal tapi jadi sampah di halaman Packages. Ditambahkan `stripAnsi()` di `src/lib/utils.ts` dan dipakai saat merender `version`. Perbaikan tampilan saja — tidak ada perubahan API atau backend.
- Judul halaman, filter, dan aksi utama sudah berada di header (sudah ada di commit sebelumnya).

### Yang tidak bisa diverifikasi dari browser

- Title bar desktop Tauri (hanya muncul di shell desktop) dan perilaku minimize/maximize/close.
- Dialog yang hanya muncul setelah aksi backend (elevation dialog SSL, panel install Packages) — alurnya mengikuti komponen yang sama, tapi tidak dipicu di QA ini.
- Checklist fitur di Lampiran B tetap perlu dicentang manual di aplikasi berjalan (butuh stack PHP/MariaDB terpasang).

### Screenshot

`images/ui/*.png` (10 file) diperbarui ke desain baru pada 1280×800 — dipakai README.

---

## Lampiran F — Kesetiaan Visual terhadap `untitled.pen`

Revamp pertama dibangun dari dokumen ini. Setelah itu file desainnya dibaca ulang
node per node (lewat pen.dev MCP: `Get`, `GetVariables`, `Export`) dan
dibandingkan layar per layar dengan hasil implementasi. Hasilnya: palet dan
token sudah benar sejak awal, tetapi **strukturnya** belum — dan itu yang
membuat tampilannya terasa "mirip, tapi bukan yang sama".

### Yang ditemukan berbeda (dan sudah diperbaiki)

| Aspek | File desain | Implementasi pertama | Sekarang |
|---|---|---|---|
| Tombol | dua lapis: edge luar 1px + celah 2px di background + permukaan tombol dengan edge sendiri | satu lapis | ring digambar dengan `box-shadow` (tetap satu elemen, fokus/disabled tidak berubah) |
| Empat angka utama | **satu** kartu dengan 4 sel dan pembatas 1px | empat kartu terpisah | `MetricStrip`: satu kartu, pembatas jadi celah 1px di atas warna pembatas |
| Label metrik | 11.5 px / 500 / `#A1A1A1`, huruf biasa | 10.5 px uppercase muted | sama dengan desain |
| Ikon | polos (15 px, aksen) | di dalam kotak dengan border | polos, seperti desain |
| Judul kartu | 14.5 px / 600 | 14.5 px / 500 | 600 |
| Sidebar aktif | ring + panel `#262626`, ikon 16 px aksen, label 13 px/600 | panel datar, ikon 15 px | ring + ikon 16 px aksen + label 13 px/600 |
| Kartu proxy | ring + isi `#121212` (inset) | datar | ring + inset |
| Top bar | pill = panel `#171717` + border `#222222`; panel tombol 28×28 | pill elevate | pill panel + panel tombol 28×28 |
| Trail | breadcrumb di top bar (hanya di halaman site) | breadcrumb di dalam halaman | dipindah ke top bar, seperti desain |
| Dashboard | urutan: metrik → server → daemon → installed services → additional services → chart | installed services di paling bawah | urutan sama dengan desain, tabel punya baris kosong |
| Settings | bento: General 2/3 + php.ini & profiles 1/3 | satu kolom | bento |
| Config site | bento 5/7 dengan kartu "Apply changes" | grid 2 kolom | bento + kartu Apply |
| About | daftar komponen jadi grid 3 kolom + tile logo | daftar baris | grid tile + tile logo |
| Terminal | tab shell berupa segmented control | tombol biasa | segmented control + tombol "New shell" berlabel |
| Subjudul halaman | 13 px | 12 px | 13 px |
| Baris tabel | 54 px, header 10 px/600 | 44 px, header 10 px/500 | 54 px, header 10 px/600 |

### Yang tetap tidak diambil (dan kenapa)

- **Kotak pencarian di top bar, lonceng notifikasi, avatar** — tidak ada
  backendnya; memaksakan UI yang tidak bisa bekerja lebih buruk daripada
  menyisihkannya.
- **Chip instalasi cepat di kartu "Additional services"** — aksi yang belum ada
  di aplikasi.
- **"Service status" berupa daftar service dengan bar mini** di desain — versi
  aplikasi menyimpan *riwayat* running/enabled, jadi digrafik tetap.
- **Tombol "Clear" di halaman Terminal** — keputusan lama (Lampiran 2.2) tetap
  berlaku: Clear hanya ada di terminal dock Sites.

### Cara membandingkan

`Export` frame dari `untitled.pen` ke PNG, lalu screenshot aplikasi yang
sama pada lebar yang sama, keduanya dilihat berdampingan. Warna diambil dari
node desain (`Get` → `fill`/`stroke`), bukan dari mata — contoh: fill tombol
primary sudah `#973C00` sejak awal, hanya struktur dua-lapisnya yang belum.

---

## Lampiran G — Putaran Kesetiaan Terakhir (setelah.umpan balik pengguna)

### 1. Judul halaman kembali ke top bar

Keputusan Lampiran C butir 1 (judul pindah ke body) **diubah**: judul halaman
lagi di top bar, bersama ikon halamannya, dan **baris detail kecil di bawahnya
dihapus** — halaman langsung dibuka dengan apa yang bisa dilakukan di halaman
itu. `PageHeader` berubah jadi `PageActions` (`src/components/page-actions.tsx`):
hanya baris aksi, rata kanan, di atas isi halaman.

Untuk halaman site, top bar menampilkan trail `Sites / <nama>` (sesuai desain),
dan status running/stopped pindah ke baris aksi halaman site (karena status
bukan judul, dan tetap harus terlihat di tab mana pun).

### 2. Skala tipografi dinaikkan

| Peran | Lama | Baru |
|---|---|---|
| Body | 13 px | 14 px |
| Judul kartu | 14.5 px | 15.5 px |
| Label form / header tabel | 12.5 / 10 px | 13.5 / 11 px |
| Isi tabel | 12.5 px | 13.5 px |
| Label metrik | 11.5 px | 12.5 px |
| Teks terkecil di layar | 10 px | 11.5 px |

### 3. Hierarki tombol mengikut desain

Aturan dari desain: **hanya satu tombol oranye per layar atau per kartu** —
tombol aksi utama. Semua tombol lain adalah tombol *surface* (isi `#262626`,
edge `#232323`, dibungkus ring).

| Tombol |Antes | Sesudah |
|---|---|---|
| Start/Stop di baris tabel Sites | oranye | surface |
| Install di kartu paket | oranye | surface |
| Save port | surface | surface (tetap) |
| Save php.ini | oranye | surface |
| Start di tabel Installed services | oranye | surface |
| Langkah SSL | semua oranye | hanya langkah 1 yang bisa jalan; sisanya surface |

### 4. Scroll halaman site

Sebelumnya hanya panel tab yang bisa digulir; bagian di bawah navbar (baris
status, tab) diam. Sekarang **satu halaman** di bawah top bar yang bergulir:
header, tab, dan isi bergerak bersama. Tab Terminal memakai tinggi sendiri
(70vh, min 22rem) supaya xterm tetap punya ruang.

### 5. Perbaikan kecil

- Halaman **Database** kehilangan padding atas setelah judulnya dihapus →
  tab Daemons menempel ke navbar. Dipperbaiki (`pt-7`).
- Tombol **New shell** di halaman Terminal dihapus dari kanan; tombol **+**
  masuk ke dalam segmented control tab, seperti pada desain.
- `npm run typecheck` memakai `tsc --noEmit` di atas tsconfig *solution-style*,
  sehingga **tidak memeriksa apa pun**. Diganti `tsc -b` (yang dipakai build).

### 6. Judul grup di sidebar dihapus

Label **MANAGE / TOOLS / SYSTEM** dihapus. Tiga label kecil itu memecah daftar
sembilan halaman jadi tiga daftar pendek tanpa menambah informasi apa pun —
halaman yang tersedia sudah diketahui, dan jaraknya antar kelompok tetap terasa.
Sidebar sekarang satu daftar bersih, dengan jarak antar kelompok yang lebih longgar.

### 7. Bento Settings: php.ini jadi kartu terlebar

Susunan Settings 최종:

```
[ General (5/12) ][ PHP configuration php.ini (7/12) ]
[ Profiles (12/12)                                   ]
```

Kartu php.ini yang paling lebar (kodenya butuh ruang — versi窄 = 12/24 dan
baris panjang membungkus), dan **tinggi kartu General = tinggi kartu php.ini**
(grid `items-stretch`, editor php.ini `flex-1` sehingga keypadnya ikut meregang).

### 8. Refresh pindah ke navbar, pill jadi tombol ber-ring

- Tombol **Refresh** di halaman Dashboard dipindah ke **top bar**, tepat di
  sebelah kanan judul halaman. Halaman yang punya sesuatu untuk dimuat ulang
  mendaftarkan aksinya lewat `PageRefreshProvider`
  (`src/components/page-refresh.tsx`): tombol di navbar muncul selama ada
  halaman yang terdaftar, dan menghilang lagi saat pindah halaman. Halaman
  Dashboard kini langsung dibuka dengan strip metrik — tanpa baris tombol
  yang hanya berisi satu tombol.
- Pill status di top bar (`mariadb`, `PHP 8.5.5`, `v0.12.0 · up …`) sekarang
  memakai **ring yang sama dengan tombol** (edge luar 1px + celah 2px + edge
  sendiri), jadi seluruh bar bagian kanan terbaca sebagai kumpulan tombol
  dari bahasa yang sama.

### 9. Tabel Sites: baris berselang

Baris tabel Sites sekarang berselang: baris genap diberi `surface-raised/25` di atas
kartu, baris ganjil dibiarkan transparan. Hover tetap menang (diperiksa di DOM:
baris genap `0.25` → `0.6` saat di-hover), jadi selisih tidak pernah menutupi
feedback mouse.
