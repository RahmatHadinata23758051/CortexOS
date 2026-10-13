# Diskusi Progres Lanjutan — Phase 9

**Tanggal:** 2026-10-13  
**Status:** Terbuka untuk keputusan eksekusi

## Posisi Saat Ini

Phase 8 sudah selesai dengan keputusan **GO**:
- Production Orchestra adapter sudah terhubung ke Wails bridge.
- Merge accepted dipersist atomik di SQLite.
- Compare-and-set mencegah merge ganda lintas proses.
- Packaged Wails Windows smoke test berhasil melalui Bridge → Service → Adapter → Inspector → SQLite.
- Milestone Phase 8 di Linear 100%.

## Gap yang Masih Perlu Ditutup

1. **Race evidence Linux** belum dijalankan dari mesin lokal Windows; CI Linux adalah authoritative gate.
2. **Packaged smoke lintas OS** baru terbukti lokal/CI untuk Windows; Linux dan macOS perlu smoke runtime.
3. **Release artifact contract** belum satu sumber untuk version, checksum, provenance, upgrade, dan rollback.
4. **Observability merge** belum memiliki event log terstruktur yang konsisten untuk accepted/rejected.
5. **UI-to-Bridge** masih memerlukan validasi generated Wails bindings tanpa mock.
6. **Real-engine vertical slice** belum diverifikasi dengan Pi/OMP adapter aktual.

## Rekomendasi Urutan

### Rekomendasi: Quality Gates → Release Contract → Product Slice

1. **BAN-137** — Buktikan `go test -race ./...` di Linux CI.
2. **BAN-138** — Tambahkan smoke packaged Linux/macOS.
3. **BAN-139** — Tetapkan versioning, artifact checksum, upgrade, dan rollback.
4. **BAN-140** — Tambahkan structured merge logs.
5. **BAN-141** — Validasi UI/Bridge generated bindings dan rejection rollback.
6. **BAN-143** — Jalankan vertical slice dengan real Pi/OMP engine.
7. **BAN-142** — Tutup dengan security/data-contract/release review dan GO/NO-GO.

Alasan: Jangan memperluas UX atau engine runtime sebelum bukti CI, packaging, persistence, dan observability siap; jika ada masalah nanti, diagnosanya lebih murah dan lebih terisolasi.

## Pertanyaan Keputusan

1. Apakah kita setuju mulai dari **BAN-137 (Quality Gates)** sebagai issue pertama Phase 9?
2. Apakah target Phase 9 tetap **2026-11-13**?
3. Untuk real engine (BAN-143), apakah environment CI menggunakan adapter asli yang tersedia atau memakai opt-in credential/local-run gate?
4. Apakah Linux/macOS packaged smoke wajib blocking release, atau advisory sampai Phase 9 review?

## Definition of Done Phase 9

- Semua issue BAN-137 s/d BAN-143 memiliki acceptance evidence.
- Linux race gate hijau.
- Packaged smoke lintas OS hijau atau residual risk diterima eksplisit.
- Build artifact deterministic dengan checksum/provenance.
- Structured logs dapat menjawab: siapa/task apa yang merge, execution mana, evidence berapa, accepted/rejected mengapa, dan berapa lama.
- UI memakai backend-authoritative result; tidak ada merge sukses dari optimistic state saja.
- Minimal satu vertical slice real engine berhasil atau diblok dengan alasan environment terdokumentasi.
- `docs/phase-9-review.md` berisi GO/NO-GO, rollback, dan residual risks.
