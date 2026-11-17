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
- Do not write claims about the commit or push into the handoff, because it is written before they happen; report their results in the reply instead.
- Do not update ADO unless explicitly asked; the user may synchronize it later.

After the handoff is written, **publish it** (this applies only to `agent update`; at any other time do not commit or push unless the user asks):

1. Run `git fetch` for each remote. If the current branch is behind or has diverged on any remote, stop and report it; never force-push, rebase, or merge on your own.
2. Stage with `git add -A`, then scan the staged changes with `mise exec -- gitleaks git --pre-commit --staged --redact -c security/gitleaks/gitleaks.toml` (the commit hook may not be active on every machine). If it reports anything, or `git status` shows unexpected files such as credentials, keys, or large artifacts, stop and ask. Never use `--no-verify`.
3. Commit with the message `Agent update: <short summary>`.
4. Push the current branch to every distinct remote URL in `git remote -v` with `git push <remote> <branch>`. Remotes that share a URL (for example `origin` and `azure`) count once. Remotes differ by machine, so list them each time instead of assuming names.
5. In the reply, report each remote's push result and the final commit hash. If a push fails, say so and do not retry with force.

At the start of a later session, read this handoff first, verify its relevant claims locally, and continue from its next task unless the user redirects you. If the handoff conflicts with the user's latest message or the checked-out repository, follow the latest user instruction and current repository, then correct the handoff when asked or when completing `agent update`.
