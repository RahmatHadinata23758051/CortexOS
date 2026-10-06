# CortexOS — AI CLI Engine Architecture Handoff

> Historical project handoff retained for traceability. The document describes CortexOS and is not executable source code.

## Context

Project ini adalah **CortexOS**, aplikasi desktop Local-First berbasis **Go + Wails** yang mensimulasikan sebuah Virtual Office dengan sekitar **10â€“20 AI Staff Agent**.

Staff bukan sekadar chat persona. Masing-masing dapat menerima task, membaca workspace, memodifikasi repository, menjalankan command/test, menggunakan skill, dan berkolaborasi melalui orchestration layer.

Target utama sistem adalah:

- Local-first desktop application.
- 10â€“20 logical AI Staff.
- Resource efficient untuk laptop consumer.
- Mendukung repository nyata melalui Git Worktree.
- Multi-provider dan local LLM.
- Reliable autonomous coding.
- Engine AI dapat diganti tanpa mengubah core CortexOS.
- Metric utama: **Time-to-Correct-Solution**, bukan raw generation speed.

---

# 1. Architecture

CortexOS dibagi menjadi empat layer utama.

```text
â”Œâ”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”
â”‚ Layer 1 â€” Workspace                          â”‚
â”‚ Git Worktree Isolation                       â”‚
â”‚ SQLite                                       â”‚
â”‚ Vector RAG                                   â”‚
â”‚ Markdown Vault / Obsidian-style Knowledge    â”‚
â””â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”˜
                      â”‚
                      â–¼
â”Œâ”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”
â”‚ Layer 2 â€” Orchestra                          â”‚
â”‚ Planner                                      â”‚
â”‚ Dispatcher                                   â”‚
â”‚ Inspector                                    â”‚
â”‚ DAG Task Engine                              â”‚
â”‚ Circuit Breaker                              â”‚
â””â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”˜
                      â”‚
                      â–¼
â”Œâ”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”
â”‚ Layer 3 â€” Harness                            â”‚
â”‚ Engine Router                                â”‚
â”‚ Model Router                                 â”‚
â”‚ Tool Broker                                  â”‚
â”‚ Sandbox                                      â”‚
â”‚ Skill Injection                              â”‚
â”‚ Resource Governor                            â”‚
â”‚ JSON Protocol                                â”‚
â””â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”˜
                      â”‚
                      â–¼
â”Œâ”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”
â”‚ Layer 4 â€” Staff                              â”‚
â”‚ Frontend                                     â”‚
â”‚ Backend                                      â”‚
â”‚ QA                                           â”‚
â”‚ DevOps                                       â”‚
â”‚ Security                                     â”‚
â”‚ Research                                     â”‚
â”‚ etc.                                         â”‚
â””â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”˜
```

---

# 2. Important Architectural Principle

**AI Staff MUST NOT be permanently tied to one CLI process or one AI engine.**

Staff adalah logical actor.

Contoh:

```text
Frontend Staff
â”œâ”€â”€ Persona
â”œâ”€â”€ Memory
â”œâ”€â”€ Skills
â”œâ”€â”€ Permissions
â”œâ”€â”€ Current Task
â””â”€â”€ Workspace
```

Execution engine dipilih secara dinamis.

```text
Staff
  â”‚
  â–¼
Engine Router
  â”œâ”€â”€ Native Go Tools
  â”œâ”€â”€ Pi
  â”œâ”€â”€ OMP
  â””â”€â”€ Future Engines
```

Artinya:

```text
20 Staff != 20 CLI Processes
```

Targetnya adalah 10â€“20 logical staff dengan jumlah active engine worker yang jauh lebih sedikit.

---

# 3. Resource Constraint

Memory efficiency adalah constraint utama.

Target:

```text
Total CLI / Agent Engine RAM < ~1 GB
```

Jangan membuat:

```text
20 Staff
â†“
20 Pi processes
```

atau:

```text
20 Staff
â†“
20 OMP processes
```

Gunakan pooled execution.

Target awal:

```text
Pi workers:
4 active workers

OMP workers:
1 active worker
maximum 2 jika memory budget memungkinkan
```

Concurrency harus adaptive berdasarkan telemetry.

Jangan hardcode jumlah worker sebagai satu-satunya limiter.

Resource Governor harus mempertimbangkan:

```text
current memory usage
memory peak
active child processes
task priority
engine type
available system RAM
CPU pressure
```

Idealnya gunakan historical `p95 memory usage` per engine/task class.

---

# 4. Current Engine Decision

## Tier 0 â€” Native Go Tools

Untuk task deterministic, JANGAN gunakan AI CLI jika tidak diperlukan.

Contoh:

```text
read file
write file
glob
grep/search
git status
git diff
git log
file move
directory operations
format
lint
run test
RAG query
Vault query
dependency query
structured transform
```

Flow:

```text
Orchestra
   â”‚
   â–¼
Go Tool Broker
   â”‚
   â–¼
OS / Workspace
```

Ini adalah execution tier paling murah.

---

# 5. Primary AI Engine â€” Pi

Default coding engine CortexOS adalah:

```text
Pi
```

Pi dipilih bukan karena harus dianggap "CLI terbaik", tetapi karena karakteristiknya sesuai dengan CortexOS:

- core relatif minimal;
- extensible;
- multi-provider;
- custom/OpenAI-compatible endpoints;
- local model compatible;
- RPC/JSONL mode;
- long-running worker process;
- tidak memaksakan orchestration architecture sendiri;
- benchmark yang tersedia menunjukkan quality/token efficiency yang kompetitif;
- lebih cocok menjadi worker di bawah Orchestra.

Gunakan Pi sebagai execution engine untuk task seperti:

```text
normal bugfix
frontend work
backend work
CRUD/API
test generation
normal refactor
multi-file feature
documentation/code changes
code investigation
```

Integration pattern:

```text
CortexOS Go Harness
        â”‚
        â”‚ stdin/stdout JSONL
        â–¼
    Pi Worker
```

Prefer long-lived worker pool daripada spawn process untuk setiap prompt.

---

# 6. Pi Configuration Philosophy

CortexOS adalah authority utama.

Jangan membiarkan Pi membangun orchestration layer kedua.

Disable / hindari:

```text
Pi-owned subagent orchestration
Pi-owned persistent project memory
Pi-owned model routing policy
Pi-owned security policy
```

CortexOS sudah memiliki:

```text
Planner
Dispatcher
Inspector
DAG
RAG
Vault
Staff memory
Model Router
Sandbox
Permissions
Retry logic
```

Pi sebaiknya digunakan sebagai:

```text
Reasoning + Coding Executor
```

bukan sebagai operating system di dalam operating system.

---

# 7. Specialist Engine â€” oh-my-pi / OMP

OMP bukan default worker.

OMP adalah **Heavy/Specialist Engine**.

Gunakan OMP ketika task membutuhkan capability yang tidak efektif dilakukan oleh default Pi worker.

Contoh escalation:

```text
hard runtime debugging
DAP debugger required
semantic LSP refactor
large symbol rename
complex dependency-aware refactoring
persistent Python/JS analysis
primary engine repeatedly fails
high-risk multi-file modification
complex investigation
```

Target concurrency:

```text
OMP = 1 worker normally
OMP = max 2 workers if resources allow
```

Integration dilakukan melalui RPC/JSONL.

```text
Go Harness
    â”‚
    â”‚ JSONL RPC
    â–¼
OMP worker
```

Tidak perlu N-API integration ke Go.

---

# 8. OMP Features That Actually Matter

## DAP

OMP dapat dipakai ketika runtime debugging benar-benar diperlukan.

Contoh:

```text
deadlock
race-like runtime behavior
segfault
state-dependent failure
Python process freeze
complex runtime state
```

Jangan gunakan DAP hanya karena tersedia.

---

## Semantic LSP

Gunakan OMP untuk operasi seperti:

```text
workspace rename
public API rename
cross-package refactor
large symbol migration
code actions
reference-aware modification
```

Normal code editing tetap menggunakan Pi.

---

## Persistent Kernel

Aktifkan hanya untuk task yang memerlukannya.

Contoh:

```text
data investigation
numerical analysis
AST experimentation
protocol analysis
log exploration
debugging stateful scripts
```

Default:

```text
Python Kernel = OFF
JS Kernel     = OFF
```

Enable on-demand.

---

# 9. OMP Features That Should Be Disabled

CortexOS already owns these capabilities.

Default OMP Agency profile:

```text
Subagents = OFF
Persistent Memory = OFF
Browser = OFF unless required
Kernel = OFF unless required
DAP = lazy
LSP = lazy
```

Reason:

Nested orchestration seperti:

```text
Agency Staff
   â†“
OMP
   â†“
OMP Subagent
   â†“
more OMP agents
```

tidak diinginkan.

Orchestra CortexOS harus menjadi **single source of orchestration truth**.

---

# 10. Hashline / Safe Editing

Hashline-style stale-edit protection adalah capability bagus, tetapi jangan dibuat khusus OMP.

Implementasikan konsep ini di **Agency Tool Broker** sehingga semua engine mendapat proteksi yang sama.

Contoh protocol:

```json
{
  "operation": "read_file",
  "path": "src/user.go"
}
```

response:

```json
{
  "path": "src/user.go",
  "content": "...",
  "revision": "sha256:abcd..."
}
```

Edit request:

```json
{
  "operation": "edit_file",
  "path": "src/user.go",
  "expected_revision": "sha256:abcd...",
  "patch": "..."
}
```

Harness harus mengecek:

```text
current_revision == expected_revision
```

Jika tidak:

```text
STALE_REVISION
```

Agent wajib membaca ulang file.

Semua write sebaiknya:

```text
validate
â†’ temporary write
â†’ syntax/check if applicable
â†’ atomic replace
â†’ new revision
```

Dengan begitu Pi, OMP, dan engine lain mendapat stale-edit protection yang sama.

---

# 11. Model Routing

Model routing adalah tanggung jawab CortexOS.

Jangan biarkan setiap CLI mempunyai routing policy sendiri.

Architecture:

```text
Task
 â”‚
 â–¼
Agency Model Router
 â”‚
 â”œâ”€â”€ Anthropic
 â”œâ”€â”€ OpenAI
 â”œâ”€â”€ Gemini
 â”œâ”€â”€ OpenRouter
 â”œâ”€â”€ 9router
 â””â”€â”€ Ollama
 â”‚
 â–¼
Engine Adapter
```

Engine menerima keputusan model dari Harness.

Contoh internal descriptor:

```json
{
  "provider": "openrouter",
  "model": "...",
  "reasoning_effort": "medium",
  "fallback_policy": "coding-standard"
}
```

Fallback provider/model tetap diatur centrally.

---

# 12. Required Provider Support

CortexOS harus mampu bekerja dengan:

```text
Anthropic
OpenAI
Gemini
OpenRouter
9router
Ollama
OpenAI-compatible endpoints
```

Jangan membuat provider logic terlalu bergantung pada satu CLI.

Buat abstraction sendiri:

```go
type ModelTarget struct {
    Provider string
    Model string
    BaseURL string
    ReasoningEffort string
    CredentialRef string
}
```

Engine adapter bertugas menerjemahkan `ModelTarget` menjadi format engine masing-masing.

---

# 13. Engine Adapter Interface

Engine harus replaceable.

Contoh conceptual Go interface:

```go
type Engine interface {
    Start(ctx context.Context) error
    Stop(ctx context.Context) error

    Execute(
        ctx context.Context,
        req TaskExecutionRequest,
    ) (<-chan EngineEvent, error)

    Cancel(taskID string) error

    Health() EngineHealth
    Capabilities() EngineCapabilities
}
```

Implementasi awal:

```text
NativeEngine
PiEngine
OMPEngine
```

Future:

```text
CodexEngine
OpenCodeEngine
ClaudeEngine
GooseEngine
CrushEngine
```

Core Orchestra tidak boleh mengetahui protocol internal Pi atau OMP.

---

# 14. Capability-Based Routing

Jangan route hanya berdasarkan nama Staff.

Gunakan capability requirement.

Contoh:

```go
type TaskRequirements struct {
    Complexity       int
    RuntimeDebug     bool
    SemanticRefactor bool
    StatefulAnalysis bool
    Deterministic    bool
    FileCount        int
    DependencyDepth  int
    PreviousFailures int
}
```

Routing awal:

```text
IF deterministic
    â†’ Native Go

ELSE IF runtime_debug
    â†’ OMP

ELSE IF semantic_refactor && high_scope
    â†’ OMP

ELSE IF stateful_analysis
    â†’ OMP

ELSE IF previous_failures >= 2
    â†’ OMP escalation

ELSE
    â†’ Pi
```

---

# 15. Circuit Breaker / Retry Policy

Current system mempunyai:

```text
max retry = 3
```

Jangan menjalankan tiga retry identik.

Recommended:

```text
Attempt 1
Pi + normal model
        â”‚
        â–¼ FAIL

Attempt 2
Pi + Inspector feedback
possibly stronger model
        â”‚
        â–¼ FAIL

Attempt 3
OMP specialist
+ previous attempts
+ Inspector diagnosis
+ test output
+ current diff
        â”‚
        â–¼ FAIL

REPLAN or HUMAN REVIEW
```

Jadi retry juga merupakan **strategy escalation**.

---

# 16. Inspector Must Remain Independent

Agent tidak boleh menentukan sendiri bahwa task selesai.

Flow:

```text
Engine
   â”‚
   â–¼
Candidate Change
   â”‚
   â–¼
Inspector
   â”‚
   â”œâ”€â”€ tests
   â”œâ”€â”€ lint
   â”œâ”€â”€ acceptance criteria
   â”œâ”€â”€ diff inspection
   â”œâ”€â”€ security checks
   â””â”€â”€ task-specific validator
```

Status `SUCCESS` hanya diberikan oleh Orchestra/Inspector.

Metric utama adalah:

```text
Time-to-Correct-Solution
```

bukan:

```text
Time-to-Agent-Stops
```

---

# 17. Worker Pool Model

Recommended initial limits:

```text
Native Go:
unlimited within safe OS limits

Pi:
min = 2
default = 4
max = adaptive 4â€“6

OMP:
default = 1
max = 2
```

20 Staff dapat tetap aktif secara logical.

Contoh:

```text
20 Staff

4 RUNNING
5 READY
3 WAITING_DEPENDENCY
4 BLOCKED
2 REVIEWING
2 IDLE
```

Hanya RUNNING task yang membutuhkan engine worker.

---

# 18. Resource Governor

Resource Governor harus menjadi komponen first-class.

Track per worker:

```text
RSS
PSS if available
memory peak
CPU
process count
child process memory
execution duration
token usage
tool call count
model cost
retry count
```

Resource accounting harus mencakup process tree.

Bukan hanya PID utama.

Contoh:

```text
Pi
 â”œâ”€â”€ language server
 â”œâ”€â”€ test runner
 â”œâ”€â”€ compiler
 â””â”€â”€ shell
```

Semua harus dihitung ke budget task/worker tersebut.

---

# 19. Worker Recycling

CLI worker jangan diasumsikan aman hidup selamanya.

Implementasikan recycling berdasarkan:

```text
max task count
memory threshold
memory growth
idle timeout
engine error
protocol desync
health check failure
```

Pattern:

```text
ACTIVE
   â†“
DRAINING
   â†“
finish current task
   â†“
terminate
   â†“
fresh worker
```

Jangan kill worker di tengah mutation workspace kecuali emergency.

---

# 20. Sandbox Ownership

Security boundary utama harus berada di CortexOS.

Engine tidak boleh menjadi authority keamanan.

```text
AI Engine
   â”‚
   â–¼
Agency Tool Broker
   â”‚
   â–¼
Policy Engine
   â”‚
   â”œâ”€â”€ command whitelist
   â”œâ”€â”€ path jail
   â”œâ”€â”€ worktree restriction
   â”œâ”€â”€ environment filtering
   â”œâ”€â”€ network policy
   â”œâ”€â”€ execution timeout
   â”œâ”€â”€ process limit
   â””â”€â”€ memory limit
   â”‚
   â–¼
OS Sandbox
```

Agent tidak boleh bebas:

```text
read ~/.ssh
read sibling worktree
write outside workspace
spawn unlimited process
access arbitrary secrets
```

---

# 21. Skills

Skills adalah Agency-owned resource.

```text
Global Skills
Role Skills
Project Skills
Task Skills
```

Harness menentukan skill apa yang diberikan ke Staff.

Jangan bergantung pada global skill directory milik satu CLI.

Engine adapter hanya mengubah skill Agency menjadi representation yang dimengerti engine.

---

# 22. Memory / Context Ownership

CortexOS memiliki persistent memory.

```text
Markdown Vault
SQLite
Vector RAG
Task History
Staff Knowledge
Project Knowledge
```

Pi/OMP session context hanya dianggap:

```text
ephemeral working context
```

Persistent truth harus disimpan di CortexOS.

Dengan demikian worker dapat direstart kapan saja tanpa kehilangan organizational memory.

---

# 23. Context Construction

Sebelum task diberikan ke engine, Harness membangun context package:

```text
System / Staff Role
Task
Acceptance Criteria
Relevant Skills
Relevant Vault Memory
Relevant RAG
Workspace Metadata
Relevant Files
Previous Attempt Summary
Inspector Feedback
Tool Permissions
```

Jangan inject seluruh repository secara default.

Gunakan retrieval dan on-demand file reads.

---

# 24. Current CLI Assessment

Current design decision:

```text
Tier 0
Native Go Tools

Tier 1
Pi
PRIMARY / DAILY ENGINE

Tier 2
OMP
HEAVY / SPECIALIST ENGINE
```

Engine lain tidak dibuang.

Mereka dipertahankan sebagai future adapters / benchmark candidates:

```text
Codex CLI
OpenCode
Claude Code
Goose
Aider
Crush
```

Tetapi jangan membuat core architecture bergantung pada mereka.

---

# 25. Why Not OpenCode as Primary Right Now

Jangan menggunakan asumsi:

```text
OpenCode = Go native 30â€“50 MB
```

Itu mengacu pada generasi OpenCode lama.

Current OpenCode adalah TypeScript/Bun.

OpenCode Go lama berevolusi menjadi project lain, yaitu Crush.

Current OpenCode tetap menarik dan dapat diuji sebagai optional engine, tetapi jangan memilihnya hanya berdasarkan angka RAM Go lama.

---

# 26. Why Pi Is Primary

Pi dipilih karena CortexOS membutuhkan engine yang:

```text
lightweight relative to feature-heavy engines
multi-provider
RPC controllable
extensible
minimal orchestration opinion
good coding quality
compatible with local/custom models
easy to pool
```

CortexOS sudah memiliki feature seperti:

```text
orchestration
memory
skills
sandbox
routing
retry
workspaces
```

jadi engine minimal lebih cocok daripada engine yang mengimplementasikan ulang semuanya.

---

# 27. Why OMP Is Specialist

OMP dipertahankan karena memiliki capability yang mahal tetapi bernilai tinggi:

```text
DAP debugger
semantic LSP operations
advanced refactoring
stateful analysis/kernel
hard-task recovery
```

Agency tidak perlu membayar memory/resource overhead capability tersebut pada setiap worker.

Load capability hanya saat dibutuhkan.

---

# 28. Architectural Goal

Target akhir bukan:

> menemukan satu CLI yang melakukan semuanya.

Target akhir adalah:

```text
CortexOS owns:
- orchestration
- memory
- security
- tools
- model routing
- skills
- resource policy
- validation

CLI engines own:
- reasoning
- coding
- specialized execution
```

Engine harus dianggap seperti driver.

```text
Pi today
OMP specialist today

future:
Codex / OpenCode / another engine
```

dapat ditambahkan atau diganti tanpa merombak sistem.

---

# 29. Immediate Implementation Priority

Kerjakan dalam urutan berikut:

1. Buat generic `Engine` interface.
2. Buat `NativeEngine`.
3. Buat `PiEngine` berbasis RPC/JSONL.
4. Buat worker pool untuk Pi.
5. Tambahkan Resource Governor.
6. Tambahkan centralized Tool Broker.
7. Tambahkan revision/hash based file editing.
8. Hubungkan Inspector ke result validation.
9. Buat capability-based Engine Router.
10. Implementasikan `OMPEngine`.
11. Tambahkan escalation Pi â†’ OMP.
12. Tambahkan worker recycling.
13. Baru benchmark concurrency dan tune jumlah worker.

Jangan implementasikan banyak CLI adapter sekaligus sebelum abstraction di atas stabil.

---

# 30. Non-Negotiable Rules

- Jangan membuat satu process CLI per Staff.
- Jangan tie Staff persona ke engine tertentu.
- Jangan menyimpan persistent organizational memory hanya di CLI session.
- Jangan memberikan engine unrestricted filesystem/shell access.
- Jangan membiarkan subagent engine mengambil alih orchestration.
- Jangan menduplikasi Model Router pada masing-masing CLI.
- Jangan mengukur performance hanya dari response speed.
- Ukur **Time-to-Correct-Solution**.
- Semua perubahan harus diverifikasi Inspector.
- Semua engine harus replaceable melalui adapter.
- Resource telemetry harus menjadi bagian dari architecture sejak awal.

Final target architecture:

```text
Logical Staff
     â”‚
     â–¼
Orchestra
     â”‚
     â–¼
DAG Scheduler
     â”‚
     â–¼
Engine Router
     â”‚
 â”Œâ”€â”€â”€â”¼â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”
 â”‚   â”‚              â”‚
 â–¼   â–¼              â–¼
Go   Pi             OMP
â”‚    Primary        Specialist
â”‚
â””â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”¬â”€â”€â”€â”€â”€â”€â”€â”€â”˜
           â–¼
     Agency Tool Broker
           â”‚
           â–¼
      OS Sandbox
           â”‚
           â–¼
      Git Worktree
```

The core principle is:

**CortexOS is the operating system. Pi and OMP are replaceable execution engines.**



