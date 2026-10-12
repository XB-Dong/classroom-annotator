# Agent Instructions

## Read before work

Read `CONSTITUTION.md`, `P0-PLAN.md`, `docs/decisions/0001-confirmed-choices.md`, and the active wave's specification, task packet, and handoff. All fourteen choices are confirmed. P0 execution still requires the user's explicit start instruction. `NEXT-WAVES.md` is reference only and never authorizes P1/P2 work.

## Scope and language

- Keep code identifiers, comments, API fields, configuration keys, commit messages, and agent execution instructions in English.
- Write user-facing constitutions, plans, decisions, and wave reports in Chinese. Preserve the original language of raw evidence.
- Work only within the paths assigned by the Leader.
- Keep unrelated tools, credentials, company materials, videos, model weights, runtime data, and caches outside this repository.
- Do not print secrets or authenticated URLs. Do not commit private data.
- Use configuration for deployment-specific values and contracts for domain invariants.

## Coordination

- The Leader is the integration and shared-contract owner.
- Use native Codex subagents for development and OpenCode with GLM for independent review. Do not introduce a custom orchestration framework or silently substitute another client.
- Use independent worker contexts and isolated external worktrees for concurrent edits.
- Do not modify another worker's files, canonical contracts, or dependency manifests without coordination.
- Provide requirement IDs, observable test evidence, changed files, commit references, limitations, and next steps in each handoff.
- Treat memory notes as scoped evidence; stale or unverified claims must be checked.
- Do not claim that a GLM task ran unless its actual adapter/model and outcomes were recorded.
- Reuse GLM for both independent read-only code review and experience checks of GPT work. Do not substitute Codex self-review for cross-model review.
- Require both GLM code review and actual browser-experience checks using synthetic data. Reading E2E logs alone does not satisfy the browser-experience gate.
- The Leader must verify GLM findings and record accepted, rejected, or deferred decisions with reasons and evidence.
- Do not send private classroom media or credentials to an external review model. Use shareable code and synthetic demonstrations.

## Quality and delivery

- Use behavior-focused TDD where appropriate; preserve real Red and Green evidence.
- Run the checks appropriate to your change and report exit codes truthfully.
- Tests belong in every delivery wave. P2 is expanded verification, not first verification.
- Prefer cohesive capability modules and small explicit boundaries over speculative frameworks.
- Do not silently skip company API acceptance or replace live evidence with mocks.
- Report P0 core and intranet acceptance separately. Deliver `core_ready_pending_intranet` when all core gates pass but Mac live verification is pending; only both groups passing establish complete P0.
- Use React Konva, a single-user workflow, student-head-only labels, visible-part boxes for partial occlusion, independent extraction batches, and completed-frame-only COCO image exports including explicitly completed zero-head frames.
- Log observable progress using timezone-aware timestamps and redact sensitive content.
- Follow the active wave's deadline and retry budget. Record blockers and continue independent work when possible.
- P0 ends at the earlier of 7.5 hours after the explicit start instruction or 2026-10-13T08:30:00+08:00. Do not extend the deadline automatically. Successor tasks inherit the root task's repair counts.
- Only the Leader integrates/deploys and decides whether another round is needed.
- At the P0 core delivery gate, complete-wave gate, deadline, or budget exhaustion, produce the report and stop for user review. Do not enter P1 or P2 automatically. After P0 is explicitly started, the Leader may integrate verified task branches and deploy to the authorized demo host without per-round user approval.
