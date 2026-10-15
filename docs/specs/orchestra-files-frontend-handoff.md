# CortexOS Frontend Handoff — Orchestra Workspace & Files

**Audience:** frontend implementation agent (Claude)
**Status:** product direction agreed; implementation roadmap
**Scope:** app shell, project entry flow, Orchestra workspace, floating prompt HUD, sidebar, Files surface

> Read this document before changing the frontend. This is the agreed product direction from the user discussion. Do not revert to the old dashboard-first information architecture.

---

## 1. Product Direction

CortexOS is a local-first desktop application for developing real projects with an Orchestra agent.

The user does **not** open into a dashboard full of independent architecture tabs. The user first chooses an existing Project/Office or creates a new one, then enters that project's workspace.

The project workspace has three important visual regions:

```text
┌──────────────┬──────────────────────┬──────────────────────────────┐
│ Collapsible  │ Orchestra prompt     │ Main project surface          │
│ sidebar      │ / conversation HUD   │ default: Orchestra graph       │
│              │ floating + movable   │ node-based worker cards        │
└──────────────┴──────────────────────┴──────────────────────────────┘
```

The Orchestra graph is the main visual experience. The virtual-office/Phaser scene is **not** the primary interface for this phase; the node-based graph is its practical alternative.

---

## 2. Core Mental Model

```text
User prompt
  → Orchestra understands intent
  → Orchestra creates a plan / task graph
  → user confirms only when the plan or risk requires it
  → Orchestra assigns ephemeral workers
  → worker cards appear on the graph
  → worker produces evidence/change
  → Orchestra reviews and decides outcome
  → accepted worker/task is removed from active graph
    (history/evidence remains accessible)
```

Responsibilities:

- **Orchestra:** receives user prompts, plans work, chooses worker role/name, assigns tasks, controls approval, reviews evidence, and owns the final outcome.
- **Worker:** ephemeral execution actor. It is represented by a card while active. It is not a permanent Staff directory entry.
- **Pi:** primary daily execution engine for normal coding, bug fixes, tests, and refactors.
- **OMP:** specialist escalation engine for hard debugging, semantic/LSP refactors, or repeated Pi failure. It is not the default visible product concept.
- **Knowledge:** project memory in multiple Markdown files. Orchestra writes validated decisions/reviews/learnings; the retrieval index is disposable and read-only.

There is intentionally **no standalone Staff tab** in the new product shell.

---

## 3. Required App Flow

### Level 0 — Project/Office Picker

On first launch, show a focused project picker instead of the old Workspace dashboard:

- list existing local projects/offices;
- show project name, repository root/status, and last activity where available;
- action to create/register a new project;
- selecting a project enters the project workspace;
- preserve a way to switch project later from the shell/sidebar.

If the bridge is unavailable, show an explicit connection state; do not fabricate projects or metrics.

### Level 1 — Project Workspace

Default route/view is **Orchestra**:

- collapsible sidebar;
- floating Orchestra prompt/conversation HUD;
- full main canvas containing the node-based orchestration graph;
- project/worktree context always visible;
- approval and review synchronized between HUD and graph.

The old route set (`workspace`, `operations`, `terminal`, `activity`, `office`, `knowledge`, `staff`, `brain`, `settings`) must not remain as nine equal-priority top-level destinations. Fold or remove them according to the target navigation below.

---

## 4. Sidebar Direction

Use the provided Aceternity-style interaction as the visual reference, adapted to React + Vite + Wails:

- collapsed desktop rail: icon-first, approximately 60px;
- hover/focus expands smoothly to a wider rail, approximately 260–300px;
- labels fade/slide in only when expanded;
- mobile becomes a full-screen overlay with menu/close controls;
- keyboard users must be able to expand/navigate without relying on hover;
- use `lucide-react`, not emoji or hand-drawn icon glyphs;
- keep active state obvious with semantic text + accent/border, not color alone;
- preserve the existing CortexOS dark/light token system.

The sidebar is navigation only. It must not become another dense dashboard.

### Target top-level destinations

1. **Orchestra** — default/main work surface.
2. **Files** — repository, support files, and Knowledge Vault.
3. **Settings** — system/provider/preferences, preferably a secondary utility destination.

Do not add a Staff destination. Do not add separate top-level Knowledge, Activity, Brain, Operations, and Terminal destinations unless a later product decision explicitly requires them. Their useful content belongs inside Orchestra drawers, task detail, Files, or Settings.

---

## 5. Orchestra Surface

### 5.1 Floating Prompt HUD

The prompt is a floating control surface, not a rigid rectangular split-pane card. The user supplied a reference showing a tall, rounded, outlined metal/device-like panel with a prompt control near the bottom.

Implement the concept as:

- smooth large-radius outline and restrained surface treatment;
- draggable by a clear header/handle;
- resizable by the user;
- sensible minimum and maximum dimensions;
- compact mode for a small prompt-only surface;
- expanded mode for conversation, plan, approval, and review details;
- position and size can be retained per project/session if practical;
- does not hide the entire graph by default;
- does not cover important controls permanently;
- keyboard-accessible drag/resize alternatives or a reset-position action;
- touch/mobile fallback: docked full-width or bottom-sheet behavior rather than free dragging.

The prompt input should grow/shrink naturally with content. Do not force a huge fixed textarea. Short prompts remain compact; longer prompts expand up to a bounded height and then scroll.

The HUD contains:

- Orchestra conversation messages;
- concise plan proposal;
- confirmation action for plans requiring user agreement;
- synchronized approval card;
- synchronized review/result card;
- prompt input and submit/cancel controls.

### 5.2 Plan Confirmation Policy

The Orchestra proposes a plan before starting work that is large, ambiguous, destructive, or multi-step. The user must be able to confirm, reject, or request a revision.

Small, low-risk, deterministic tasks should not require a separate plan approval. They may execute directly, similar to the practical flow of coding agents such as OpenCode, while still respecting tool approval and review policy.

The frontend must render the backend's decision/state. It must not invent a local heuristic that disagrees with Orchestra.

### 5.3 Node Graph

Keep the existing node-based graph visual language as the main execution view:

- cards represent active workers/tasks, not permanent Staff profiles;
- Orchestra chooses each worker's role, display name/call sign, and task;
- cards spawn when Orchestra dispatches work;
- cards show task title, role/name, status, progress, dependencies, and relevant approval/review state;
- cards visibly distinguish running, waiting approval, reviewing, blocked, failed, and completed states;
- waiting approval/review must be visually prominent but not rely on animation alone;
- accepted/completed workers unspawn from the active graph;
- completed history/evidence remains accessible from Orchestra history/task detail;
- graph pan/zoom must remain usable behind the floating HUD;
- status is authoritative from bridge snapshots/events.

Do not render a permanent roster of 10–20 Staff cards. Logical worker identity may be shown while active, but it is lifecycle-bound.

### 5.4 Approval and Review Synchronization

Approval/review must appear in both places:

1. inline in the floating Orchestra HUD; and
2. on the relevant graph node/card.

Both views represent the same backend state. Approval actions from either surface must update the other through bridge state/events. Avoid two independent frontend approval stores.

Orchestra owns the final review decision. Worker/engine output is evidence, not success.

---

## 6. Files Surface

Files is a project-scoped surface with a worktree switcher. It is not the old Workspace dashboard.

### Three sub-areas

```text
Files
├── Repository   source code in the selected worktree
├── Support      user-provided reference/support files (read-only to workers)
└── Vault        Knowledge Markdown files
```

Use a segmented control or equivalent local sub-navigation inside Files; do not create three new sidebar destinations.

### Worktree viewer

The top of Files needs an explicit viewer switcher:

```text
Viewing: [ orchestra/task-142 ▼ ]
```

Possible entries include the main repository and active Orchestra worktrees. The selected worktree controls the file tree, file content, Git status, and diff context.

Expected behavior:

- default to the active Orchestra worktree when a task is running;
- otherwise default to the main repository;
- show lifecycle/status labels such as running, review, ready to merge, merged where supplied by backend;
- refresh after a completed atomic save/write, not on every character written by a worker;
- allow explicit refresh;
- keep worktree identity visible so the user does not confuse a task branch with main.

### File update behavior

Do **not** stream file content character-by-character while a worker is writing. Refresh on:

- completed worker save/write event;
- external editor save event when detected;
- explicit user refresh;
- task/review transition that produces a new snapshot.

### Repository

Show source code tree, file viewer, Git status, and eventually diff/review context. Editing/mutations continue to be governed by Orchestra/Harness; Files is not a second execution authority.

### Support

User-added supporting material. It should be clearly marked read-only for worker mutation. It may be read by Orchestra/worker context according to policy but must not be overwritten silently.

### Vault / Knowledge

Knowledge consists of many Markdown files, not one giant file. Example:

```text
vault/
├── project/
├── decisions/
├── specs/
├── conventions/
├── reviews/
├── learnings/
└── notes/
```

Orchestra may create/update validated knowledge after successful review. User may edit files manually. Workers submit evidence/proposals; they do not silently write organizational truth. Refresh/index on save, not per keystroke.

The existing vault/retrieval contracts remain authoritative: Markdown is source of truth, SQLite/index is rebuildable derivative, malformed content fails closed, and provenance must be retained.

---

## 7. Settings Scope

Settings should be a compact utility surface, not a primary work tab. It may contain:

- provider/model configuration and credential references (never display secrets);
- engine policy/status: Pi primary, OMP specialist;
- theme and interface preferences;
- diagnostics and bridge status;
- retrieval/index maintenance;
- reset HUD position/size and other UI preferences if persisted.

Do not place task execution, worker roster, or project browsing in Settings.

---

## 8. Frontend/Backend Work Split

Use a **contract-first parallel workflow**:

### Backend owns

- Wails DTOs and TypeScript-facing contract stability;
- Orchestra plan/task/approval/review lifecycle;
- authoritative task and worker events/snapshots;
- active worktree listing/selection/read/diff contracts;
- save-completed/file-change events;
- Vault write/provenance/retrieval behavior;
- Pi/OMP engine adapters and worker lifecycle.

### Frontend owns

- project picker and shell transition;
- collapsible sidebar behavior;
- floating/resizable prompt HUD;
- graph/node presentation;
- synchronized approval/review presentation;
- Files tree/viewer/worktree switcher;
- responsive/accessibility behavior;
- loading, empty, offline, error, and permission states.

### Parallelization rule

Frontend may build against typed bridge interfaces and deterministic fixtures while backend completes implementation. Do not fabricate runtime data in production routes. When a contract is missing, add a typed boundary/mock adapter only in tests or story-like development fixtures; do not silently make up metrics, workers, approvals, or task state.

The backend should freeze/update contracts before frontend integration points are changed. Contract changes require coordinated updates to `web/src/types/`, bridge adapters, tests, and Go Wails bindings.

---

## 9. Phased Roadmap

### Phase A — Shell and entry flow

- Replace dashboard-first entry with project/office picker.
- Add project context and switcher to shell.
- Implement collapsible desktop sidebar and mobile overlay.
- Reduce navigation to Orchestra, Files, Settings.
- Preserve bridge/offline states.

**Gate:** user can choose/create a project and enter a clean Orchestra workspace without seeing the old nine-tab dashboard.

### Phase B — Orchestra workspace

- Mount existing node graph as the full main surface.
- Build floating prompt HUD with drag, resize, compact/expanded modes, and reset position.
- Render conversation and plan proposal states.
- Add confirm/reject/revise actions for plan-required flows.
- Wire graph events/snapshots.

**Gate:** a real backend snapshot/event can render without fake worker state, and HUD does not permanently block graph controls.

### Phase C — Approval and review loop

- Render approval/review cards inline in HUD and node.
- Make actions idempotent and bridge-backed.
- Refresh both surfaces after action/event.
- Keep completed worker history accessible while removing completed workers from active graph.

**Gate:** approval/review action from either surface produces the same visible state in both.

### Phase D — Files and Knowledge

- Add Repository/Support/Vault segmented sub-areas.
- Add active worktree viewer switcher.
- Add save-completed refresh behavior.
- Add code/Markdown viewer and diff context.
- Display Knowledge as many categorized Markdown files with provenance.

**Gate:** selecting a worktree changes the viewer context, and file content does not update character-by-character.

### Phase E — Integration QA

- Project picker → prompt → plan → confirm → worker spawn → save → review → accept → worker unspawn.
- Verify offline/empty/loading/error states.
- Verify 375px, 768px, 1024px, and desktop layouts.
- Run frontend tests, lint, typecheck, and build.
- Run integrated `wails dev` bridge verification; `npm run dev` is for frontend-only work.

---

## 10. Frontend Guardrails

- This is React + Vite + TypeScript + Wails, not Next.js. Adapt `next/link` examples to existing hash/state navigation.
- Reuse `web/src/tokens.css`, `web/src/styles.css`, existing bridge adapters, and Lucide icons.
- Do not add a new dependency for drag/resize unless existing dependencies cannot support the minimum behavior.
- Prefer native CSS and pointer events for a simple draggable/resizable HUD before adding a library.
- Do not preserve old navigation merely for backward compatibility if it conflicts with this product direction; preserve behavior only where it remains useful inside Orchestra/Files/Settings.
- Do not add fake AI chat, fake plans, fake workers, fake approvals, or fake file contents to production paths.
- Every new non-trivial interaction needs a focused test or a runnable headless check.
- Keep the local design backlog `docs/specs/phase-14-design-backlog.md` uncommitted.

---

## 11. Definition of Done

The frontend direction is considered aligned when:

- opening the app presents a project/office choice or creation flow;
- entering a project presents Orchestra as the default workspace;
- the sidebar is collapsible and icon-first like the supplied reference;
- the prompt HUD is smooth, floating, movable, resizable, and content-aware;
- the graph occupies the main canvas and shows active ephemeral workers;
- Orchestra plan confirmation is required only when backend policy says it is required;
- approval/review is synchronized between HUD and graph node;
- accepted workers disappear from the active graph while history remains available;
- Files contains Repository, Support, and Vault with a worktree switcher;
- Knowledge is represented as multiple purpose-named Markdown files;
- no standalone Staff tab or dashboard-first nine-tab architecture remains;
- no runtime state is fabricated by the frontend.
