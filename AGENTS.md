# Agent Instructions

## Current authorization

The human explicitly authorized P2 planning then autonomous startup on 2026-10-14. Read `CONSTITUTION.md`, `docs/plans/P2-PLAN.md`, `specs/p2/*.md`, `docs/waves/P2/TASK-PACKET.md`, and `docs/waves/P2/state.json`. Historical P0/P1 preparation gates, expired deadlines and no-P2 instructions do not override this authorization. P1 remains `partial_closed_fast_path`.

## Scope and language

- Code identifiers, comments, API/config keys, commit messages and agent execution instructions are English. User-facing plans, decisions and reports are Chinese. Preserve raw evidence language.
- Keep media, model weights, credentials, databases, raw logs, dependencies, build caches and worktrees outside the canonical source repository. Only sanitized evidence indexes belong in Git.
- Never print credentials, authenticated URLs or private media. Use synthetic/public demonstrations. No production migration, tunnel restoration, bulk private-media processing or scope expansion.

## Architecture and ownership

- Preserve React/TypeScript/Konva, Go/Gin/PostgreSQL17, Python/FastAPI/ONNX CPU and same-host media. Go is the sole business entry point. No speculative orchestration framework, new ORM/vector store/queue or distributed platform.
- Preserve original-pixel xywh coordinates, finite in-bounds geometry, revision CAS, candidate/formal-label separation, human review and completed-frame COCO export including explicit zero-head frames.
- AI/help failure must not block manual annotation. Historical remote media must not be silently remapped to local files.
- Leader freezes shared API/contracts/types/dependencies, integrates, deploys and signs reports. Use native Codex development subagents and isolated external worktrees for parallel edits, at most Leader plus three workers. Assigned files have one owner; coordinate before shared edits.
- Frontend/Backend implement behavior-focused tests. Test owns independent acceptance and evidence. Roles are responsibilities, not permanently running agents.
- Use cohesive capability modules and explicit boundaries. Inspect large components before splitting; avoid mechanical abstractions or broad rewrites.

## Models and independent review

- Preserve decision0003 role policy: requested workers `gpt-6.1-sol/high`, architecture reviewer `gpt-6-astra/xhigh` when required. Configure parent separately, do not infer actual routing from model self-identification.
- Record requested versus actual provider/model/effort, context mode and evidence. Unverified actual values are null. Use `fork_turns="none"` for explicit child overrides and provide a self-contained task packet.
- P1 fallback model authorizations are historical and do not carry into P2. If selected tooling/model is unavailable, record the blocker and continue independent authorized work, without silently substituting.
- Independent GLM code review and actual synthetic/public browser experience remain separate gates using external OpenCode. Code reading or E2E logs are not browser experience, and native self-review is not a substitute. Leader records accepted/rejected/deferred findings with evidence.
- Product DeepSeek credentials are authorized for document help only, not independent code review. Never send private media or secrets to review models.

## Verification, audit and stopping

- Define requirements/specs/examples/failure behavior before implementation. Preserve real Red and Green for behavior changes; dependency failures do not count as behavioral Red.
- Run appropriate checks and retain commands/timestamps/exit codes/exact local commits. Label mocked, actual API, real model, local, CI and deployed evidence separately. Do not fabricate accuracy, recall, speedups, deployment, browser actions or full coverage.
- One heavy local process at a time. Reassess a root cause after two failed repairs; preserve cumulative history. No endless retries, test weakening, task-renaming budget resets or unnecessary CI pushes.
- Each wave keeps state, timestamped events, handoffs, evidence index and report. Preserve historical failures and source versions. Documentation privacy redaction must record hashes/retained originals externally and must not erase technical outcomes.
- P2 has no user-specified new deadline. Do not reuse P0/P1 clocks. Completed requires all current mandatory gates, including independent review; missing gates mean partial with concrete limitations.
- Deliver reviewable local changes, current evidence and reports, then stop at the wave boundary. Do not enter the next wave automatically.

Historical instructions are retained in `docs/agents/ARCHIVED-INSTRUCTIONS-20261014.md` for audit, not as current startup authority.
