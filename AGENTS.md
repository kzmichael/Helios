# HELIOS Agent Instructions

These instructions apply to work in this repository on any machine.

## Resume work

1. Read `docs/CONTINUATION.md` for the current handoff.
2. Verify the repository root and worktree with `git rev-parse --show-toplevel` and `git status --short --branch`.
3. Treat the handoff as a pointer, not unquestionable truth. Verify only the files and acceptance checks for the current task; do not rescan the whole repository by default.
4. Use `docs/03_ROADMAP.md` for phase order and the relevant section of `backlog/phase-0.yaml` for acceptance criteria. Do not consult or update ADO unless the user asks.

Use repository-relative paths. Do not assume a Mac path, a particular drive letter, or that a sibling/archive checkout is canonical. The Git root of the current workspace is authoritative. Preserve existing user changes.

## Project boundaries

HELIOS is a DevOps/SRE/Platform/SecOps learning project. Trackline is its supplied fleet-telemetry application. The operator owns infrastructure and platform artifacts. Explain, review, or validate those by default; implement them when the user explicitly asks. Do not provision cloud resources without discussing cost and cleanup.

Follow the roadmap in order. Distinguish user-reported completion from repository evidence and commands actually verified. A file's existence is not proof that its acceptance criteria pass. Do not reveal intentionally seeded application weaknesses before the user discovers them through the planned exercises.

## `agent update` handoff

When the user says **"agent update"** (or asks to update the handoff), update `docs/CONTINUATION.md` before ending the task:

- Review the current conversation's completed work, `git status`, relevant diffs, and validation results.
- Record what changed, what was verified, what was only reported by the user, blockers, and the single best next task with its acceptance evidence.
- Keep it concise and machine-independent. Replace stale current-state claims; retain only history useful to resume. Never claim a test, deployment, push, or acceptance that was not observed.
- Do not include credentials, tokens, private keys, or sensitive values.
- Do not commit or push automatically. The user pushes from whichever machine they are working on.
- Do not update ADO unless explicitly asked; the user may synchronize it later.

At the start of a later session, read this handoff first, verify its relevant claims locally, and continue from its next task unless the user redirects you. If the handoff conflicts with the user's latest message or the checked-out repository, follow the latest user instruction and current repository, then correct the handoff when asked or when completing `agent update`.
