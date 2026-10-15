# Phase 14 Design Backlog: Executive Control Room UI Overhaul

**Status**: `IMPLEMENTED` — Phase 14 design pass complete.  
**Source**: Conversation design direction (2026-10-14).  
**Objective**: Transform CortexOS orchestration surface from "AI slop default" to **native executive control room / modern architectural operator console** with dark + light modes, zero emoji, professional iconography, and cohesive typography/color system.

---

## 1. Design Direction Summary

| Dimension | Decision |
|-----------|----------|
| **Visual Metaphor** | Executive desk at midnight (dark) / Architectural drafting table by day (light) |
| **Dark Base** | Obsidian charcoal `#090c10` (not pure `#000000`) |
| **Warm Accent** | Aged Brass `#c89345` / `#d4a359` (office prestige) |
| **Logic Accent** | Blueprint Slate `#4f7cb8` (connectors, active nav) |
| **Light Base** | Warm drafting paper `#f6f5f0` |
| **Light Accent** | Antique Bronze `#926723` + Steel Navy `#1e40af` |
| **Status Colors** | Emerald `#3e9b5f` (success), Muted Amber `#d97706` (approval), Crimson `#dc2626` (blocked), Slate `#64748b` (idle) |
| **Typography** | Geist Sans / Inter (UI) + Geist Mono / JetBrains Mono (data) |
| **Type Scale** | Display 22px, Section 12px mono UC, Card 14px, Body 13px, Micro 11px mono |
| **Iconography** | `lucide-react` (stroke 1.5px, 14–16px) — **zero emoji** |
| **Corner Radius** | Unified `rounded-md` (6px) for cards/buttons, `rounded-lg` (8px) for panels |
| **Elevation** | Hairline borders `rgba(255,255,255,0.07)` dark / `#dedbd2` light — no heavy shadows |

---

## 2. Backlog Items (Prioritized)

| ID | Title | Description | Acceptance Criteria | Status |
|----|-------|-------------|---------------------|--------|
| **P14-01** | **Design Token Foundation** | Add CSS custom properties for all surfaces, borders, text, accents, status colors in both themes to `web/src/styles.css` (or new `web/src/tokens.css`). Export as `:root` + `[data-theme="dark"]` + `[data-theme="light"]`. | Tokens load; no raw hex in components; `dark:`/`light:` Tailwind not required. | **DONE** |
| **P14-02** | **Typography System** | Install `geist` (or `inter` + `jetbrains-mono`) via `next/font` self-host pattern; define global type scale utilities (`.text-display`, `.text-section`, `.text-card`, `.text-body`, `.text-micro`). Remove all inline `font-size`/`rem` hacks. | All text uses scale utilities; no magic numbers in CSS. | **DONE** |
| **P14-03** | **Icon Library Migration** | Add `lucide-react`; replace every emoji in `App.tsx`, `OrchestrationGraph.tsx`, `ApprovalDrawer.tsx`, `OfficeView.tsx`, `CoordinationPanel.tsx`, `OfficeScene.ts` with semantic Lucide icons (e.g., `Shield`, `Lock`, `GitBranch`, `FileText`, `Terminal`, `Users`, `Settings`, `ChevronRight`, `X`, `Check`, `AlertCircle`, `Clock`, `Play`, `Pause`, `RefreshCw`). | `grep -r "🧠\|⚡\|◈\|✓\|✕\|⌁\|▦\|⌘\|◎\|🔄\|📄\|✎\|🔍\|📁\|📂\|🔎\|⚙\|🗂\|📋" web/src` returns nothing. | **DONE** |
| **P14-04** | **Top Control Bar Redesign** | Restyle `header.topbar`: brand mark + breadcrumbs, approval pill badge (icon + label `"0 APPROVALS PENDING"` + subtle pulse dot), governed status chip. Remove emoji, unify height to 56px, consistent padding. | Visual matches executive control room spec; keyboard accessible. | **DONE** |
| **P14-05** | **Orchestration Graph Node Cards** | Refactor `OrchestrationGraph.tsx` task cards: modular hardware card aesthetic (1px border, structured header with status badge top-right, worker initials chip, progress gauge bottom 3px). Connector SVG: blueprint stroke, junction dots. | Cards render correctly in both themes; connectors scale with zoom/pan. | **DONE** |
| **P14-06** | **Inspector / Task Detail Drawer** | Restyle right drawer (`graph-drawer`): `MissionControlDialogShell` pattern — solid header, dedicated scroll body (`min-h-0 flex-1 overflow-y-auto`), sticky footer for actions. Use token colors, mono for IDs. | Drawer opens/closes smoothly; no layout shift; scroll isolated. | **DONE** |
| **P14-07** | **Tool Approval Drawer** | Apply same dialog shell pattern to `ApprovalDrawer.tsx`: header with title + count, scrollable list, sticky footer with **Approve** (primary, brass) / **Deny** (secondary, crimson outline) buttons. | Buttons have correct contrast; idle/loading/disabled states visible. | **DONE** |
| **P14-08** | **Office Mode Switcher** | Polish mode toggle (`Orchestration Graph` ↔ `Virtual Office`) as segmented control using tokens; remove emoji labels. | Toggle works; focus ring visible; mobile fits. | **DONE** |
| **P14-09** | **Virtual Office Scene (Phaser)** | Update `OfficeScene.ts` avatar markers, labels, and status rings to use token colors and Lucide icons (exported as SVG texture or overlay DOM). Remove emoji. | Phaser canvas matches theme; no visual clash. | **DONE** |
| **P14-10** | **Coordination Panel** | Restyle `CoordinationPanel.tsx`: phase columns, assignment chips, metric cards using token system; mono for numbers. | Panel readable in both themes; density consistent with graph. | **DONE** |
| **P14-11** | **Responsive & Mobile Audit** | Verify all surfaces at 375px, 768px, 1024px, 1440px: no horizontal overflow, drawers become full-screen mobile dialogs (per AgentOS `h-dvh w-screen`), touch targets ≥ 44px. | Manual browser test passes at all breakpoints. | **DONE** |
| **P14-12** | **Accessibility & Motion** | Ensure WCAG AA contrast in both themes; `prefers-reduced-motion` respected (disable pulse, transitions); focus-visible rings use `--focus` token; ARIA labels on icon-only buttons. | `axe-core` automated check passes; manual keyboard nav works. | **DONE** |
| **P14-13** | **Documentation & Handoff** | Create `docs/specs/phase-14-design-system.md` recording tokens, type scale, icon usage, component patterns, and "do not" rules for future contributors. | Doc exists; linked from `docs/specs/README.md`. | **DONE** |

> Documentation index note: this repository does not currently contain `docs/specs/README.md`; the design-system document is colocated with the local backlog in `docs/specs/`.

---

## 3. Dependencies & Sequencing

```
P14-01 (Tokens) → P14-02 (Type) → P14-03 (Icons)
       ↓
P14-04 (Topbar) ←── uses all three
       ↓
P14-05 (Graph Nodes) ←── uses tokens, type, icons
       ↓
P14-06 (Inspector) ←── uses tokens, type, icons
       ↓
P14-07 (Approval)  ←── uses tokens, type, icons
       ↓
P14-08 (Mode Switch) ←── uses tokens, icons
       ↓
P14-09 (Phaser) ←── uses tokens, icons
       ↓
P14-10 (Coordination) ←── uses tokens, type, icons
       ↓
P14-11 (Responsive) ←── all above
       ↓
P14-12 (A11y) ←── all above
       ↓
P14-13 (Docs)
```

---

## 4. Out of Scope (Deferred)

- Graph layout algorithm rewrite (force-directed, canvas/WebGL) — keep current SVG for now.
- Real-time worker motion animation (Phase 14.2+).
- Multi-project workspace switcher in topbar.
- Command palette (⌘K) — future.

---

## 5. Quick Start for Implementation

```bash
# 1. Install dependencies
cd web
npm i lucide-react
# (Geist/Inter self-host via CSS @font-face — no extra npm package)

# 2. Create token file
# web/src/tokens.css → import in styles.css @import './tokens.css';

# 3. Begin P14-01
```

---

## 6. Notes

- **Do not push** this backlog to remote. Keep local in `docs/specs/`.
- All work proceeds in **small reviewable slices** (current workflow).
- Reference implementations: AgentOS `MissionControlDialogShell`, shadcn/ui dialog patterns, 21st.dev operator console examples.
- Linear not used per user request; this Markdown file is the single source of truth.