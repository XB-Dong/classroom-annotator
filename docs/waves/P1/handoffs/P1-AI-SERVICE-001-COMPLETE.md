# P1-AI-SERVICE-001 implementation handoff

Status: implementation complete for Leader review and integration; deployment, GLM, CI and Go/PG end-to-end acceptance remain separate gates.

- Root task: P1-AI-SERVICE-001. Requirements: P1-AI/COORD/DEGRADE and internal AI boundary in specs/p1/ai-review.md§1.
- Worktree: D:/Project/cvte/worktrees/p1-ai; branch p1/ai; base822eb26160958037b09542dd26623b833c50db45.
- Commits:81fec8b23223625d778ca176cd705e0bfa404347 (R0 model provenance/evidence), c57adf5ba5dafc97c9ff1260561281c09df25ee1 (coherent service/tests/Docker preparation). Apply the full range after review; the first manifest alone precedes the final pipeline version.
- Started2026-10-14T01:36:00+08:00; implementation committed02:01:57; maximum slice end02:18; hard wave deadline08:30.
- Requested gpt-6.1-sol/high; actual provider/model/reasoning remain null/unverified; context self-contained worker packet. No observed capacity failure or model fallback.
- Root repair counter unchanged; Leader state remains authoritative. Initial TDD failures and one corrected analytic fixture are preserved; no new review repair round was allocated by this worker.
- Owned paths only: ai-service/**, deploy/ai.Dockerfile, docs/waves/P1/handoffs/P1-AI-*, external runtime/p1-ai. No shared OpenAPI, Go, Compose, CI, database, cloud deployment, SSH, GLM or business media mutation.

## Delivered behavior

Pinned3,221,705B Nano SHA a5ece478fc918787258afccd0b313e54319f8f39aeb1ceca11f27dbd269e78bc is signature-checked before readiness. Source/class/model-license evidence and full MIT notice are in model-manifest.json/MODEL-LICENSE. R0 evidence remains the earlier handoff.

POST/v1/detections accepts multipart image/confidenceThreshold/nmsThreshold, Bearer internal authentication, safe X-Request-ID. Raw request body is bounded before parsing; image15MiB/16MP bounds, JPEG/PNG decoding, finite numeric thresholds, no disk spool and safe code/message/requestId failures are tested. Liveness remains200 when readiness is503. CPU ORT is sequential one-thread/no-spinning, one inference slot covers the entire pipeline. Busy429, unavailable503, invalid422, oversized413 and timeout504 are explicit. Timeout marks readiness503 and retains the slot until the actual worker completes; late output is discarded. Native termination is requested, never represented as proof that a thread was killed.

RGB0–1 letterbox uses actual rounded per-axis scale and integer padding. Raw[1,38,8400] is decoded as cx/cy/w/h plus34 independent scores; only Head=7/channel11 is retained. Invalid/nonfinite/degenerate candidates are removed, original xywh is clipped, and NMS is deterministic. clipped/tileBoundary booleans remain server owned.

Leader approved pipelineVersion `classroom-head-v1-fullframe-2x2-15`. Original max dimension≥1280 runs full frame plus four15%-overlapping tiles, at most5 invocations; smaller images run once. Tile coordinates map to original pixels, then global NMS. Full frame and tiles share one deadline/slot. Response includes contractVersion="1.0", modelId/modelSHA256, pipelineVersion, mode, image, detections, timingMs. Go must persist pipelineVersion and risk flags; it must not infer clipping from the final clipped bbox.

## Actual Red/Green evidence

External runtime: D:/Project/cvte/runtime/p1-ai. Python3.12.14, ORT1.23.2, numpy2.2.6, Pillow11.3.0, FastAPI0.119.0, uvicorn0.37.0, python-multipart0.0.20, pytest8.4.2, httpx0.28.1. Leader approved direct pins before install. requirements.lock pins32 actual resolved packages with hashes for published artifacts, including Windows-only markers. `pip check` exit0: no broken requirements.

| Actual command from ai-service | Exit | Evidence/result |
| --- | --- | --- |
| python -m pytest tests/test_coordinates.py tests/test_output_validation.py -q |1| coordinates-red.log:12 missing-feature failures |
| same narrow command, first implementation |1| coordinates-green.log:11 passed/1 failed; incorrect analytic fixture center190 corrected to220, no implementation change |
| python -m pytest tests/test_http.py -q before HTTP implementation |1| http-red.log:7 missing-feature failures |
| python -m pytest tests/test_tiling.py -q before tiling implementation |1| tiling-red.log:4 missing-feature failures |
| combined coordinates/output/tiling/HTTP |1| service-first-green.log:25 passed/1 failure; timeout readiness expected503 but200. State correction was then implemented |
| python -m pytest tests/test_http.py::test_image_part_limit_excludes_multipart_overhead -q |1| image-part-red.log:expected413 but422; explicit bounded image-part413 then implemented |
| python -m pytest tests -q -o cache_dir=D:/Project/cvte/runtime/p1-ai/pytest-cache with actual model/sample env |0| service-green.log:30 passed; service-final-green.log:30 passed in4.50s after recording HTTP wall time |
| git diff --cached --check |0| no whitespace errors before service commit |

Actual tests include rectangular/portrait rounding, clipping/head filtering/NMS, original tile mapping, reversed score ordering/global duplicate suppression, tile boundaries, auth, readiness, concurrency429, timeout retained slot, Content-Length and chunked body limits, image-part limits, PNG, multipart no-disk spool above1MiB, safe failure and slot reuse. Dependency import produces one upstream Starlette0.48/anyio4.15 DeprecationWarning about BlockingPortal; it is preserved rather than hidden.

## Real model results and limits

Configured AI_TEST_MODEL_PATH and AI_TEST_SAMPLE_DIRECTORY use the exact external artifact and restricted local test sample directory. No real image path or bytes are committed. Eight PNG frames are converted to JPEG in memory and sent through actual FastAPI TestClient+ORT; original PNG is also sent through the same HTTP endpoint. The test is skipped and counts not_run if paths are absent.

Latest real-http-results.json contains original dimensions, input hashes, all actual candidates/risk flags, model/pipeline identity and timing. Counts:10,14,9,13,13,10,13,12. Internal preprocessing/inference/decode pipeline timing284.9–292.7ms; actual in-process HTTP TestClient wall time308.8–316.9ms, excluding JPEG encoding and network/Go/PG. Entire pytest-process peak working set207.4MiB, not isolated service RSS or cloud resource measurement. Initial tiling probe used per-tile JPEG re-encoding; production crops directly in memory and therefore has slightly different counts/times. Do not mix those two records.

Sample01 tiled overlay was actually viewed by worker and Leader:10 plausible heads, at least15 clear visible omissions, no obvious non-head false positive among its10. Precision/recall, teacher/student separation and time savings remain unevaluated; no candidate may auto-complete a frame. Model quality is partial and requires manual review/supplementation.

## Integration and remaining gates

Run from ai-service with `PYTHONPATH=src`, AI_MODEL_PATH set to the external read-only model and AI_INTERNAL_TOKEN supplied only in backend/service deployment secrets. Start one worker: `uvicorn classroom_ai.app:create_app --factory --host 0.0.0.0 --port 8090 --workers 1 --no-access-log`. Default real tests require AI_TEST_MODEL_PATH, AI_TEST_SAMPLE_DIRECTORY and optional AI_TEST_EVIDENCE_PATH outside Git. Follow README and source CLI arrays for exact runnable invocation.

Dockerfile is prepared only. Official DockerHub python3.12.14-slim-bookworm tag has amd64 and arm64 images, verified by API; Docker build, Linux CI dependency install, arm64 inference and actual Mac execution are not_run. Leader must build immutable artifacts, mount model read-only, restrict8090 to Go, apply CPU/memory controls and integrate readiness/monitoring. No Windows Docker/autostart or persistent service was started.

All named Local\CVTE_P1_BUILD acquisitions used≤120s wait and finally release. Tests/installation ran sequentially. No owned Python process/server/port/lock remains after handoff. Initial generated ignored caches were moved intact into external cache-archive-0201 after automatic policy rejected recursive deletion; subsequent tests disable bytecode and use external pytest cache. Source worktree is clean after commits.

Next owner: Leader review/integration, then Backend Go client and independent Test acceptance. Required remaining gates: actual Go→Python→PG publication/review/export, stale/cancel/job behavior, GLM code review, independent public browser experience, Linux image/CI, deployment/resource observation. This worker does not claim those gates passed.
