# HELIOS Scaffold Status

This folder is the isolated working copy for the public HELIOS repository.

## What was copied

- Existing HELIOS documentation, backlog, contracts, schemas, seed data, and delivered
  Phase 0 application source were copied from the parent `Major Project/` folder.
- The parent folder was not modified or converted into a separate Git repository.
- The public repository should be connected only from this `helios/` folder.

## What was scaffolded

Paths described as future work in `docs/07_REPOSITORY_BLUEPRINT.md` were created as
empty placeholders. They are not implemented infrastructure, deployment configuration,
security policy, observability, or test coverage.

Before using a placeholder, replace it with a real implementation and validate it. Empty
files exist only to make the planned repository shape visible and to prevent accidental
path invention.

## Ownership

The application source under `apps/` is supplied for you to build, test, and debug. The
operator-owned areas remain yours to implement:

- `infra/`
- `deploy/`
- `observability/`
- `security/`
- `platform/`
- `ops/`
- `tests/`
- `.github/`
- `scripts/`
- root tooling such as `Taskfile.yml` and Compose files

## First export verification

Before the first public push:

1. Run `git status` from this `helios/` directory.
2. Confirm no `.env`, private key, Terraform state, kubeconfig, or parent-workspace file
   is present.
3. Run gitleaks against the complete tree.
4. Review `git diff --no-index` against the intended source export.
5. Commit and push only from this directory.