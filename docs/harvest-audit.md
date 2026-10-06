# CortexOS Harvest Audit

**Issue:** BAN-26
**Audit date:** 2026-10-06
**Policy:** Raw clones remain local-only. Extracted material is reference-only until license, attribution, dependency, and asset rights review is complete.

## Provenance manifest

The machine-readable manifest is `_harvest/harvest-manifest.json`. It records the repository URL, requested ref, observed commit SHA, detected license/notice files, repository status, license status, and review flag.

| Source | URL | Ref | Observed commit | Detected license | Preliminary compatibility | Adoption status |
| --- | --- | --- | --- | --- | --- | --- |
| pi | https://github.com/badlogic/pi-mono.git | main | `428a12bc775145afa342530a9eaa652efb3e4422` | `LICENSE` / MIT | Compatible for reference; preserve notice if code is adopted | Reference-only |
| oh-my-pi | https://github.com/can1357/oh-my-pi.git | main | `3f000c524cf82279f804ffd7526280cc9a5f25fe` | Multiple LICENSE/NOTICE files | Requires dependency and nested-license review | Reference-only |
| agency-agents | https://github.com/msitarzewski/agency-agents.git | main | `83294689da3832c0a9f223221148c411fd3eacc0` | `LICENSE` / MIT | Compatible for reference; inspect prompt/assets attribution | Reference-only |
| agent-teams-ai | https://github.com/777genius/agent-teams-ai.git | main | `90625bee6409b19b39a6a021c7f976f2b6a7c6ae` | `LICENSE` / AGPL-3.0 | Copyleft/network obligations require legal decision before code adoption | Blocked pending legal review |
| ai-town | https://github.com/a16z-infra/ai-town.git | main | `8e05997f2409275669c8344b84a51692e83f3f33` | `LICENSE` / MIT | Compatible for reference; review dependencies and assets | Reference-only |
| ai-office | https://github.com/ChristianFJung/AIOffice.git | main | `ae6ed0e61ead26467253d9a56ae697c97a42e623` | `LICENSE`, asset `LICENSE.txt` / MIT plus asset terms | Asset-specific obligations must be preserved separately | Reference-only |
| pixel-agents | https://github.com/pixel-agents-hq/pixel-agents.git | main | `3537e140c2094761beae748592aeb92ece8edfdd` | `LICENSE` / MIT | Review actual asset paths and attribution before use | Reference-only |
| chatdev | https://github.com/OpenBMB/ChatDev.git | main | `4fb2db0ea90375ce1059f44fe03ffbd191a7a169` | `LICENSE` / Apache-2.0 | Preserve Apache notices and dependency obligations if adopted | Reference-only |
| opencode-harness | https://github.com/Awaiswilll/opencode-harness.git | main | `20cbdc1a79de48d93ee54c683b4038fa97758174` | Nested `skills/design/ui-styling/LICENSE.txt`; GitHub metadata declares no repository license | Repository-wide rights are ambiguous; do not adopt code, skills, or assets | Blocked pending legal review |

## Findings

### Resolved source issue

The original Pi URL (`mariozechner/pi-coding-agent`) did not resolve as a Git repository. The official source is the `badlogic/pi-mono` monorepo; the coding agent is under `packages/coding-agent`. The harvest script now uses the monorepo URL.

### Structure mismatches

The harvest script reports missing paths without deleting user work:

- `opencode-harness/commands` is absent at the pinned commit.
- `pixel-agents/assets` is absent at the pinned commit; inspect the actual asset layout before any extraction.
- `chatdev/ChatDev/prompts` is absent at the pinned commit.

These are audit findings, not permission to substitute unrelated paths.

### License and attribution rules

- `reviewed` remains `false` for every source until a human/legal review records the decision.
- A detected license file means **review required**, not automatic permission to copy.
- AGPL-3.0 material and repositories without a clear repository-wide license are blocked from adoption.
- Nested licenses, notices, package dependencies, fonts, tilesets, audio, and other assets require independent review.
- No harvested third-party code is CortexOS source. Curated excerpts must carry source and commit attribution.

## Validation evidence

Commands/checks performed:

- `git ls-remote` and GitHub metadata checks verified the configured repository URLs.
- `scripts/harvest.ps1` completed cloning with shallow repositories and wrote `_harvest/harvest-manifest.json`.
- `scripts/harvest.ps1 -SkipClone -Extract` reproduced provenance and extraction checks without recloning.
- `git diff --check` passed.
- Raw clones remain ignored by Git; the manifest and attribution review are explicitly trackable.

## Open risks

1. Full dependency/license trees have not been mechanically generated for every source.
2. Asset rights and third-party package licenses need review before porting.
3. The harvest script currently copies reference material for analysis but does not promote it to product source; this is intentional.
4. `reviewed` must only become `true` after the Project Lead accepts the review evidence.
