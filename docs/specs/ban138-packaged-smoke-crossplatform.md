# BAN-138: Packaged Smoke Tests for Linux and macOS Desktop Targets

**Issue:** BAN-138  
**Date:** 2026-10-13  
**Status:** In Progress / Implemented  

---

## 1. Objective & Scope
Memperluas coverage packaged binary smoke test dari Windows ke Linux (`ubuntu-22.04`) dan macOS (`macos-latest`) di CI desktop matrix.

Tujuan:
1. Menyediakan script runner cross-platform POSIX (`scripts/smoke-wails-packaged.sh`) yang portable untuk Linux dan macOS.
2. Mengintegrasikan penanganan display headless Linux dengan `xvfb-run` (GTK/WebKit2GTK display requirement).
3. Mengonfigurasi job `desktop` di `.github/workflows/ci.yml` agar menjalankan packaged smoke pada semua runner matrix (`windows-latest`, `macos-latest`, `ubuntu-22.04`).
4. Memastikan timeout dan bounded exit via `CORTEXOS_SMOKE_QUIT_MS` tanpa hangs.

---

## 2. Platform Nuances Addressed
- **Linux (`ubuntu-22.04`)**:
  - Binary Wails membutuhkan X11 display context untuk inisialisasi GTK.
  - Ditambahkan instalasi `xvfb` pada step dependensi runner Linux.
  - Script mengeksekusi biner dengan `xvfb-run -a --server-args="-screen 0 1024x768x24"` jika `xvfb-run` tersedia.
  - Output binary: `build/bin/cortexos`.
- **macOS (`macos-latest`)**:
  - Binary berjalan langsung di bawah Cocoa environment runner tanpa server display eksternal.
  - Output binary: `build/bin/cortexos`.
- **Windows (`windows-latest`)**:
  - Tetap menggunakan PowerShell runner yang sudah ada (`scripts/smoke-wails-packaged.ps1`).
  - Output binary: `build/bin/cortexos.exe`.

---

## 3. Implementation Details
- `scripts/smoke-wails-packaged.sh`:
  - Parameter lingkungan: `QUIT_AFTER_MS`, `TIMEOUT_SECONDS`, `SKIP_BUILD`, `OUTPUT`.
  - Background process execution dengan polling loop dan bounded timeout (default 120s).
  - Trap cleanup otomatis membunuh proses jika timeout atau exit dini terjadi.
  - Memastikan exit code 0 dilaporkan hanya jika proses desktop keluar dengan bersih via `CORTEXOS_SMOKE_QUIT_MS`.
- `.github/workflows/ci.yml`:
  - Ditambahkan step `Packaged smoke (Linux/macOS)` dengan kondisi `runner.os != 'Windows'`.
  - Ditambahkan paket `xvfb` ke `apt-get install` di job Linux desktop matrix.
