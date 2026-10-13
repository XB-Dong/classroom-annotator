import type { Box } from '../../features/annotation/geometry';
export type FrameStatus = 'not_started' | 'in_progress' | 'completed';
export interface Annotation {
  id: string;
  bbox: Box;
  source: 'manual';
}
export interface Snapshot {
  frameId: string;
  revision: number;
  status: FrameStatus;
  annotations: Annotation[];
}
export interface Video {
  id: string;
  name: string;
  width: number;
  height: number;
  durationSeconds: number;
  status: string;
  createdAt: string;
  sourceKind?: 'public_sample' | 'mac' | 'upload' | 'dataset' | 'legacy';
  storageProvider?: 'local' | 'mac';
}
export interface PublicSample {
  sampleId: string;
  name: string;
  description: string;
  sourcePageUrl: string;
  creator: string;
  licenseName: string;
  licenseUrl: string;
  availability: 'ready' | 'unavailable';
  width: number;
  height: number;
  durationSeconds: number;
}
export interface MacSource {
  sourceId: string;
  mediaKind: 'video' | 'image';
  name: string;
  width: number;
  height: number;
  durationSeconds?: number;
  sha256: string;
  sizeBytes: number;
}
export interface MacSourcePage {
  contractVersion: '1.0.0';
  items: MacSource[];
  nextCursor: string | null;
}
export type SourceKind = 'public_sample' | 'mac' | 'upload' | 'dataset';
export interface Availability {
  available: boolean;
  reasonCode: string | null;
}
export interface MediaStorageStatus {
  providers: ({ provider: 'local' | 'mac' } & Availability)[];
  sources: ({ sourceKind: SourceKind } & Availability)[];
}
export interface Job {
  id: string;
  kind: 'import' | 'extraction';
  videoId?: string;
  batchId?: string;
  status: 'queued' | 'running' | 'cancelling' | 'completed' | 'failed' | 'cancelled';
  progress: number;
  error?: string;
  createdAt: string;
}
export interface Batch {
  id: string;
  videoId: string;
  jobId: string;
  createdAt: string;
  frameCount: number;
}
export interface Frame {
  id: string;
  videoId: string;
  batchId: string;
  timestampMs: number;
  width: number;
  height: number;
  imageUrl: string;
  status: FrameStatus;
  revision: number;
}
export interface Progress {
  total: number;
  notStarted: number;
  inProgress: number;
  completed: number;
}
export interface DatasetVideo {
  id: number;
  lesson_id?: number;
  catalog_file_id: number;
  channel: number;
  part_index?: number;
  tags?: Record<string, unknown>;
}
export interface DatasetLesson {
  id: number;
  course_name?: string;
  organization_name?: string;
  teacher?: string;
  date?: string;
  section_index?: number;
  name?: string | null;
  room?: string | null;
  start_time?: string | null;
  end_time?: string | null;
  videos: DatasetVideo[];
}
export interface DatasetPage {
  type: string;
  items: DatasetLesson[];
  total: number;
  stats?: Record<string, unknown>;
}
