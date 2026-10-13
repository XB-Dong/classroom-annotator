-- +goose Up
ALTER TABLE jobs DROP CONSTRAINT jobs_kind_check, DROP CONSTRAINT jobs_check;
ALTER TABLE jobs ADD CONSTRAINT jobs_kind_check CHECK(kind IN ('extraction','import','preannotation'));
ALTER TABLE jobs ADD CONSTRAINT jobs_check CHECK((kind='extraction' AND video_id IS NOT NULL AND batch_id IS NOT NULL) OR (kind='import' AND batch_id IS NULL) OR (kind='preannotation' AND video_id IS NOT NULL AND batch_id IS NULL));
ALTER TABLE annotations DROP CONSTRAINT annotations_source_check;
ALTER TABLE annotations ADD CONSTRAINT annotations_source_check CHECK(source IN ('manual','ai','ai_corrected'));
ALTER TABLE annotations ADD COLUMN created_at TIMESTAMPTZ NOT NULL DEFAULT now();
ALTER TABLE annotation_events DROP CONSTRAINT annotation_events_action_check;
ALTER TABLE annotation_events ADD CONSTRAINT annotation_events_action_check CHECK(action IN ('save','complete','reopen','accept','reject','resolve','ai_publish','adjust'));
CREATE TABLE ai_requests (
 id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL REFERENCES workspaces(id), scope TEXT NOT NULL,
 request_key TEXT NOT NULL CHECK(length(request_key) BETWEEN 1 AND 128), fingerprint TEXT NOT NULL,
 job_id TEXT NOT NULL UNIQUE REFERENCES jobs(id), actor_id TEXT NOT NULL REFERENCES users(id),
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(), UNIQUE(workspace_id,scope,request_key)
);
CREATE TABLE ai_runs (
 id TEXT PRIMARY KEY, request_id TEXT NOT NULL REFERENCES ai_requests(id), job_id TEXT NOT NULL REFERENCES jobs(id),
 frame_id TEXT NOT NULL REFERENCES frames(id), base_revision BIGINT NOT NULL CHECK(base_revision>=0),
 input_sha256 TEXT NOT NULL, model_sha256 TEXT NOT NULL, model_id TEXT NOT NULL DEFAULT '',
 pipeline_version TEXT NOT NULL, risk_policy_version TEXT NOT NULL, options JSONB NOT NULL,
 status TEXT NOT NULL CHECK(status IN ('queued','running','completed','stale','failed','cancelled')),
 actor_id TEXT NOT NULL REFERENCES users(id), error_code TEXT NOT NULL DEFAULT '',
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(), finished_at TIMESTAMPTZ, UNIQUE(job_id,frame_id)
);
CREATE UNIQUE INDEX ai_runs_one_active_frame ON ai_runs(frame_id) WHERE status IN ('queued','running');
CREATE INDEX ai_runs_job ON ai_runs(job_id);
CREATE INDEX ai_runs_latest_frame ON ai_runs(frame_id,created_at DESC,id DESC);
ALTER TABLE frames ADD COLUMN active_ai_run_id TEXT REFERENCES ai_runs(id);
CREATE TABLE ai_candidates (
 id TEXT PRIMARY KEY, run_id TEXT NOT NULL REFERENCES ai_runs(id), frame_id TEXT NOT NULL REFERENCES frames(id),
 x DOUBLE PRECISION NOT NULL,y DOUBLE PRECISION NOT NULL,width DOUBLE PRECISION NOT NULL,height DOUBLE PRECISION NOT NULL,
 confidence DOUBLE PRECISION NOT NULL CHECK(confidence>=0 AND confidence<=1), risk_reasons JSONB NOT NULL,
 review_state TEXT NOT NULL DEFAULT 'pending' CHECK(review_state IN ('pending','accepted','adjusted','rejected')),
 annotation_id TEXT UNIQUE, reason_code TEXT NOT NULL DEFAULT '', decided_by TEXT REFERENCES users(id),decided_at TIMESTAMPTZ,
 UNIQUE(id,run_id,frame_id)
);
CREATE INDEX ai_candidates_run ON ai_candidates(run_id);
CREATE TABLE frame_review_issues (
 id TEXT PRIMARY KEY,run_id TEXT NOT NULL REFERENCES ai_runs(id),frame_id TEXT NOT NULL REFERENCES frames(id),
 reason_code TEXT NOT NULL CHECK(reason_code IN ('NO_DETECTIONS','AI_FAILED')),
 resolved BOOLEAN NOT NULL DEFAULT false,resolved_by TEXT REFERENCES users(id),resolved_at TIMESTAMPTZ,
 UNIQUE(run_id,reason_code)
);
CREATE TABLE annotation_provenance (
 annotation_id TEXT PRIMARY KEY,candidate_id TEXT NOT NULL UNIQUE,run_id TEXT NOT NULL,frame_id TEXT NOT NULL,
 deleted BOOLEAN NOT NULL DEFAULT false,created_at TIMESTAMPTZ NOT NULL DEFAULT now(),updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 FOREIGN KEY(candidate_id,run_id,frame_id) REFERENCES ai_candidates(id,run_id,frame_id)
);
-- +goose Down
DROP TABLE annotation_provenance,frame_review_issues,ai_candidates;
ALTER TABLE frames DROP COLUMN active_ai_run_id;
DROP TABLE ai_runs,ai_requests;
ALTER TABLE annotations DROP COLUMN created_at;
ALTER TABLE annotations DROP CONSTRAINT annotations_source_check;
ALTER TABLE annotations ADD CONSTRAINT annotations_source_check CHECK(source='manual');
ALTER TABLE annotation_events DROP CONSTRAINT annotation_events_action_check;
ALTER TABLE annotation_events ADD CONSTRAINT annotation_events_action_check CHECK(action IN ('save','complete','reopen','accept','reject','resolve'));
ALTER TABLE jobs DROP CONSTRAINT jobs_kind_check,DROP CONSTRAINT jobs_check;
ALTER TABLE jobs ADD CONSTRAINT jobs_kind_check CHECK(kind IN ('extraction','import'));
ALTER TABLE jobs ADD CONSTRAINT jobs_check CHECK((kind='extraction' AND video_id IS NOT NULL AND batch_id IS NOT NULL) OR (kind='import' AND batch_id IS NULL));
