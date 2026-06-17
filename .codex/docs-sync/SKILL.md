---
name: docs-sync
description: Use when invest-compass changes affect docs, implementation checklist progress, MVP scope, Tauri/Rust commands, Go core APIs, SQLite migrations, sidecar lifecycle, SSE task events, AI Provider/model behavior, Prompt templates, credential storage, settings, packaging, validation commands, or long-lived project rules.
---

# Docs Sync for Invest Compass

## Goal

Keep this repository's Simplified Chinese docs, implementation checklist, acceptance notes, README, and long-lived agent rules aligned with real code and validated behavior.

This skill only handles documentation/rule synchronization. It does not replace implementation, testing, security review, or code review.

## Project Shape

Current planned shape:

```text
invest-compass/
├── apps/desktop/       # Tauri v2 desktop shell and Rust commands
├── frontend/           # React + TypeScript + Vite UI
├── core/               # Go sidecar core
├── packages/shared/    # shared API schema/types
├── scripts/            # build/dev/release scripts
├── docs/               # technical solution, checklist, acceptance docs
├── README.md
└── AGENTS.md
```

The repository is still in early planning. Do not assume these directories or build commands exist until T01 in the implementation checklist is completed and verified.

## Authoritative Documents

Update only documents whose ownership matches the change.

- `docs/2026-06-17-invest-compass-technical-solution.md`
  - Current authoritative technical solution.
  - Update when product/MVP scope, architecture, security boundary, API contract, database schema, task model, page model, packaging, or acceptance criteria changes.

- `docs/2026-06-17-invest-compass-implementation-checklist.md`
  - Current authoritative execution checklist.
  - Update task status, dependencies, validation evidence, blockers, residual risks, and Review Gate state.
  - Only change `[ ]` / `[~]` / `[!]` to `[x]` after implementation and required verification are complete.

- `README.md`
  - Repository entry point.
  - Update when project layout, startup commands, development workflow, scope summary, or user/developer-facing entry points change.

- `AGENTS.md`
  - Long-lived project rules for future agents.
  - Update when reusable workflow, validation, security boundary, or docs-sync behavior changes.
  - Do not record one-off task notes.

- Future acceptance docs, for example `docs/acceptance-report.md`
  - Use for release/internal-test evidence, pass/fail lists, unverified items, platform-specific manual checks, and residual risks.

## Trigger Conditions

Use this skill when a current change affects any of these areas:

- MVP scope or non-goals.
- `docs/` technical solution, checklist, acceptance report, or release notes.
- `AGENTS.md`, `.codex/docs-sync/SKILL.md`, or long-lived project rules.
- `apps/desktop/src-tauri/` Rust commands, capabilities, CSP, updater/check-update, tray, notification, autostart, sidecar management, or credential integration.
- Go core server, middleware, local API, token validation, request/trace IDs, logging, panic recovery, secret redaction, or provider status.
- SQLite schema, migrations, sqlc queries, storage transactions, backups, workspace path, or RUNNING task recovery.
- Sidecar token handshake, port/token lifecycle, process shutdown, protocol version compatibility, or externalBin packaging.
- SSE task event stream, Rust event/channel forwarding, task history, event replay, cancellation, or report idempotency.
- Stock, market, K-line, indicator, news, dashboard, provider-status, or data-source behavior.
- AI Provider/model configuration, API Key handling, `resolved_api_key` injection, Prompt templates, Prompt variables, model testing, or analysis output constraints.
- Settings, cache, proxy, workspace, logs export, update check, about page, or FREE license placeholder.
- Frontend pages, routes, command bindings, empty/error states, Markdown rendering, external links, or removal/addition of user-visible entry points.
- Build/test commands, dependencies, packaging targets, signing/notarization notes, or CI.

## Non-Triggers

Usually skip docs when the change is:

- Pure formatting with no behavior, API, schema, workflow, validation, or long-lived rule change.
- Test-only refactoring that does not change validation coverage or documented commands.
- CSS-only polish that does not alter user-facing workflows or page availability.
- Temporary debugging, failed experiments, local logs, generated artifacts, or machine-local files.
- Incomplete implementation that is not being documented as a blocker/risk.

## Scope Rules

- Keep docs in Simplified Chinese unless editing YAML frontmatter or code identifiers.
- Keep edits narrow and traceable.
- Do not document future or planned work as completed behavior.
- Do not mark checklist tasks complete from partial implementation.
- Do not mark platform behavior complete without platform-specific verification when the checklist requires it.
- Do not treat Rust unit tests alone as proof that macOS Keychain, Windows Credential Manager, packaging, or real desktop UI behavior is complete.
- If technical solution and checklist conflict, prefer the more specific checklist for execution and record the conflict.

## Change-to-Docs Mapping

- MVP scope, non-goals, or page availability changes
  - Update technical solution scope sections, checklist task/Review Gate wording, README summary, and AGENTS.md if the rule should persist.

- Rust command, capability, CSP, sidecar, or Tauri desktop behavior changes
  - Update technical solution desktop/security/API sections.
  - Update checklist tasks around T03-T07, T20-T21, T27, T37, T39-T42 as applicable.
  - Record platform-specific verification evidence when claiming completion.

- Go API, response shape, task events, SSE, or provider behavior changes
  - Update technical solution API/module sections.
  - Update checklist tasks around T03, T10-T19, T21-T29.
  - Keep Rust command and Go API contracts aligned.

- SQLite migration, sqlc, storage, backup, or task recovery changes
  - Update technical solution database/storage sections.
  - Update checklist tasks T08-T11, T25, T28, T29.
  - Never rewrite published migrations silently.

- AI config, credentials, Prompt, analysis, or report behavior changes
  - Update technical solution AI/security/Prompt/report sections.
  - Update checklist tasks T20-T29 and frontend tasks T34-T36.
  - Explicitly preserve Keychain/Credential Manager and redaction boundaries.

- Frontend page or route changes
  - Update technical solution page list and checklist T30-T38.
  - Remove or document any entry that would expose non-MVP features.
  - Do not present fake data/buttons as completed behavior.

- Settings, cache, proxy, logs, update check, packaging, or release changes
  - Update technical solution settings/release/testing sections.
  - Update checklist T37-T44 and README if developer/user workflows changed.

- Verification command or CI changes
  - Update README, AGENTS.md, and checklist verification sections.
  - Prefer commands defined by actual repository files over inferred commands.

## Workflow

1. Inspect current state:

   ```bash
   git status --short
   git diff --stat
   git diff -- <path>
   git ls-files --others --exclude-standard
   ```

2. Classify the impact:

   - Docs-only / skill-only
   - MVP scope / product boundary
   - Tauri/Rust command or desktop capability
   - Go core API/server/provider
   - SQLite/migration/storage
   - Sidecar lifecycle/security
   - SSE/task/report lifecycle
   - AI credential/Provider/Prompt behavior
   - Frontend page/workflow
   - Settings/cache/proxy/logs/update
   - Build/test/dependency/config
   - Packaging/release/platform verification
   - Long-lived agent rule

3. Locate candidate checklist items:

   ```bash
   rg -n "T[0-9]{2}|RG[0-9]|Review Gate|sidecar|Rust|Go|SQLite|sqlc|SSE|任务|报告|Prompt|Provider|API Key|Keychain|Credential|设置|缓存|代理|打包|验收" docs/2026-06-17-invest-compass-implementation-checklist.md
   ```

4. Decide document updates:

   - Update the technical solution when the intended contract changes.
   - Update the checklist when implementation status, validation evidence, blockers, or task wording changes.
   - Update README when user/developer entry points change.
   - Update AGENTS.md when a reusable project rule changes.

5. Edit docs/rules:

   - Use concise Simplified Chinese.
   - Keep checklist syntax as `[ ]`, `[~]`, `[x]`, `[!]`.
   - Include exact commands and observed validation results when marking progress.
   - Record unverified work as "未验证 / 风险 / 阻塞"; do not present it as done.
   - Link to exact file paths, commands, task IDs, or Review Gates where helpful.

6. Validate docs/rules:

   ```bash
   git diff --check
   ```

   If docs mention paths, commands, migrations, task IDs, command names, or generated files, verify those references exist when feasible.

## Verification Commands

Do not invent commands. Prefer repository-defined commands in this order:

1. `AGENTS.md` / nearer `AGENTS.override.md`
2. `Makefile`
3. `package.json`
4. `go.mod`
5. `Cargo.toml`
6. README or docs commands

Before the project skeleton exists, docs-only or skill-only changes require:

```bash
git diff --check
```

After the project skeleton exists, use actual repository commands. Initial expected command families are:

```bash
pnpm install
pnpm build
pnpm test
go test ./...
cargo check
pnpm tauri build
```

Run platform/manual verification when claiming these areas complete:

- macOS Keychain
- Windows Credential Manager
- sidecar process startup/shutdown
- Tauri capabilities/CSP denial cases
- SSE Rust forwarding
- tray/notification/autostart behavior
- macOS/Windows packaging
- signing/notarization or signing plan

If a validation command cannot be run, document the reason, impact, and residual risk.

## Invest Compass Rules to Preserve

- First release is local desktop only.
- The frontend never directly accesses Go sidecar.
- Rust command proxy must be whitelist-only; no generic `core_request`.
- Go sidecar only listens on `127.0.0.1`.
- runtime token is transferred by stdin handshake, never argv/env/log/config/database.
- SSE is subscribed by Rust and forwarded to the frontend; no browser `EventSource` to Go core.
- API Key and proxy password live in system credential storage; SQLite stores references and masked state only.
- `resolved_api_key` is internal-only and must not reach frontend types, OpenAPI, SQLite, task events, report snapshots, logs, or exported files.
- Logs, errors, task events, and exports must be redacted for secrets and user position input.
- RUNNING tasks must be recovered to a terminal state on Go core restart.
- Reports are idempotent by `task_id`.
- Prompt templates only expose MVP-supported template types and variables.
- AI output must include risk, data-timeliness, and non-investment-advice boundaries.
- MVP UI must not expose strategy observation, license activation, dedicated announcement/research/fund-flow sources, trading, or real auto-update install flows.
- Check update is MVP-only version提示; no download/install/silent update.
- FREE license status is placeholder only; no activation flow in MVP.

## Final Response Requirements

When this skill is used, the final reply must include:

- Which docs or skill files were updated.
- Which candidate docs were checked and intentionally not updated, with reasons.
- Which checklist items were marked complete; if none, say none.
- Validation commands run and results.
- Remaining risks or unverified items.

## Prohibitions

- Do not mark checklist work complete without implementation and verification evidence.
- Do not document future/planned work as completed behavior.
- Do not broaden a narrow docs sync into a full design rewrite.
- Do not weaken sidecar token, Rust command whitelist, SSE forwarding, credential storage, or redaction boundaries.
- Do not commit secrets, tokens, passwords, private config, local logs, generated build outputs, `.DS_Store`, or unrelated files.
- Do not add dependency lockfile churn unless dependencies actually changed and the user approved the dependency change.
