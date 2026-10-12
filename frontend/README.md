# Frontend

React 19 / TypeScript / Vite / React Konva annotation workbench. Chinese user interface;
all production data comes from the frozen Go API contract (`specs/contracts/api.md`, v1.0.0).

## Local development

```powershell
npm ci --cache D:\Project\cvte\runtime\npm-cache
npm run dev
```

Vite proxies `/api` and `/media` to `http://localhost:8080`. A unavailable backend is
reported in the interface. Company dataset configuration belongs to the backend;
the frontend does not accept credentials or fabricate company lesson responses.

## Verification and production output

```powershell
npm run typecheck
npm run lint
npm test
npm run build
```

`dist/` contains static production assets. The production web server must serve
these assets and proxy `/api/` and `/media/` to the Go service on the same origin.
No production mock server is included. The Leader owns shared Nginx/Compose configuration.

## Interaction and persistence

Create boxes in draw mode. Select mode permits movement and resizing; all boxes
use original image pixels. New gestures below two pixels are ignored; existing
positive boxes may be smaller. Delete removes a selected box. Ctrl/Cmd+Z and
Shift+Ctrl/Cmd+Z undo/redo; input focus suppresses these shortcuts.

Edits debounce for 500ms and save serially with captured snapshots and revision
checks. New edits during a save remain dirty until their own acknowledgement.
Frame, video, and batch navigation wait for the entire captured save queue to settle;
failed or conflicting saves block navigation. Export is also disabled while the current
editor is loading or unsaved, and whenever the selected batch contains browser drafts.
Browser drafts are scoped by stable frame ID. A stale draft requires explicit
replacement using the latest server revision, or discarding the draft. Conflicts
suspend autosave. A disposed editor cannot mutate drafts after a delayed response.
Conflict resolution permits one active request at a time and disables both choices
until the latest revision lookup finishes.

Editing a completed frame returns it to in-progress. Explicit completion is
required again, including zero-head frames. COCO ZIP export selects the current
video and extraction batch, and includes only server-confirmed completed frames.

Independent browser tests can use `annotation-image-bounds` for displayed image
coordinates; the outer `annotation-canvas` includes the surrounding letterbox.
`save-state`, `conflict-reload`, and the workflow selectors are exposed for acceptance.
