# Agent Instructions

## Read before work

Read `CONSTITUTION.md`, `P0-PLAN.md`, `docs/decisions/0001-confirmed-choices.md`, `docs/decisions/0003-lean-agent-roles.md`, and the active wave's specification, task packet, and handoff. All fourteen choices and the lean role/model policy are confirmed. Decision 0003 supersedes decision 0002. P0 execution still requires the user's explicit start instruction. `NEXT-WAVES.md` is reference only and never authorizes P1/P2 work.

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

## Model allocation and context

- Treat decision 0003 as the canonical role/model policy. Leader uses `gpt-6.1-sol` with `xhigh`; Frontend, Backend, Test / Acceptance, and P1 AI Integration use `gpt-6.1-sol` with `high`.
- Activate Architecture Reviewer as needed with `gpt-6-astra` and `xhigh` for core architecture/contracts, coordinate/save consistency, and complex cross-module issues. Do not automatically escalate to `max/ultra`.
- P0 has five core responsibilities: Leader, Frontend, Backend, Test / Acceptance, and external GLM Reviewer/Experience. Leader owns product clarification; Test organizes verified evidence and drafts reports. Do not create separate Product, Evidence Assistant, or Luna roles under this policy. Roles are responsibilities, not permanently running processes.
- All native roles share the limit of one Leader plus three Workers. Specialist roles occupy Worker slots; they do not create extra capacity.
- Configure the main Leader session separately. Child model settings do not change the parent session. Never infer the actual parent model from self-identification.
- In the current native spawn interface, use `fork_turns="none"` for explicit model/effort overrides and provide a self-contained task packet. Full-history forks inherit the parent's model/effort and cannot satisfy an explicit override.
- Record `requestedModel`, `requestedReasoningEffort`, actual `provider/model/reasoningEffort`, `modelVerificationStatus`, `modelEvidenceRef`, and `contextMode`. Use `null` for unverifiable actual settings; never label requested values as verified actual values.
- If a selected model is unavailable, record a blocker, stop dependent tasks, and continue independent authorized work. Do not silently switch models or reset repair budgets.
- Determine and freeze the exact GLM model ID through the R0 company-key and tool/browser smoke checks. Use only provider-supported reasoning controls; record unsupported controls explicitly. GLM runs through external OpenCode, not native Codex spawn.
- Reload specifications, verified memory, and handoffs when changing roles or models. Do not assume context automatically transfers.
- Implement and smoke-check project-specific English agent definitions or spawn parameters during R0 after explicit P0 authorization. This documentation update does not activate runtime configuration.

## Scheduling and load ownership

- During R0, Leader clarifies product behavior, Test designs independent acceptance, and Architecture Reviewer audits the proposed architecture/contracts. Do not implement behavior against unfrozen contracts.
- During core development, normally allocate the three Worker slots to Frontend, Backend, and Test. Implementers own their behavior-focused tests; Test designs and executes independent integration/E2E acceptance early.
- Split backend work into self-contained video/import/extraction and annotation/persistence/export packets. Two Backend Workers may run only when dependencies and disjoint path ownership permit and a slot is free. Keep independent acceptance and the total three-Worker limit intact.
- Frontend prepares its image/Nginx configuration; Backend prepares its image/FFmpeg configuration. Leader assigns a single Worker to prepare each shared CI/Compose/monitoring file; ownership changes only after handoff. Leader reviews, integrates, and performs the actual release.
- Review completed contracts/modules through GLM in batches during R1-R5; experience available cloud flows early. R6 completes the actual synthetic core browser workflow and consolidates findings. R0 tool smoke checks never count as product acceptance.
- During delivery, Test prepares acceptance matrices, evidence indexes, and a report draft; Leader verifies all conclusions and issues the final report. Do not alter another actor's original evidence.
- Assess task dependencies, queues, blockers, and remaining budget each round. Record reassignment reasons; do not claim measured workload balance before execution.

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
