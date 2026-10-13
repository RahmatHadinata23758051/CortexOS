# Phase 9: Productization and Operational Readiness Execution Plan

**Generated:** 2026-10-13  
**Status:** Planned  
**Milestone:** `Phase 9 — Productization and Operational Readiness` (`139b779c-6c38-4ac5-9f94-a7234cafa2fc`)

---

## 1. Overview & Context

Phase 8 sukses menyelesaikan adapter produksi Orchestra, penguncian merge atomik via transaksi SQLite CAS, dan smoke test binary terpaket Windows.

Fokus Phase 9 adalah **Productization & Operational Readiness**:
Menutup celah sisa sebelum rilis (CI race coverage di Linux, smoke test lintas OS, versioning & upgrade contract, structured observability, validasi UI/Bridge tanpa mock, serta vertical slice dengan engine coding riil).

---

## 2. Linear Backlog Breakdown

| Issue | Judul | Prioritas | Label | Deskripsi Singkat |
|---|---|---|---|---|
| **BAN-137** (P9-01) | Linux CI race detector evidence and coverage gate | High | `Architecture` | Menjalankan `go test -race ./...` di Linux CI dan mengunci zero-race gate. |
| **BAN-138** (P9-02) | Packaged smoke tests for Linux and macOS desktop targets | High | `Infrastructure` | Menjalankan smoke binary terpaket di Linux (xvfb/headless) dan macOS di CI matrix. |
| **BAN-139** (P9-03) | Desktop packaging, versioning, upgrade and rollback contract | High | `Infrastructure`, `Architecture` | Menetapkan versioning tunggal, metadata Wails, checksum build, dan kontrak rollback SQLite. |
| **BAN-140** (P9-04) | Structured merge audit log and observability hooks | High | `Architecture` | Structured logging kanonikal untuk keputusan merge (accepted/rejected) tanpa bocor rahasia. |
| **BAN-141** (P9-05) | Real frontend-to-bridge end-to-end integration and bindings verification | High | `Feature` | Menghubungkan generated TypeScript bindings Wails ke React Cockpit & memvalidasi rollback UI. |
| **BAN-142** (P9-06) | Phase 9 security, data-contract and release readiness review | High | `Architecture`, `Documentation` | Audit formal ADR-0001 s/d ADR-0005, migrasi DB, dan dokumen rilis GO/NO-GO. |
| **BAN-143** (P9-07) | End-to-end orchestration vertical slice with real engines (Pi/OMP) | Medium | `Feature` | Uji alur nyata Orchestra → Staff → Harness → Pi engine adapter → evidence → merge. |

---

## 3. Urutan Eksekusi Bertahap (Sprint Execution DAG)

```text
Sprint 1 (Fondasi & CI Gates):
  BAN-137 (P9-01: Linux CI race gate)
      │
      ▼
  BAN-138 (P9-02: Smoke packaged Linux & macOS)
      │
      ▼
Sprint 2 (Release & Observability):
  BAN-139 (P9-03: Packaging, versioning & rollback)
      │
      ▼
  BAN-140 (P9-04: Structured merge logging)
      │
      ▼
Sprint 3 (Integrasi Desktop & Engine):
  BAN-141 (P9-05: Real frontend-bridge verification)
      │
      ▼
  BAN-143 (P9-07: Vertical slice with real engines)
      │
      ▼
Sprint 4 (Review Rilis):
  BAN-142 (P9-06: Phase 9 formal review & GO/NO-GO)
```

---

## 4. Opsi Arah Diskusi dengan User

Untuk memulai eksekusi Phase 9, ada 2 pendekatan strategis:

1. **Jalur A (Quality Gates & CI Pertama - Direkomendasikan)**:
   - Mulai dari **BAN-137** (verifikasi race detector di CI) & **BAN-138** (smoke test lintas OS).
   - Menjamin bahwa fondasi biner di semua platform (Windows, Linux, macOS) 100% stabil sebelum ekspansi fitur.

2. **Jalur B (Frontend / User Experience Pertama)**:
   - Mulai dari **BAN-141** (integrasi Wails generated bindings ke Cockpit UI).
   - Memberikan visualisasi langsung di UI desktop bagi operator sebelum finalisasi packaging CI.

3. **Jalur C (Real AI Engine Integration Pertama)**:
   - Mulai dari **BAN-143** (menghubungkan engine Pi/OMP nyata ke Orchestra/Harness).
   - Menguji eksekusi AI coding nyata di worktree sebelum menyelesaikan observability dan packaging.
