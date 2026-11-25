# HELIOS Agent Instructions

These instructions apply to work in this repository on any machine.

## Resume work

The user normally starts a session with `agent update in` (see Session commands), which pulls and reads the handoff. Without it:

1. Read `docs/CONTINUATION.md` for the current handoff.
2. Verify the repository root and worktree with `git rev-parse --show-toplevel` and `git status --short --branch`.
3. Treat the handoff as a pointer, not unquestionable truth. Verify only the files and acceptance checks for the current task; do not rescan the whole repository by default.
4. Use `docs/03_ROADMAP.md` for phase order and the relevant section of `backlog/phase-0.yaml` for acceptance criteria. Do not consult or update ADO unless the user asks.

Use repository-relative paths. Do not assume a Mac path, a particular drive letter, or that a sibling/archive checkout is canonical. The Git root of the current workspace is authoritative. Preserve existing user changes.

## Project boundaries

HELIOS is a DevOps/SRE/Platform/SecOps learning project. Trackline is its supplied fleet-telemetry application. The operator owns infrastructure and platform artifacts. Explain, review, or validate those by default; implement them when the user explicitly asks. Do not provision cloud resources without discussing cost and cleanup.

Follow the roadmap in order. Distinguish user-reported completion from repository evidence and commands actually verified. A file's existence is not proof that its acceptance criteria pass. Do not reveal intentionally seeded application weaknesses before the user discovers them through the planned exercises.

## Remotes

HELIOS is pushed to two remotes that must stay in sync. Identify them by URL, not by name: names differ by machine, and a branch's tracking upstream can differ too.

| Remote | URL | Names seen |
| --- | --- | --- |
| GitHub | `https://github.com/kzmichael/Helios.git` | `github` on Windows; `origin` on macOS (reported 2026-11-20) |
| Azure DevOps | `https://Kugleblitz712@dev.azure.com/Kugleblitz712/Helios/_git/Helios` | `azure` and `origin` on Windows (same URL) |

Run `git remote -v` each time and map the names to these URLs; remotes sharing a URL count once. If a machine lacks one of the two URLs, report it and ask before adding the remote. Never use `--force`, `--no-verify`, or a destructive history rewrite on either remote.

## Session commands

Two commands bracket a work session on any machine. A bare "agent update" means `agent update out`.

### `agent update in` (start of a session: pull and read)

1. Verify the repository root, `git status --short --branch` and `git remote -v`, and map the remotes to the URLs above.
2. If there are uncommitted changes, list them and do not stash, reset, or discard them. Continue only if the pull cannot touch them; otherwise stop and ask.
3. Run `git fetch` for each distinct remote URL.
4. Compare the current branch with the same branch on each remote. If a remote is ahead and the local branch can fast-forward, run `git pull --ff-only <remote> <branch>` from the remote that is furthest ahead. If the local branch has diverged from a remote, the two remotes have diverged from each other, or the branch is missing on a remote, stop and report; never merge, rebase, or force.
5. Read `docs/CONTINUATION.md` without editing it. Report briefly: when and where it was last updated, which commits just arrived (`git log` from the previous HEAD), what was completed, what was verified versus only reported, blockers, and the next task with its acceptance checks. Run cheap checks only for claims the next task depends on.
6. Say whether the two remotes are level with each other and with the local branch, so it is clear whether `agent update out` will push to both.
7. Do not commit or push, and do not update ADO.

### `agent update out` (end of a session: write and push)

When the user says **"agent update out"** (or just "agent update", or asks to update the handoff), update `docs/CONTINUATION.md` before ending the task:

- Review the current conversation's completed work, `git status`, relevant diffs, and validation results.
- Record what changed, what was verified, what was only reported by the user, blockers, and the single best next task with its acceptance evidence.
- Keep it concise and machine-independent. Replace stale current-state claims; retain only history useful to resume. Never claim a test, deployment, push, or acceptance that was not observed.
- Do not include credentials, tokens, private keys, or sensitive values.
- Do not write claims about the commit or push into the handoff, because it is written before they happen; report their results in the reply instead.
- Do not update ADO unless explicitly asked; the user may synchronize it later.

After the handoff is written, **publish it** to both remotes (this applies only to `agent update out`; at any other time do not commit or push unless the user asks):

1. Run `git fetch` for each remote. If the current branch is behind or has diverged on any remote, stop and report it; never force-push, rebase, or merge on your own.
2. Stage with `git add -A`, then scan the staged changes with `mise exec -- gitleaks git --pre-commit --staged --redact -c security/gitleaks/gitleaks.toml` (the commit hook may not be active on every machine). If it reports anything, or `git status` shows unexpected files such as credentials, keys, or large artifacts, stop and ask. Never use `--no-verify`.
3. Commit with the message `Agent update: <short summary>`.
4. Push the current branch to both remotes in the Remotes table with `git push <remote> <branch>`, using whichever local name maps to each URL. Remotes that share a URL (for example `origin` and `azure`) count once. If only one of the two URLs exists on this machine, push to it and report that the other was skipped.
5. In the reply, report each remote's push result and the final commit hash. If a push fails, say so and do not retry with force.

At the start of a later session, `agent update in` reads this handoff, verifies its relevant claims locally, and continues from its next task unless the user redirects you. If the handoff conflicts with the user's latest message or the checked-out repository, follow the latest user instruction and current repository, then correct the handoff when asked or when completing `agent update out`.
