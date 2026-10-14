# Phase 14 Design System

CortexOS uses a compact executive control-room visual system. Keep the interface quiet, dense, and operational: surfaces are structured with hairline borders, brass is reserved for primary action, and blueprint slate marks navigation and logic.

## Tokens

The canonical token source is `web/src/tokens.css`, imported by `web/src/styles.css`.

- **Surfaces:** `--surface-base`, `--surface-panel`, `--surface-card`, `--surface-inset`, `--surface-hover`, `--surface-active`
- **Text:** `--text-primary`, `--text-secondary`, `--text-muted`, `--text-subtle`, `--text-inverse`
- **Accents:** `--accent-warm` for brass/bronze action, `--accent-cool` for blueprint/nav logic
- **Status:** `--status-success`, `--status-warning`, `--status-danger`, `--status-info`, `--status-idle` and their soft/border variants
- **Layout:** `--radius-sm`, `--radius-md`, `--radius-lg`, `--radius-full`, `--shadow-sm`, `--shadow-md`, `--shadow-lg`
- **Motion:** `--duration-fast`, `--duration-base`, `--duration-slow`, `--ease-standard`

The shell applies `data-theme="light"` or `data-theme="dark"`; components must use semantic tokens rather than duplicating theme-specific values.

## Typography

- `--font-sans` is the interface face.
- `--font-mono` is used for IDs, statuses, metrics, labels, and protocol data.
- Scale utilities are available as `.text-display`, `.text-display-lg`, `.text-section`, `.text-card`, `.text-body`, `.text-body-sm`, `.text-micro`, and `.text-micro-sm`.
- Use `var(--leading-tight)`, `var(--leading-snug)`, and `var(--leading-normal)` for line height.

## Icons

Use `lucide-react` for interface icons. Icon-only controls require an accessible `aria-label`; decorative icons use `aria-hidden="true"`. Keep protocol markers in terminal/evidence output only when they are part of the integration contract.

## Component patterns

- **Topbar:** 56px control surface, brand mark, breadcrumb, approval queue pill, governed status chip, and theme control.
- **Navigation:** semantic links with `aria-current="page"`; active state uses the warm accent.
- **Cards:** one-pixel token border, `--radius-md`/`--radius-lg`, structured metadata, and no heavy decorative shadow.
- **Drawers:** fixed shell, solid header, isolated `min-height: 0` scroll body, and full-screen `100dvh` mobile behavior.
- **Segmented controls:** `role="tablist"` with each option as a `role="tab"`, `aria-selected`, and visible focus ring.
- **Status:** communicate state with text and color together; never rely on color alone.
- **Phaser:** read computed semantic CSS colors at scene creation/render time and retain fallbacks for non-DOM/test contexts.

## Responsive and accessibility rules

- Use fluid containers and preserve a 44px minimum touch target for primary controls and mobile drawer close controls.
- At mobile widths, drawers occupy the viewport instead of compressing the underlying layout.
- Use `:focus-visible` with `--focus-ring`; do not remove keyboard outlines.
- Honor `prefers-reduced-motion`; pulse, reveal, and transition effects must stop or become effectively instantaneous.
- Preserve contrast in both themes; status colors are paired with labels and borders.

## Do not

- Do not add emoji as UI iconography.
- Do not introduce raw theme colors or one-off font stacks in new components.
- Do not use a custom icon for a control when a Lucide icon exists.
- Do not simulate runtime, bridge, approval, or orchestration state for visual effect.
- Do not add gradients, shadows, or animation without an operational reason.
- Do not push the local design backlog; `phase-14-design-backlog.md` is the local source of truth.
