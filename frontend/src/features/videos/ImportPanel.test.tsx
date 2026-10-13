import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { ImportPanel } from './ImportPanel';
afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});
describe('video import interfaces', () => {
  it('uploads a real selected File using multipart field file', async () => {
    const onVideo = vi.fn();
    vi.stubGlobal(
      'fetch',
      vi.fn(
        async () =>
          new Response(JSON.stringify({ id: 'v', name: 'synthetic.mp4' }), {
            headers: { 'Content-Type': 'application/json' },
          }),
      ),
    );
    render(<ImportPanel onVideo={onVideo} onJob={() => {}} />);
    fireEvent.click(screen.getByRole('tab', { name: '本地上传' }));
    const file = new File(['synthetic'], 'synthetic.mp4', { type: 'video/mp4' });
    fireEvent.change(screen.getByTestId('upload-file'), { target: { files: [file] } });
    fireEvent.click(screen.getByTestId('upload-submit'));
    await waitFor(() => expect(onVideo).toHaveBeenCalledOnce());
    const options = vi.mocked(fetch).mock.calls.find(([url, options]) => url === '/api/v1/videos' && options?.method === 'POST')?.[1];
    expect((options?.body as FormData).get('file')).toBe(file);
  });
  it('shows an unconfigured company API without invented lessons', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(
        async () =>
          new Response(
            JSON.stringify({ code: 'DATASET_NOT_CONFIGURED', message: 'missing', requestId: 'r' }),
            { status: 503 },
          ),
      ),
    );
    render(<ImportPanel onVideo={() => {}} onJob={() => {}} />);
    fireEvent.click(screen.getByRole('tab', { name: '公司接口' }));
    fireEvent.click(screen.getByText('加载公司课节'));
    expect(await screen.findByRole('alert')).toHaveProperty(
      'textContent',
      '公司数据接口尚未配置，请联系部署管理员。',
    );
  });
  it('offers channel four only and converts safe catalog ID to a decimal business string', async () => {
    const fetchMock = vi.fn(
      async (url: string) =>
        new Response(
          JSON.stringify(
            url.includes('lessons')
              ? {
                  type: 'lessons',
                  items: [
                    {
                      id: 1,
                      course_name: '合成课程',
                      videos: [
                        { id: 1, catalog_file_id: 42, channel: 4 },
                        { id: 2, catalog_file_id: 99, channel: 1 },
                      ],
                    },
                  ],
                  total: 1,
                  stats: {},
                }
              : {
                  id: 'j',
                  kind: 'import',
                  status: 'queued',
                  progress: 0,
                  createdAt: '2026-10-13T00:00:00Z',
                },
          ),
          { headers: { 'Content-Type': 'application/json' } },
        ),
    );
    vi.stubGlobal('fetch', fetchMock);
    const job = vi.fn();
    render(<ImportPanel onVideo={() => {}} onJob={job} />);
    fireEvent.click(screen.getByRole('tab', { name: '公司接口' }));
    fireEvent.click(screen.getByText('加载公司课节'));
    const buttons = await screen.findAllByText('导入学生全景 · 分段 1');
    expect(buttons).toHaveLength(1);
    fireEvent.click(buttons[0]);
    await waitFor(() => expect(job).toHaveBeenCalledOnce());
    const importCall = vi.mocked(fetch).mock.calls.find(([url, options]) => url === '/api/v1/dataset/imports' && options?.method === 'POST');
    expect(JSON.parse(importCall?.[1]?.body as string)).toEqual({
      catalogFileId: '42',
      name: '合成课程',
    });
    expect(job.mock.calls[0][0].videoId).toBeUndefined();
  });
});
