# Phase 12 Execution Plan: AgentOS Cockpit & Brain Integration

**Generated:** 2026-10-13  
**Status:** Approved for Implementation  
**Estimated Complexity:** High  
**Objective:** Hubungkan Brain Runtime (Phase 11) ke application bridge, expose real-time token streaming dan lifecycle events ke frontend, dan implementasikan AgentOS-style Cockpit UI (Phaser avatars + real-time reasoning panel + context inspector).

---

## 1. Architecture Boundaries & Constraints

1. **Fail-Closed & Secret Safety**:
   - Provider API keys tidak pernah disentuh oleh layer UI atau terekspos via Wails bridge bindings.
   - Provider status `"disabled"` jika tidak terkonfigurasi.
   - Semua error provider dibersihkan dari payload mentah sebelum dikirim ke bridge.

2. **Concurrency & Thread Safety**:
   - `brain.Session` dan `Dispatcher` dieksekusi secara non-blocking di goroutine terpisah.
   - Token streaming di-buffer dan dikirim melalui `EventEmitter` Wails runtime tanpa memblokir thread eksekusi utama.

3. **Frontend Performance Budget**:
   - Virtual Office (Phaser 3) tetap mempertahankan 60 FPS under stress.
   - Streaming tokens di-batch pada requestAnimationFrame (raf) throttle untuk mencegah React re-render thrashing.

---

## 2. Sprint Roadmap

### Sprint 12.1: Brain Bridge & Application Port
**Goal**: Hubungkan `internal/cortex/brain` ke `application.Service` dan `platform.Bridge` dengan typed DTOs dan Wails event hooks.
**Demo/Validation**:
- Go test verifikasi bridge method: `DispatchBrainTask` dan event listener menangkap token streaming `cortexos:brain:stream:chunk`.

- **Task 12.1.1: Brain Port Definition in Application Layer**
  - **Location**: `internal/cortex/application/brain.go`
  - **Description**: Buat interface `BrainPort`, DTO `BrainDispatchRequest`, `BrainDispatchResult`, dan schema version `cortexos.brain.bridge.v1`.
  - **Dependencies**: `internal/cortex/brain`
  - **Validation**: Unit test serialization DTO di `internal/cortex/application/brain_test.go`.

- **Task 12.1.2: Brain Application Adapter**
  - **Location**: `internal/cortex/application/brain_adapter.go`
  - **Description**: Implementasi `BrainAdapter` yang membungkus `brain.ResilientDispatcher` dan `brain.ContextAssembler`.
  - **Dependencies**: Task 12.1.1
  - **Validation**: Contract test adapter di `internal/cortex/application/brain_adapter_test.go`.

- **Task 12.1.3: Wiring Service & Platform Bridge**
  - **Location**: `internal/cortex/application/service.go`, `internal/platform/bridge.go`, `internal/platform/events.go`
  - **Description**: Tambahkan field `brain BrainPort` di `Service`, expose `DispatchBrainTask(req)` di `Bridge`, dan definisikan event names `EventBrainStreamChunk`, `EventBrainStateChanged`.
  - **Dependencies**: Task 12.1.2
  - **Validation**: Bridge test di `internal/platform/bridge_test.go`.

- **Task 12.1.4: Bootstrap Wiring**
  - **Location**: `internal/bootstrap/bootstrap.go`
  - **Description**: Inisialisasi `brain.NewResilientDispatcher` dan assemble ke `application.NewServiceWithFull...`.
  - **Dependencies**: Task 12.1.3
  - **Validation**: Run `go test ./internal/bootstrap -count=1`.

---

### Sprint 12.2: Frontend Bridge Client & TypeScript Types
**Goal**: Buat TypeScript bridge wrapper di `web/src/bridge/brain.ts` dan hook `useBrainDispatch`.
**Demo/Validation**:
- Vitest unit test di `web/src/bridge/brain.test.ts` membuktikan event streaming tertangkap dan state transisi ter-update.

- **Task 12.2.1: TypeScript DTOs & Wails Event Subscriptions**
  - **Location**: `web/src/types/brain.ts`, `web/src/bridge/brain.ts`
  - **Description**: Buat interface TypeScript untuk `BrainDispatchRequest`, `BrainDispatchResult`, `BrainStreamChunk`, dan fungsi wrapper `dispatchBrainTask()`.
  - **Dependencies**: Sprint 12.1
  - **Validation**: `npm run test` di `web/`.

- **Task 12.2.2: React Hook `useBrainDispatch`**
  - **Location**: `web/src/hooks/useBrainDispatch.ts`
  - **Description**: Hook yang mengelola status eksekusi (`idle`, `planning`, `executing`, `streaming`, `completed`, `failed`), akumulasi token text stream, dan error handling.
  - **Dependencies**: Task 12.2.1
  - **Validation**: Hook test dengan mock Wails runtime events.

---

### Sprint 12.3: AgentOS-Style UI — Reasoning Panel & Stream Viewer
**Goal**: Bangun UI panel interaktif bergaya AgentOS untuk memantau pikiran agen (thinking indicator, token streaming, context preview).
**Demo/Validation**:
- Browser preview menampilkan prompt input box, klik "Run", melihat state `planning` -> `streaming` token real-time dengan autoscroll.

- **Task 12.3.1: Token Stream Markdown Viewer**
  - **Location**: `web/src/components/brain/StreamViewer.tsx`
  - **Description**: Komponen rendering streaming markdown ringan dengan syntax highlighting dan autoscroll lock.
  - **Dependencies**: Sprint 12.2
  - **Validation**: Render test dengan token chunk burst.

- **Task 12.3.2: Thinking & State Indicator**
  - **Location**: `web/src/components/brain/ReasoningTimeline.tsx`
  - **Description**: Visualisasi fase lifecycle (`Planning`, `Context Search`, `LLM Call`, `Tool Execution`) dengan badge durasi dan token usage.
  - **Dependencies**: Task 12.3.1
  - **Validation**: Component test untuk setiap state transisi.

- **Task 12.3.3: Context Inspector Drawer**
  - **Location**: `web/src/components/brain/ContextInspector.tsx`
  - **Description**: Drawer/accordion untuk menginspeksi snippets dokumen yang diambil dari Knowledge retrieval yang dimasukkan ke prompt.
  - **Dependencies**: Task 12.3.2
  - **Validation**: Verifikasi snippets terformat dan tidak membocorkan filepath absolut.

---

### Sprint 12.4: Virtual Office & Staff Avatar Integration
**Goal**: Hubungkan status Brain ke avatar Staff di Phaser canvas Virtual Office (animasi "thinking", speech bubble streaming ringkasan).
**Demo/Validation**:
- Avatar Staff bergerak ke meja kerja saat task dimulai, muncul indikator "thinking" / pulse, dan teks bubble muncul saat selesai.

- **Task 12.4.1: Phaser Staff State Visualizer**
  - **Location**: `web/src/office/scenes/OfficeScene.ts`
  - **Description**: Tambahkan state animasi pada Sprite avatar Staff: `idle`, `thinking`, `executing`, `done`.
  - **Dependencies**: Sprint 12.3
  - **Validation**: Playwright benchmark mempertahankan >= 58 FPS saat avatar aktif.

- **Task 12.4.2: Staff Selection to Brain Prompt**
  - **Location**: `web/src/components/cockpit/StaffActionModal.tsx`
  - **Description**: Klik avatar Staff di canvas -> buka modal instruksi -> kirim task ke `useBrainDispatch` dengan system role staff terpilih.
  - **Dependencies**: Task 12.4.1
  - **Validation**: End-to-end user flow test di web.

---

### Sprint 12.5: End-to-End Hardening & Packaging
**Goal**: Verifikasi performa, stabilitas soak, memory bounds, dan pastikan build Wails desktop berjalan mulus.
**Demo/Validation**:
- E2E Playwright test + Go unit test pass 100%. Package build sukses.

- **Task 12.5.1: E2E Playwright Smoke Test**
  - **Location**: `web/benchmarks/phase12-brain-cockpit.spec.ts`
  - **Description**: Test otomatis dispatch task -> streaming token -> finish -> verify state persistence.
  - **Validation**: `npx playwright test phase12`.

- **Task 12.5.2: Release Documentation & Version Bump**
  - **Location**: `docs/specs/phase-12-evidence.md`
  - **Description**: Dokumentasi evidence, snapshot metrics FPS, dan changelog.

---

## 3. Testing Strategy

1. **Go Unit & Contract Tests**:
   - Semua bridge & adapter di-test dengan `FakeProvider` dan `fakeKnowledge`.
   - Timeout dan cancellation wajib diuji di setiap layer.

2. **Frontend Vitest**:
   - Mock Wails runtime event stream.
   - Batch update rendering tidak membuat frame drop.

3. **Opt-in Live 9router Integration**:
   - Dijalankan secara manual (`CORTEXOS_9ROUTER_LIVE_TEST=1`) untuk memastikan prompt nyata dan model `OMP` bekerja mulus end-to-end.

---

## 4. Risks & Gotchas

1. **Wails Event Flooding**:
   - *Risk*: Token stream yang terlalu cepat (50+ token/detik) dapat membebani IPC bridge Wails.
   - *Mitigation*: Gunakan batching/micro-throttle (16ms) pada emission chunk jika diperlukan.

2. **Phaser Canvas Unmount / WebGL Context Loss**:
   - *Risk*: Switching view antara Cockpit dan Brain drawer bisa memicu WebGL context lost jika canvas tidak di-handle dengan benar.
   - *Mitigation*: Phaser canvas tetap mounted di background, drawer dirender sebagai overlay (fixed z-index).

---

## 5. Rollback Plan

Setiap sprint adalah commit atomic. Jika ada issue regresi bridge, rollback dengan aman ke commit `bf43689` (Phase 11 release).
