# P2 实施任务包

The human explicitly authorized P2 planning followed by autonomous startup on 2026-10-14. Read AGENTS.md, docs/plans/P2-PLAN.md, specs/p2/*.md and state.json. Historical P0/P1 start gates and deadlines do not apply. P1 remains partial. Do not operate production or tunnels. Keep private media/secrets/runtime/caches outside the repository.

## Ownership

- Leader owns canonical API/OpenAPI/types, repository/document organization, integration, deployment, audit and final report.
- P2-FRONT-001 owns frontend adaptive layout, progressive disclosure, and component behavior tests. Use a dedicated external worktree. Coordinate shared App/ExportButton changes with Leader. No API/DB behavior changes.
- P2-QC-001 owns backend/internal/exports quality-check implementation and tests, with only the minimal route registration change. No migrations or unrelated annotation writes.
- P2-TEST-001 owns tests/e2e/p2-responsive.spec.mjs and dedicated helpers/docs. Test mocked UI states separately from actual product routes. Cover viewport geometry, overlaps, resize continuity, long strings, failure states and keyboard reachability. Never weaken old tests.

## Evidence and execution

Write actual RED output before a behavior fix, then GREEN. Dependency/network failure is not a behavioral RED. Each root task tracks cumulative repairs and investigated causes, no reset by renaming. Only one heavy local test/build at a time: coordinate a slot with Leader. Do not push a failing test-only branch to main. Never claim independent GLM review, real model accuracy or GitHub CI without actual evidence.

Return changed paths, local commit reference, requirement mapping, commands/exit codes, limitations and evidence paths. Requested native worker model follows repository policy gpt-6.1-sol/high, actual model/effort null unless independently verified. The product DeepSeek credential is not permission to repurpose it for code review.
