# Agent Instructions

## Read before work

Read `CONSTITUTION.md`, the approved parts of `PLAN.md`, and the active wave's specification, task packet, and handoff. A discussion draft does not authorize product implementation by itself.

## Scope and language

- Keep code identifiers, comments, API fields, configuration keys, commit messages, and agent execution instructions in English.
- Write user-facing constitutions, plans, decisions, and wave reports in Chinese. Preserve the original language of raw evidence.
- Work only within the paths assigned by the Leader.
- Keep unrelated tools, credentials, company materials, videos, model weights, runtime data, and caches outside this repository.
- Do not print secrets or authenticated URLs. Do not commit private data.
- Use configuration for deployment-specific values and contracts for domain invariants.

## Coordination

- The Leader is the integration and shared-contract owner.
- Use independent worker contexts and isolated external worktrees for concurrent edits.
- Do not modify another worker's files, canonical contracts, or dependency manifests without coordination.
- Provide requirement IDs, observable test evidence, changed files, commit references, limitations, and next steps in each handoff.
- Treat memory notes as scoped evidence; stale or unverified claims must be checked.
- Do not claim that a GLM task ran unless its actual adapter/model and outcomes were recorded.
- Reuse GLM as an independent read-only reviewer or experience agent for GPT work. Do not substitute Codex self-review for cross-model review.
- The Leader must verify GLM findings and record accepted, rejected, or deferred decisions with reasons and evidence.
- Do not send private classroom media or credentials to an external review model. Use shareable code and synthetic demonstrations.

## Quality and delivery

- Use behavior-focused TDD where appropriate; preserve real Red and Green evidence.
- Run the checks appropriate to your change and report exit codes truthfully.
- Tests belong in every delivery wave. P2 is expanded verification, not first verification.
- Prefer cohesive capability modules and small explicit boundaries over speculative frameworks.
- Do not silently skip company API acceptance or replace live evidence with mocks.
- Log observable progress using timezone-aware timestamps and redact sensitive content.
- Follow the active wave's deadline and retry budget. Record blockers and continue independent work when possible.
- Only the Leader integrates/deploys and decides whether another round is needed.
- At wave completion or budget exhaustion, produce the wave report and stop for user review. Do not enter P1 or P2 from P0 automatically.
