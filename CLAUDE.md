# CortexOS — Frontend & UI Redesign (Claude Code Guide)

## 🎯 Project & Mission
You are working on **CortexOS**, an **Executive Control Room** native desktop console application built with React + Vite + TypeScript.
The user specifically asked for you (Claude Code) to lead the frontend/UI overhaul because the previous iterations fell far short of their standards ("masih ampas broo", "kayak gada perubahan", "masih jelek").

## 🎨 Design Vision & References
- **Aesthetic Inspiration:** **21st.dev**, **shadcn/ui**, **AgentOS**.
- **Mode:** Executive Control Room / Operational Cockpit — dense, cohesive, high-information-density desktop console (NOT a marketing landing page, NO oversized editorial hero text like "Keep the ground reliable").
- **Priority Areas from User:**
  1. **Sidebar (Navigation Rail):** High-density, grouped navigation, workspace selector, live runtime bridge connection indicator, clean hover/active states.
  2. **Topbar / Navbar:** Breadcrumb navigation, dynamic status badges, tool approvals queue trigger, theme toggle, telemetry.
  3. **Workspace View & Sub-screens:** Bento KPI cards, sleek project list with action affordances, compact form drawers.
- **Themes:** Dual-mode support:
  - Dark: obsidian / brass / blueprint
  - Light: drafting-paper / bronze / steel-navy
  - Tokens reside in `web/src/tokens.css` and are consumed across `web/src/styles.css`.

## ⚙️ Technical Constraints & Guardrails
- **Preserve Real Runtime Bridges:** The UI connects to a real Go/Wails backend via `wailsRuntimeBridge` (defined in `web/src/types.ts` and `web/src/bridge/`). Never mock or fabricate fake metrics, fake approvals, or fake execution state in production routes.
- **Iconography:** Use `lucide-react` consistently (do not hand-roll random SVG paths or emojis).
- **Quality Gates (must pass before declaring any work done):**
  - Run in `web/`:
    - `npm test` (all 219 tests / 37 suites must pass)
    - `npm run lint`
    - `npm run typecheck`
    - `npm run build`
- **Backlog & Specs:** Keep `docs/specs/phase-14-design-backlog.md` and related notes local; do not push.

## 🤝 Communicating with the User
- The user speaks Indonesian / informal ("santai", "bro", "lu/gua").
- Be direct, show designs or visual structure clearly, ask for their feedback, and avoid overclaiming completion until verified.
