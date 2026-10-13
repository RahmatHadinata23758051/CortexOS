# BAN-136: Packaged Wails Production Smoke Test & End-to-End Pipeline Verification

## Objective
Menyelesaikan **P8-03** untuk Phase 8: Memverifikasi bahwa seluruh pipeline produksi Orchestra (`Bridge` -> `Service` -> `Adapter` -> `Inspector` -> `SQLite`) berfungsi penuh end-to-end pada binary Wails produksi yang telah dipaket (`windows/amd64`).

## Background & Architecture Decisions
1. **Isolated Data Root**:
   - `bootstrap.defaultWorkspaceDataRoot()` mendukung override via environment variable `CORTEXOS_DATA_ROOT`.
   - Ini memungkinkan proses binary Wails terisolasi di direktori temporary tanpa menyentuh konfigurasi user di `AppData/Roaming/CortexOS`.
2. **Packaged Smoke Probe Lifecycle**:
   - Binary desktop Wails memiliki hook startup `CORTEXOS_SMOKE_MERGE_TASK` dan `CORTEXOS_SMOKE_QUIT_MS`.
   - Ketika kedua variabel lingkungan ini aktif, aplikasi menjalankan pemanggilan `bridge.MergeOrchestraTask` di runtime Wails aktual.
   - Hasil merger (termasuk status, schemaVersion, dan detail error jika ada) ditulis ke `CORTEXOS_SMOKE_RESULT_FILE`.
   - Aplikasi menutup dirinya secara bersih setelah batas waktu `CORTEXOS_SMOKE_QUIT_MS` (exit code 0).
3. **Cross-Process Worktree Discovery**:
   - `workspacegit.Adapter` diperluas dengan lazy discovery (`discoverWorktreeByID`).
   - Ketika proses binary baru dimulai (tanpa cache in-memory worktree dari proses setup), adapter dapat menemukan worktree fisik yang ada di disk berdasarkan path yang diturunkan (`idForPath`) atau ID folder eksplisit, lalu memeriksa status kebersihannya secara authoritative.

## Verification Evidence
1. **E2E Smoke Pipeline**:
   - Menyiapkan repository Git lokal temporer dengan clean worktree.
   - Melakukan registrasi project dan worktree di `state/workspace.db`.
   - Mempersiapkan execution dan bukti `harness.EvidenceRecord` di `state/harness.db`.
   - Mempersiapkan task graph di `state/orchestra.db`:
     - Parent task: `smoke-parent` (`awaitingInspection`)
     - Dependent task: `smoke-child` (`draft`, depends on `smoke-parent`)
   - Membangun binary produksi menggunakan Wails CLI: `wails build -clean -nopackage -o cortexos-smoke.exe`.
   - Menjalankan binary dengan `CORTEXOS_DATA_ROOT`, `CORTEXOS_SMOKE_QUIT_MS=5000`, `CORTEXOS_SMOKE_MERGE_TASK=smoke-parent`.
   - Verifikasi exit code = 0 (bersih).
   - Verifikasi file output `smoke-result.json`: `status: "success"`.
   - Verifikasi independen pada database SQLite:
     - Parent task beralih ke `TaskSuccess`.
     - Dependent child task beralih ke `TaskReady` (unblocked otomatis).
     - Event `inspection.accepted` dan `task.validated` tersimpan di database SQLite.
2. **Execution Result**:
   - `go test -tags=p803_smoke ./internal/bootstrap -run TestPackagedWailsSmoke_MergePipeline -count=1 -timeout 300s` -> **PASS (86.36s)**.
   - `go test ./internal/cortex/... ./internal/bootstrap` -> **PASS**.
   - `go vet ./...` -> **PASS**.
   - Working tree bersih.
