# ADR-0001: Record architecture decisions

**Status:** Accepted
**Date:** 2026-09-16

## Context

HELIOS spans many technologies chosen from larger sets of alternatives. Without a record,
the reasoning behind each choice is lost, and the project degrades into an unexplainable
pile of configuration. The ability to justify a decision — including its downsides — is
the difference between having used a tool and having engineered with it.

## Decision

Every non-obvious decision is recorded as an ADR in `docs/adr/`, numbered sequentially,
using the format below. An ADR is required for:

- Any technology selection where a credible alternative existed
- Any topology or placement decision
- Any deliberate acceptance of a limitation, risk, or cost
- Any reversal of a previous ADR (which supersedes rather than edits it)

ADRs are immutable once accepted. Changing a decision means writing a new ADR that marks
the old one `Superseded by ADR-XXXX`.

**Numbering:** an ADR takes the next free number *at the time it is written*. ADR numbers
referenced in `03_ROADMAP.md` are indicative placeholders, not reservations — decisions
arrive in a different order than planned, and renumbering a roadmap is not work.

## Format

```
# ADR-NNNN: Title

**Status:** Proposed | Accepted | Superseded by ADR-XXXX
**Date:** YYYY-MM-DD

## Context
What forces are at play? What constraints apply?

## Decision
What was decided, stated plainly.

## Alternatives considered
What else was viable, and why it lost.

## Consequences
What becomes easier. What becomes harder. What risk is being accepted.
```

## Consequences

- **Easier:** onboarding, interviews, future reversals, and honest self-review.
- **Harder:** every decision now carries a small documentation cost.
- **Accepted risk:** ADRs can rot if not maintained. Mitigated by making an ADR a
  mandatory exit criterion for every phase.
