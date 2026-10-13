# Phase 9 P9-01: Linux CI Race Detector Evidence and Coverage Gate

**Issue:** BAN-137  
**Date:** 2026-10-13  
**Status:** ✅ **VERIFIED & PASSED**

---

## 1. Context & Objective
Sesuai ADR dan quality gates rilis, CortexOS mewajibkan seluruh rangkaian unit/integration test berjalan bersih di bawah detektor data race Go (`go test -race ./...`).

Karena platform development Windows tidak memiliki toolchain CGO untuk kompilasi race detector (`runtime/cgo`), pipeline CI GitHub Actions pada `ubuntu-latest` (dan `macos-latest`) difungsikan sebagai **Authoritative Race Gate**.

Tujuan BAN-137:
1. Memverifikasi konfigurasi workflow CI `.github/workflows/ci.yml` untuk job race detector Linux.
2. Memverifikasi hasil eksekusi race detector di CI run.
3. Mendokumentasikan bukti log dan zero data race findings.

---

## 2. CI Workflow Configuration Gate
File workflow `.github/workflows/ci.yml` mengonfigurasi gate sebagai berikut:

```yaml
jobs:
  go:
    name: Go (Linux, race detector)
    runs-on: ubuntu-latest
    timeout-minutes: 15
    steps:
      - uses: actions/checkout@v4
      - name: Setup Go
        uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
      - name: Install Wails Linux dependencies
        run: |
          sudo apt-get update
          sudo apt-get install -y libgtk-3-dev libwebkit2gtk-4.1-dev
      - name: gofmt
        run: |
          unformatted="$(gofmt -l .)"
          if [ -n "$unformatted" ]; then
            echo "Unformatted files:"
            echo "$unformatted"
            exit 1
          fi
      - name: go vet
        run: go vet ./...
      - name: go test -race
        run: go test -race ./...
```

Perilaku: Setiap push ke `main` atau pull request otomatis mengeksekusi `go test -race ./...`. Kegagalan race detector menghasilkan exit code non-zero dan membatalkan build.

---

## 3. Evidence of Verification (CI Run 38241393237)

Hasil audit run CI terbaru (`https://github.com/RahmatHadinata23758051/CortexOS/actions/runs/38241393237`):

| Step | Hasil | Status |
|---|---|---|
| **Set up job** | `success` | Selesai |
| **Run actions/checkout@v4** | `success` | Selesai |
| **Setup Go** | `success` | Selesai |
| **Install Wails Linux dependencies** | `success` | Selesai |
| **gofmt** | `success` | Bersih, zero unformatted files |
| **go vet** | `success` | Bersih, zero warnings |
| **go test -race** | `success` | **PASSED (2m 21s) — ZERO data race warnings** |
| **Job Conclusion** | `success` | **GREEN** |

---

## 4. Concurrency Protection Coverage Verified
Rangkaian test yang tercakup dalam race detector mencakup modul konkurensi kritis:
1. `internal/cortex/orchestra/adapter_atomic_test.go`:
   - `TestProductionAdapter_MergeTask_ConcurrentOnlyOneSucceeds`: 4 goroutine konkuren memanggil `MergeTask` pada task yang sama; race detector memvalidasi tidak ada kontensi memori tidak aman pada `activeMerge`, map, dan store.
2. `internal/cortex/orchestra/sqlite`:
   - Transaksi serializable CAS atomik compare-and-set untuk `awaitingInspection -> success` dan unblocking dependent tasks `draft -> ready`.
3. `internal/platform`:
   - `Application.OnStartup` / `Application.OnShutdown` mutex synchronization hooks.

---

## 5. Acceptance Criteria Checklist
- [x] CI Linux job green dengan race detector diaktifkan.
- [x] Zero data race warnings pada seluruh package `./...`.
- [x] Bukti hasil run tercatat dan terdokumentasi secara permanen.

**Status: GO / PASSED**
