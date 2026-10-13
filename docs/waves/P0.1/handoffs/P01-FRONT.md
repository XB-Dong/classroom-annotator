# P01-FRONT-001 handoff

- Status: implemented and locally verified; pending Leader integration, independent review, and deployed E2E acceptance.
- Base: `07eea66203680a7421b79c5d3ed8a1d4de968822`.
- Implementation commit: `ab66dead4dcb39477312b5ca15493332d156fc28` (`p01/frontend`).
- Requested model/reasoning: `gpt-6-sol` / `high`, approved temporary fallback after `gpt-6.1-sol` capacity errors. Actual provider/model/reasoning: `null`/`null`/`null`; verification status: `unverified`, evidence ref: `null`; context mode: `none`.
- Scope and requirements: P01-FRONT-001; P01-SRC-001/002, P01-SAMPLE-001/002 (frontend portions), P01-DATASET-001 (frontend portion).

## Delivered behavior

Four source tabs remain visible and keyboard reachable: `公共示例` is the default, followed by `Mac素材`, `本地上传`, and `公司接口`. The sample panel displays source and license links, registers only `availability=ready` samples through the official import endpoint, then uses the common video/editor flow. The Mac panel lists videos and images, registers only videos with opaque `sourceId`, supports cursor pagination, and marks zero probe dimensions/duration as unknown. Upload keeps the existing multipart `file` contract. Dataset keeps P0 pagination and channel 4 filtering without coupling its visibility to the Mac source status. Source status explains offline entrances and offers retry where a disabled upload needs reconnection. Successful Mac directory refresh can recover a stale offline status.

`VideoLibrary` labels source and storage and hides explicitly tagged `legacy` videos until `显示历史素材` is selected, while preserving an already selected legacy video and keeping newly imported sources visible. A failed frame image has a retry action; a previously loaded image remains on screen after a later load failure so editing and cloud annotation saves remain available. Thumbnail failure displays an explicit message. The mixed-provider export 422 error asks the user to split the selection by storage location.

## Changed paths

`frontend/src/app/{App.tsx,App.test.tsx,styles.css}`; `frontend/src/shared/api/{types.ts,client.ts}`; `frontend/src/features/videos/{ImportPanel.tsx,ImportPanel.test.tsx,VideoLibrary.tsx,VideoLibrary.test.tsx,PublicSamplePanel.tsx,MacSourcePanel.tsx,Sources.test.tsx}`; `frontend/src/features/annotation/{Canvas.tsx,Canvas.test.tsx,FramePicker.tsx,FramePicker.test.tsx}`. `JobsPanel.tsx` remains unchanged because the existing job status and error handling already covers the new import jobs.

## Verification

Raw evidence is outside Git at `D:/Project/cvte/runtime/p01-frontend-evidence/`.

| Check | Evidence | Exit/result |
| --- | --- | --- |
| Source behavior Red | `red-sources-thread.txt` | 1; 4 expected feature failures |
| Canvas image behavior Red | `red-canvas.txt` | 1; 2 expected feature failures |
| Legacy library Red | `red-library.txt` | 1; expected legacy visibility failure |
| Thumbnail Red | `red-framepicker.txt` | 1; expected missing fallback |
| Upload recovery Red | `red-upload-retry.txt` | 1; expected missing retry action |
| Focused source Green | `green-upload-retry.txt` | 0; 6/6 |
| Full frontend suite | `all-tests-final.txt` | 0; 43/43 across 10 files, one thread, no isolate |
| Production build / TypeScript | `build-final-2.txt` | 0 |
| ESLint | `lint-final-2.txt` | 0 |
| Staged diff check | `git diff --cached --check` | 0 |

Initial default fork workers timed out before tests ran (`red-sources.txt`); the recorded Red evidence uses a single thread. A heavily loaded earlier isolated broad run had short asynchronous test wait failures (`all-tests.txt`); a focused App rerun passed 8/8 (`focused-app-2.txt`), and the final full suite passed after setting an explicit wait for the existing lazy-loaded editor. No test or UI claim relies on the worker startup failure.

## Handoff selectors and remaining gates

E2E selectors: `getByRole('tab', {name:'公共示例'|'Mac素材'|'本地上传'|'公司接口'})`; default public tab has `aria-selected=true`. Public sample action: `开始标注`. Mac video action: `登记素材`. Upload selectors: `data-testid=upload-file`, `data-testid=upload-submit`. Dataset action: `加载公司课节`. Legacy reveal: `显示历史素材`; image retry: `重试加载原图`.

Backend contracts, a running Mac bridge, deployed public sample bytes, live PostgreSQL, the real company API, and cloud/browser E2E were unavailable in this isolated frontend worktree. Leader/Test should verify the integrated endpoints, actual image recovery, and cross-provider 422 in the synthetic browser workflow. The source contract uses `availability: 'ready'|'unavailable'`, and Mac `sourceId` is treated as opaque; do not substitute the old Mac `id` or company catalog ID.
