import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, expect, it, vi } from 'vitest';
import { ImportPanel } from './ImportPanel';
import { ExportButton } from '../export/ExportButton';
import { MacSourcePanel } from './MacSourcePanel';

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});
const sample = {
  sampleId: 'public-1',
  name: '公开课堂',
  description: '真实公开视频',
  sourcePageUrl: 'https://example.org/source',
  creator: 'Creator',
  licenseName: 'CC0',
  licenseUrl: 'https://example.org/license',
  availability: 'ready',
  width: 320,
  height: 240,
  durationSeconds: 42,
};
const status = {
  providers: [
    { provider: 'local', available: true, reasonCode: null },
    { provider: 'mac', available: false, reasonCode: 'MEDIA_SOURCE_UNAVAILABLE' },
  ],
  sources: [
    { sourceKind: 'public_sample', available: true, reasonCode: null },
    { sourceKind: 'mac', available: false, reasonCode: 'MEDIA_SOURCE_UNAVAILABLE' },
    { sourceKind: 'upload', available: false, reasonCode: 'MEDIA_SOURCE_UNAVAILABLE' },
    { sourceKind: 'dataset', available: true, reasonCode: null },
  ],
};
function api(values: Record<string, unknown>) {
  const fetchMock = vi.fn(async (url: string, options?: RequestInit) => {
    const value = values[`${options?.method ?? 'GET'} ${url}`];
    return new Response(JSON.stringify(value ?? {}), {
      headers: { 'Content-Type': 'application/json' },
    });
  });
  vi.stubGlobal('fetch', fetchMock);
  return fetchMock;
}
it('defaults to attributed public samples and retires the remote entrance without faking a local directory', async () => {
  const fetchMock = api({
    'GET /api/v1/samples': { items: [sample] },
    'GET /api/v1/media-storage/status': status,
    'GET /api/v1/sources/local': { items: [] },
    'POST /api/v1/samples/imports': {
      id: 'public-video',
      sourceKind: 'public_sample',
      storageProvider: 'local',
    },
  });
  const onVideo = vi.fn();
  render(<ImportPanel onVideo={onVideo} onJob={() => {}} />);
  for (const label of ['公共示例', '本机素材', '本地上传', '公司接口'])
    expect(screen.getByRole('tab', { name: label })).toBeTruthy();
  expect(screen.queryByRole('tab', { name: 'Mac素材' })).toBeNull();
  expect(screen.getByRole('tab', { name: '公共示例' }).getAttribute('aria-selected')).toBe('true');
  await screen.findByText('公开课堂');
  expect(screen.getByRole('link', { name: '素材出处' }).getAttribute('href')).toBe(
    sample.sourcePageUrl,
  );
  expect(screen.getByRole('link', { name: 'CC0' }).getAttribute('href')).toBe(sample.licenseUrl);
  fireEvent.click(screen.getByText('开始标注'));
  await waitFor(() => expect(onVideo).toHaveBeenCalledOnce());
  const call = fetchMock.mock.calls.find(([url]) => url.endsWith('/samples/imports'));
  expect(JSON.parse(call?.[1]?.body as string)).toEqual({ sampleId: 'public-1' });
  fireEvent.click(screen.getByRole('tab', { name: '本机素材' }));
  await screen.findByText('目录中暂无授权视频。');
  expect(fetchMock.mock.calls.some(([url]) => url.includes('/media-sources/mac'))).toBe(false);
  fireEvent.click(screen.getByRole('tab', { name: '公司接口' }));
  expect((screen.getByText('加载公司课节') as HTMLButtonElement).disabled).toBe(false);
});
it('lists Mac images as information and imports only videos using opaque sourceId with cursor pagination', async () => {
  const online = {
    ...status,
    sources: status.sources.map((s) => ({ ...s, available: true, reasonCode: null })),
  };
  const fetchMock = api({
    'GET /api/v1/samples': { items: [] },
    'GET /api/v1/media-storage/status': online,
    'GET /api/v1/media-sources/mac': {
      contractVersion: '1.0.0',
      items: [
        {
          sourceId: 'opaque-video',
          mediaKind: 'video',
          name: '授权视频',
          width: 100,
          height: 80,
          durationSeconds: 20,
          sha256: 'a'.repeat(64),
          sizeBytes: 100,
        },
        {
          sourceId: 'opaque-image',
          mediaKind: 'image',
          name: '已抽帧图片',
          width: 100,
          height: 80,
          sha256: 'b'.repeat(64),
          sizeBytes: 20,
        },
      ],
      nextCursor: 'cursor+/=',
    },
    'GET /api/v1/media-sources/mac?cursor=cursor%2B%2F%3D': {
      contractVersion: '1.0.0',
      items: [],
      nextCursor: null,
    },
    'POST /api/v1/media-sources/mac/imports': { id: 'job', kind: 'import', status: 'queued' },
  });
  const onJob = vi.fn();
  render(<MacSourcePanel available onJob={onJob} />);
  await screen.findByText('授权视频');
  expect(screen.getByText('已抽帧图片')).toBeTruthy();
  expect(screen.getAllByText('登记素材')).toHaveLength(1);
  fireEvent.click(screen.getByText('登记素材'));
  await waitFor(() => expect(onJob).toHaveBeenCalledOnce());
  expect(
    JSON.parse(
      fetchMock.mock.calls.find(([url]) => url.endsWith('/mac/imports'))?.[1]?.body as string,
    ),
  ).toEqual({ sourceId: 'opaque-video' });
  fireEvent.click(screen.getByText('加载更多素材'));
  await waitFor(() =>
    expect(fetchMock.mock.calls.some(([url]) => url.endsWith('?cursor=cursor%2B%2F%3D'))).toBe(
      true,
    ),
  );
});
it('keeps unavailable samples visible and prevents their registration', async () => {
  api({
    'GET /api/v1/samples': { items: [{ ...sample, availability: 'unavailable' }] },
    'GET /api/v1/media-storage/status': status,
  });
  render(<ImportPanel onVideo={() => {}} onJob={() => {}} />);
  await screen.findByText('公开课堂');
  expect((screen.getByText('开始标注') as HTMLButtonElement).disabled).toBe(true);
  expect(screen.getByText(/示例尚未缓存或暂不可用/)).toBeTruthy();
});
it('lets a recovered Mac catalog register a video after an offline status response', async () => {
  api({
    'GET /api/v1/samples': { items: [] },
    'GET /api/v1/media-storage/status': status,
    'GET /api/v1/media-sources/mac': {
      contractVersion: '1.0.0',
      items: [
        {
          sourceId: 'recovered',
          mediaKind: 'video',
          name: '恢复的视频',
          width: 0,
          height: 0,
          durationSeconds: 0,
          sha256: 'a'.repeat(64),
          sizeBytes: 1,
        },
      ],
      nextCursor: null,
    },
    'POST /api/v1/media-sources/mac/imports': { id: 'job', kind: 'import', status: 'queued' },
  });
  const onJob = vi.fn();
  render(<MacSourcePanel available={false} reasonCode="MEDIA_SOURCE_UNAVAILABLE" onJob={onJob} />);
  await screen.findByText(/Mac 媒体服务暂时离线/);
  fireEvent.click(screen.getByText('刷新素材目录'));
  await screen.findByText('恢复的视频');
  expect(screen.getByText(/尺寸未知.*时长未知/)).toBeTruthy();
  fireEvent.click(screen.getByText('登记素材'));
  await waitFor(() => expect(onJob).toHaveBeenCalledOnce());
});
it('rechecks upload availability after Mac recovers without hiding the upload entrance', async () => {
  let statusRequests = 0;
  vi.stubGlobal(
    'fetch',
    vi.fn(async (url: string) => {
      if (url.endsWith('/media-storage/status')) {
        statusRequests += 1;
        const sources = status.sources.map((source) =>
          source.sourceKind === 'upload'
            ? {
                ...source,
                available: statusRequests > 1,
                reasonCode: statusRequests > 1 ? null : 'MEDIA_SOURCE_UNAVAILABLE',
              }
            : source,
        );
        return new Response(JSON.stringify({ ...status, sources }), {
          headers: { 'Content-Type': 'application/json' },
        });
      }
      return new Response(JSON.stringify({ items: [] }), {
        headers: { 'Content-Type': 'application/json' },
      });
    }),
  );
  render(<ImportPanel onVideo={() => {}} onJob={() => {}} />);
  fireEvent.click(screen.getByRole('tab', { name: '本地上传' }));
  await screen.findByText(/上传暂不可用/);
  expect((screen.getByTestId('upload-file') as HTMLInputElement).disabled).toBe(true);
  fireEvent.click(screen.getByText('重试连接'));
  await waitFor(() =>
    expect((screen.getByTestId('upload-file') as HTMLInputElement).disabled).toBe(false),
  );
});
it('explains mixed-provider export rejection without downloading an invalid archive', async () => {
  vi.stubGlobal(
    'fetch',
    vi.fn(
      async () =>
        new Response(
          JSON.stringify({ code: 'MIXED_MEDIA_PROVIDERS', message: 'mixed providers' }),
          { status: 422 },
        ),
    ),
  );
  render(<ExportButton videoId="v" batchId="b" completed={1} />);
  fireEvent.click(screen.getByTestId('export-zip'));
  expect((await screen.findByRole('alert')).textContent).toContain('请按存储位置分开选择');
});
