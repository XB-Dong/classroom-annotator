# P1实施任务包

状态：running，用户已于2026-10-14授权启动P1计划0.2.0。需求/规范用中文审阅；下面的执行指令按英文交接。

> Load P1-PLAN.md, specs/p1/*.md, this packet, AGENTS.md, the assigned role and verified memory. P1 implementation is explicitly authorized by the user; execute within the approved boundaries. Keep all runtime/media/models/secrets/caches/worktrees outside code. Never use a tunnel. Stop at 2026-10-14T08:30:00+08:00. A task handoff is not proof of release acceptance.

## Shared ownership and task steps

Leader owns OpenAPI, domain/shared DTO changes, migration numbers, dependency locks, Compose/CI/monitoring integration and release. Workers request shared changes before editing them. Every task records baseSha/rootTaskId/allowedPaths, timestamps, requested/actual model, commands/exit codes, repair counts, changed files and limitations.

For each behavior change:

- [ ] Write a requirement-driven failing test in the assigned test path.
- [ ] Run the narrow test and preserve actual output/exit code before implementation; unavailable dependencies do not count as RED.
- [ ] Implement the smallest cohesive change in the assigned module.
- [ ] Run the narrow test, relevant module checks and contract checks; retain GREEN evidence.
- [ ] Refactor only as needed for module responsibility and rerun affected checks.
- [ ] Commit English message, produce a handoff and request Leader review.

Pure styling uses screenshots and interaction checks rather than implementation-mirroring unit tests. No worker deploys, migrates production data, edits another worker's files or changes frozen contracts.

## P1-BASE-001 / Leader + Backend

Requirements: P1-BASE/LOCAL/OPS. Start from reviewed ab21708 / tested product9fafbea; re-read P0.1 closeout. Map caller dependencies before removing media bridge. Preserve active data and historical evidence.

Files: backend/internal/platform/{config,store}, backend/internal/httpapi/{router,source_routes,videos,jobs_dataset}, backend/internal/media, backend/internal/extraction, backend/internal/samples, backend/internal/exports; backend/cmd/{server,import-sqlite,migrate,seed-samples}; new local source capability under backend/internal/sources. Retire remote-only files listed in specs/p1/deployment.md with caller-aware tests. Shared migration/CI changes belong to Leader.

RED: upload/provider defaults require Mac in current source mode; prove local path succeeds without Bridge. Unknown retired mac reference must never read local file. GREEN: actual upload/ffprobe/JPEG/export, known-schema synthetic company adapter, cache/import idempotence, no tunnel configuration in active Compose.

Checks: `cd backend` then `go test ./internal/httpapi ./internal/media ./internal/extraction ./internal/samples ./internal/exports ./internal/sources`; Go package sources is created by this task, not currently present. Final `go vet ./...` and `go test ./...` use isolated PG via existing P01_TEST_DATABASE_URL; `go test -race ./...` runs Linux CI.

## P1-AUTH-CAS-001 / Backend

Requirements: P1-AUTH/CAS/GATE. Create backend/internal/auth/{password,session,http,middleware} and backend/cmd/bootstrap-user; extend annotation repository/http only for server-owned actor/review integration. Leader allocates new SQL migrations under backend/migrations/ and DTO/OpenAPI updates.

RED: anonymous media request, role spoofing, duplicate username, bad invite/CSRF, expired/logged-out session, two same-revision writes and cross-user draft fixtures. GREEN: CAS winner once and atomic review/annotation/audit, correct cookie and per-resource authorization.

Checks: `go test ./internal/auth ./internal/annotations ./internal/exports` with real PG integration; account/cookie/Origin/role contracts in tests/integration/auth.spec.mjs and concurrency scenarios in tests/integration/collaboration.spec.mjs assigned Test, not simultaneously edited by Backend.

## P1-AI-SERVICE-001 / AI Integration

Requirements: P1-AI/COORD/DEGRADE. First bounded R0 real model source/license/hash/head-class probe. No broad model search or training.

Create ai-service/pyproject.toml and locked dependency file; ai-service/src/classroom_ai/{app,settings,model,preprocess,postprocess,schemas}.py; ai-service/tests/{test_coordinates,test_output_validation,test_http,test_real_model}.py. Model bytes stay external. Leader reviews dependency versions and internal contract; deploy/ai.Dockerfile is prepared by this worker, integrated by Leader.

RED: analytic non-square letterbox round-trip, invalid output/size, class filtering/NMS, bad internal auth, model-not-ready/busy behavior. GREEN: true known hash ONNX + real JPEG, safe xywh output, one inference slot, no DB/file/url access. `pytest ai-service/tests -q` with declared package/fixture paths. Real-model evidence is separate from fake-output unit tests; test skipping counts not_run.

## P1-AI-REVIEW-001 / Backend after AI service handoff

Requirements: P1-REVIEW/STATS/AIRACE/GATE. Create backend/internal/preannotation/{client,service,repository,http,risk,metrics}; integrate bounded jobs, domain DTO, annotation rules and COCO eligibility. Leader allocates AI schema migrations.

RED: pending leak to export, high-risk bulk-accept, repeated runs denominator, deleting accepted box counts incorrectly, AI publication on stale revision, dual enqueue/cancel/restart races. GREEN: exact independent candidate decision/metric counts and true Go→Python→PG path; no SQL transaction spans inference.

Checks: `go test ./internal/preannotation ./internal/annotations ./internal/exports ./internal/jobs`; end-to-end AI fixtures/results under Test-owned integration files.

## P1-FRONT-001 / Frontend

Requirements: P1-UI/AUTH/REVIEW/STATS/CAS. Modify frontend/src/app/{App,App.test}, features/videos/{ImportPanel,VideoLibrary,PublicSamplePanel}, features/annotation/{Editor,Canvas,FramePicker,saveController,drafts}, features/export/ExportButton. Retire MacSourcePanel from active UI. Create features/auth/{AuthPage,AuthProvider}, features/review/{ReviewPanel,RiskFilter,QualitySummary}, shared/ui/theme and narrowly scoped styles. Shared API types/client edits coordinated with Leader.

Behavior RED: account switch draft/cache leak, conflict auto-retry overwrite, candidate clicks select specific box, bulk action skips risk, image failure doesn't block manual edit, pending completion blocked. Styling uses actual browser snapshots at three viewport sizes.

Checks from frontend: `npm run typecheck`, `npm run lint`, `npm test -- --run --maxWorkers=1 --no-file-parallelism`, `npm run build`. Reuse current test pool settings if command syntax requires it; do not weaken assertions to make checks pass.

## P1-HELP-001 / Backend + Frontend, small independent capability

Requirement: P1-HELP-001; follow specs/p1/document-help.md. Product document-grounded QA is different from ONNX detection and external OpenCode review. Send the complete versioned manual plus a single operation question directly from Go to the configured GLM HTTP endpoint. No embeddings, retrieval index, vector store, LangChain or LlamaIndex; no business data, model tools or client-side Key.

Backend owns backend/internal/help/{document,provider,service,http}.go and behavior tests; Frontend owns frontend/src/features/help/{HelpPanel,DocumentView}.tsx and tests. Leader owns OpenAPI/config/dependency integration; docs/product/ANNOTATOR-GUIDE.zh.md is the sole public manual, synchronized at build/release rather than separate conflicting copies.

- [ ] Backend RED: fake provider independently asserts full manual content/hash, auth/CSRF, timeout, bad/unknown citations, no credentials/data leak.
- [ ] Backend GREEN: bounded2-slot/15s client, session/Origin checks, validated answer/reference fields and safe static document fallback.
- [ ] Frontend behavior checks: logged-in question/reference navigation, anonymous registration manual, failure fallback, authGeneration discards late reply, closes without blocking canvas/save.
- [ ] Real provider check: three platform questions plus an out-of-document question, record actual endpoint/model/documentSHA/answer reference/time; fake response cannot satisfy real API gate.
- [ ] Module checks: `go test ./internal/help` from backend after package creation, normal frontend checks, Test-owned tests/integration/help.spec.mjs and browser smoke after true registration/login.

Each ownership slice is max45min and shares parent P1-HELP-001 repair count. It must not take over unrelated authentication or annotation files; coordinate shared client use. No product API call or feature worker is started by this preparation packet.

## P1-TEST-001 / Test, independent

Create tests/integration/{auth,collaboration,ai-review,help}.spec.mjs; tests/e2e/{p1-workflow,p1-conflict}.spec.mjs and helpers only as needed; scripts/p1-smoke and low-concurrency entrypoints with Windows/Linux invocation documented. Update tests/README after handoff. Leader alone adjusts common verify scripts/workflow shared files.

Use exact acceptance.md matrix. Tests assert independent final DB/COCO state, actual browser downloads and winner/loser outcomes; do not mirror a model decoder and call it independent coordinate verification. Real AI and fake-coordinate tests are labeled separately. Keep 5 contexts/3 accounts, 20 fresh barrier CAS rounds and 10-minute mixed workload bounded. Never test destructive scenarios on production workspace.

The old verify.sh still contains media-agent/mac checks; replacing retired gates requires mapping each non-remote business invariant to new P1 checks. Keep PG/import/CAS/FFmpeg/COCO tests, do not simply delete failing checks. All new Python and auth/review gates join common CI; tested release artifacts only after full pass.

## P1-RELEASE-001 / Leader

Prepare immutable multiarch Docker images, model manifest, whole-stack config, exact SHA/version checks, separate migrator/app credentials, bootstrap and consistent restore test. Apply authorized sample-only cloud boundary, retire dedicated tunnel infrastructure, add IP TLS and validate authenticated media routes. Run 30-minute cloud resource/health/monitoring observation and actual 2–4-minute public-sample recording. Publish only verified candidate, attach any created PR. No automatic SQLite rollback after PG writes.

External GLM reads shareable code then uses a public-only isolated workspace/instance with synthetic disposable accounts. It cannot see authorized classroom media, company keys or production credentials. Record actual model, prompts/tool calls/errors, screenshot/download and timed exit; Leader issues accepted/rejected/deferred disposition. Browser actions must actually occur.

The isolation is an independent public-only instance with its own QA database and media volume, not the shared production workspace. Serve it through the HTTPS entry before production cutover, with no tunnel or extra management ports. Keep actual experience before07:35, verified full-CI artifacts before07:15, and production ready before07:45; missed gates mean partial and no rushed cutover.

## Bounded slices under the existing root tasks

Do not treat broad capability packets as a single unlimited job. Leader dispatches these max45-minute slices with handoffs; repairs accumulate on their parent rootTaskId, not the slice name:

| Parent rootTaskId | Ordered slices |
| --- | --- |
| P1-BASE-001 | A: caller map + local upload/directory behavior tests; B: retire remote callers/active configs; C: fixed-schema4 tool/import/backfill acceptance |
| P1-AUTH-CAS-001 | A: users/password/session/register tests; B: all API/media authorization/CSRF/Origin; C: actor/revision/draft-generation/cookie contract integration |
| P1-AI-SERVICE-001 | A: true artifact/license/class/hash probe + preprocess RED; B: decode/NMS/coordinates GREEN; C: bounded HTTP/service real-model checks |
| P1-AI-REVIEW-001 | A: schema/run/idempotency/publication stale; B: provenance/decision/tombstone and risk completion rules; C: stats/COCO/AI-failure/batch races |
| P1-FRONT-001 | A: theme/layout/auth; B: candidate/risk workflow; C: conflict/account-generation/actual viewport checks |
| P1-TEST-001 | A: independent contracts/concurrency tests; B: browser workflow/degrade; C: low-concurrency + CI/evidence/recording checks |
| P1-HELP-001 | A: controlled full manual/provider and contract tests; B: guide drawer/real API/reference/failure checks |

Leader may split ownership only after a clean handoff and within the3-worker limit. Test designs cases early, takes a seat after AI service handoff; completed frontend/AI work frees a slot for bounded backend assistance. No more workers, repeated task-name resets, or background builds to bypass limits.

## Closeout

- [ ] Every required matrix row has raw evidence, exact commit/environment and truthful status.
- [ ] No worker process/test port remains running on Windows; leave normal user applications untouched.
- [ ] Source/CI/image/deployment/model versions correspond; record unverified arm64/Mac states explicitly.
- [ ] Final report contains measured outcomes, deferred items and defects; stop at deadline.

P1 proposes at most8 repairs across new tasks, max2 each; old P0.1 rootTask counters remain6/6. Approved P1 restart must explicitly assign any inherited defect budget in state rather than erasing it. Changing task owner/model does not reset limits.
