# Backlog: Langkah Selanjutnya CortexOS Orchestra & Files

**Tanggal:** 15 Oktober 2026
**Status:** Diterima & Aktif
**Fokus:** Menghubungkan execution loop, sinkronisasi approval/review, event file-save, dan verifikasi end-to-end.

---

## Ringkasan Progres Saat Ini
- [x] **Project Picker & Single Folder Registration:** Native folder picker, registrasi proyek tanpa input manual path, fallback auto-vault.
- [x] **Shell & Collapsible Sidebar:** Navigasi fokus 3 menu (Orchestra, Files, Settings), mode compact icon-first & mobile overlay.
- [x] **Chat/Command Orchestra:** Prompt sebagai obrolan langsung dengan Orchestra (agen teratas). Heuristic intent classifier:
  - `inquiry` ("halo", pertanyaan biasa) → dijawab langsung tanpa manggil worker (0 task/worker).
  - `action` (perintah kerja) → menyusun plan, menentukan role, dan auto-dispatch/minta konfirmasi.
- [x] **Graph Node Identity & Persistence:** Node graph menampilkan call sign worker (`implementer-1`), `assigned_staff_id` tersimpan persisten di SQLite (migration `orchestra-9`).
- [x] **Files Viewer (Read-only):** Segmentasi Repository, Support, dan Vault. Support kosong tidak error.

---

## Sprint Backlog

### Sprint 1: Sinkronisasi Approval & Review Loop (Phase C)
**Tujuan:** Memberikan pengalaman interaksi terpadu di mana keputusan approval tool dan review hasil kerja tersinkronisasi secara real-time di Prompt HUD dan Node Graph.

- [ ] **Task 1.1: Unified Approval State Contract**
  - **Lokasi:** `internal/cortex/application/tool_approval.go`, `internal/platform/bridge.go`
  - **Deskripsi:** Satukan state approval antrean (`pending`, `approved`, `denied`) agar emit event membawa task ID dan worker ID terkait.
  - **Validasi:** Unit test event bridge tool approval.
- [ ] **Task 1.2: Inline Approval Card di Node Graph & Prompt HUD**
  - **Lokasi:** `Big-ui/web/src/office/OrchestrationGraph.tsx`, `Big-ui/web/src/office/OrchestraPromptHud.tsx`
  - **Deskripsi:** Render kartu approval aksi tool (misal file write, git, search) langsung di node worker yang bersangkutan dan di prompt HUD secara identik.
  - **Validasi:** Memilih "Approve" di node otomatis mengupdate tampilan di prompt HUD, dan sebaliknya.
- [ ] **Task 1.3: Worker Acceptance & Auto-Unspawn Transition**
  - **Lokasi:** `internal/cortex/application/worker_lifecycle.go`, `Big-ui/web/src/office/WorkerCardsPanel.tsx`
  - **Deskripsi:** Ketika hasil kerja task di-merge/accept oleh Orchestra, worker bertransisi `reviewing` → `accepted` → `unspawned`. Node worker keluar dari canvas aktif.
  - **Validasi:** Test lifecycle card unspawn saat task status berubah menjadi `completed`.

---

### Sprint 2: Worktree File-Save Events & Files Auto-Refresh (Phase D)
**Tujuan:** Files surface otomatis ter-refresh setelah worker menyelesaikan penulisan file, tanpa polling atau refresh per-karakter.

- [ ] **Task 2.1: Worktree File-Write Completion Event**
  - **Lokasi:** `internal/cortex/application/files.go`, `internal/platform/events.go`
  - **Deskripsi:** Buat event `EventFilesChanged` / `EventWorktreeSaved` yang dipicu saat worker menyelesaikan operasi write/save.
  - **Validasi:** Test event bridge terpicu pasca eksekusi write.
- [ ] **Task 2.2: Files Surface Reactive Listener**
  - **Lokasi:** `Big-ui/web/src/files/FilesView.tsx`
  - **Deskripsi:** Hook listener ke event file-save untuk me-refresh tree file yang sedang aktif.
  - **Validasi:** Simulasikan save event, pastikan tree ter-update tanpa reload halaman.
- [ ] **Task 2.3: Active Worktree Selector Switcher**
  - **Lokasi:** `Big-ui/web/src/files/FilesView.tsx`, `internal/cortex/application/file_viewer.go`
  - **Deskripsi:** Dropdown/switcher di Files surface untuk melihat file di worktree tertentu (misal branch aktif worker) vs root repository.
  - **Validasi:** Pindah worktree mengubah daftar file secara akurat.

---

### Sprint 3: Real Execution Engine Wiring (Pi & Native Tools)
**Tujuan:** Worker yang di-spawn benar-benar mengeksekusi instruksi dari status `ready` hingga menghasilkan perubahan kode dan bukti.

- [ ] **Task 3.1: Task Execution Dispatcher**
  - **Lokasi:** `internal/cortex/orchestra/adapter.go`, `internal/cortex/harness/broker.go`
  - **Deskripsi:** Hubungkan worker lifecycle manager dengan Harness executor (Pi engine atau deterministic runner).
  - **Validasi:** Task yang di-auto-dispatch berpindah dari `ready` ke `running`.
- [ ] **Task 3.2: Evidence & Inspection Collection**
  - **Lokasi:** `internal/cortex/orchestra/inspection.go`
  - **Deskripsi:** Kumpulkan git diff dan log eksekusi sebagai evidence sebelum task masuk tahap `reviewing`.
  - **Validasi:** Task menampilkan diff perubahan di kartu review.

---

### Sprint 4: Knowledge & Vault Structured Writing
**Tujuan:** Orchestra menyimpan keputusan, spesifikasi, dan ringkasan arsitektur ke folder Markdown Vault terstruktur.

- [ ] **Task 4.1: Vault Category Layout & Provenance Writer**
  - **Lokasi:** `internal/cortex/application/knowledge.go`, `.cortexos/vault/`
  - **Deskripsi:** Penulisan otomatis ke folder terstruktur (`decisions/`, `specs/`, `conventions/`, `learnings/`) dengan header provenance (penulis, task ID, timestamp).
  - **Validasi:** File vault yang dibuat memiliki frontmatter metadata lengkap.

---

### Sprint 5: Integrated End-to-End QA (Phase E)
**Tujuan:** Memastikan keseluruhan flow berjalan mulus di lingkungan runtime nyata (`wails dev`).

- [ ] **Task 5.1: E2E Interactive Verification**
  - **Alur Pengujian:**
    1. Buka app → Project Picker → Pilih project
    2. Prompt: `"halo"` → Muncul balasan Orchestra, **0 worker**
    3. Prompt: `"buat fungsi login"` → Muncul plan proposal → Dispatched
    4. Graph: Muncul node worker dengan call sign (misal `implementer-1`)
    5. Files: File baru muncul setelah worker menulis
    6. Review/Accept: Worker selesai dan node di-unspawn (graf kembali idle).
  - **Validasi:** Tidak ada error console, UI responsif, state persisten saat app di-restart.
