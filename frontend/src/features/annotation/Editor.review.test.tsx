import { act, cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import { Editor } from './Editor';
import type { Annotation, Frame } from '../../shared/api/types';
vi.mock('./Canvas', () => ({
  AnnotationCanvas: ({
    annotations,
    change,
    changeCandidate,
    interactionChanged,
  }: {
    annotations: Annotation[];
    change: (a: Annotation[]) => void;
    changeCandidate: (b: number[]) => void;
    interactionChanged: (b: boolean) => void;
  }) => (
    <>
      <button
        onClick={() =>
          change([...annotations, { id: 'manual', bbox: [20, 20, 8, 8], source: 'manual' }])
        }
      >
        手工补框测试
      </button>
      <button onClick={() => changeCandidate([2, 3, 10, 11])}>调整候选测试</button>
      <button onClick={() => interactionChanged(true)}>开始画框测试</button>
      <button onClick={() => interactionChanged(false)}>结束画框测试</button>
    </>
  ),
}));
const frame: Frame = {
  id: 'f',
  videoId: 'v',
  batchId: 'b',
  timestampMs: 0,
  width: 100,
  height: 100,
  imageUrl: '/f.jpg',
  status: 'in_progress',
  revision: 0,
};
const candidates = [
  {
    id: 'safe',
    runId: 'run',
    bbox: [1, 1, 8, 8],
    confidence: 0.9,
    reviewState: 'pending',
    riskReasons: [],
  },
  {
    id: 'risk',
    runId: 'run',
    bbox: [10, 10, 8, 8],
    confidence: 0.4,
    reviewState: 'pending',
    riskReasons: ['LOW_CONFIDENCE'],
  },
  {
    id: 'overlap',
    runId: 'run',
    bbox: [20, 20, 8, 8],
    confidence: 0.9,
    reviewState: 'pending',
    riskReasons: ['OVERLAP_EXISTING'],
  },
];
let server: Record<string, unknown>;
let writes: Array<{ path: string; body: Record<string, unknown> }>;
beforeEach(() => {
  localStorage.clear();
  writes = [];
  server = {
    frameId: 'f',
    revision: 0,
    status: 'in_progress',
    annotations: [],
    candidates,
    currentRun: {
      id: 'run',
      status: 'completed',
      modelId: 'm',
      modelSHA256: 'a'.repeat(64),
      pipelineVersion: '1',
      baseRevision: 0,
    },
    issues: [],
  };
  vi.stubGlobal(
    'fetch',
    vi.fn(async (path: string, options?: RequestInit) => {
      if (options?.method === 'PUT') {
        const body = JSON.parse(options.body as string);
        writes.push({ path, body });
        server = { ...server, revision: Number(server.revision) + 1 };
        if (path.endsWith('/annotations'))
          server = { ...server, annotations: body.annotations, status: body.status };
        else
          server = {
            ...server,
            candidates: (server.candidates as typeof candidates).map((c) =>
              (body.decisions as Array<{ candidateId: string; decision: string }>).some(
                (d) => d.candidateId === c.id,
              )
                ? { ...c, reviewState: 'accepted' }
                : c,
            ),
          };
      }
      return new Response(JSON.stringify(server), {
        headers: { 'Content-Type': 'application/json' },
      });
    }),
  );
});
afterEach(() => {
  cleanup();
  vi.useRealTimers();
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
});
function open() {
  return render(
    <Editor
      frame={frame}
      draftScope={{ deploymentOrigin: window.location.origin, workspaceId: 'w', userId: 'u' }}
      onSaved={() => {}}
    />,
  );
}
it('blocks completion for pending candidates and bulk accepts only candidates without risk', async () => {
  open();
  await screen.findByText('AI 复核');
  expect((screen.getByTestId('complete-frame') as HTMLButtonElement).disabled).toBe(true);
  fireEvent.click(screen.getByText('接受无风险待审框'));
  await waitFor(() => expect(writes).toHaveLength(1));
  expect(writes[0].body).toMatchObject({
    expectedRevision: 0,
    decisions: [{ candidateId: 'safe', decision: 'accepted' }],
    issueResolutions: [],
  });
});
it('saves a pending manual edit before the candidate decision and uses its acknowledged revision', async () => {
  open();
  await screen.findByText('AI 复核');
  fireEvent.click(screen.getByText('手工补框测试'));
  fireEvent.click(screen.getByText('接受无风险待审框'));
  await waitFor(() => expect(writes).toHaveLength(2));
  expect(writes[0].path).toContain('/annotations');
  expect(writes[0].body.annotations).toEqual([
    { id: 'manual', bbox: [20, 20, 8, 8], source: 'manual' },
  ]);
  expect(writes[1].body.expectedRevision).toBe(1);
  await screen.findByText('学生人头 1');
});
it('requires overlap rejection with ALREADY_LABELED and disallows accepting that candidate', async () => {
  server = {
    ...server,
    annotations: [{ id: 'already-accepted', bbox: [20, 20, 8, 8], source: 'ai' }],
    candidates: candidates.map((candidate) =>
      candidate.id === 'overlap' ? { ...candidate, riskReasons: [] } : candidate,
    ),
  };
  open();
  await screen.findByText('AI 复核');
  fireEvent.click(screen.getByTestId('candidate-overlap'));
  expect((screen.getByText('接受选中候选') as HTMLButtonElement).disabled).toBe(true);
  expect((screen.getByText('调整后接受') as HTMLButtonElement).disabled).toBe(true);
  fireEvent.click(screen.getByText('拒绝选中候选'));
  await waitFor(() => expect(writes).toHaveLength(1));
  expect(writes[0].body.decisions).toEqual([
    { candidateId: 'overlap', decision: 'rejected', reasonCode: 'ALREADY_LABELED' },
  ]);
});
it('requires explicit manual image check when inference finds no candidates', async () => {
  server = {
    ...server,
    candidates: [],
    issues: [{ id: 'issue', reasonCode: 'NO_DETECTIONS', resolved: false }],
  };
  open();
  await screen.findByText('AI 复核');
  expect((screen.getByTestId('complete-frame') as HTMLButtonElement).disabled).toBe(true);
  fireEvent.click(screen.getByText('已人工检查本图'));
  await waitFor(() => expect(writes).toHaveLength(1));
  expect(writes[0].body).toMatchObject({
    expectedRevision: 0,
    decisions: [],
    issueResolutions: [{ id: 'issue', resolution: 'manual_checked' }],
  });
});
it('submits adjusted candidate coordinates only after an explicit adjusted acceptance', async () => {
  open();
  await screen.findByText('AI 复核');
  fireEvent.click(screen.getByTestId('candidate-risk'));
  fireEvent.click(screen.getByText('调整候选测试'));
  expect(writes).toHaveLength(0);
  fireEvent.click(screen.getByText('调整后接受'));
  await waitFor(() => expect(writes).toHaveLength(1));
  expect(writes[0].body.decisions).toEqual([
    { candidateId: 'risk', decision: 'adjusted', bbox: [2, 3, 10, 11] },
  ]);
});
it('does not accept unchanged coordinates after the user has adjusted the selected candidate', async () => {
  open();
  await screen.findByText('AI 复核');
  fireEvent.click(screen.getByTestId('candidate-risk'));
  fireEvent.click(screen.getByText('调整候选测试'));
  expect((screen.getByText('接受选中候选') as HTMLButtonElement).disabled).toBe(true);
  expect((screen.getByText('接受无风险待审框') as HTMLButtonElement).disabled).toBe(true);
});
it('keeps a provisional candidate adjustment when a server refresh arrives', async () => {
  vi.spyOn(document, 'hidden', 'get').mockReturnValue(false);
  let resolveLatest!: (r: Response) => void;
  let reads = 0;
  vi.stubGlobal(
    'fetch',
    vi.fn(async () => {
      if (++reads === 1)
        return new Response(JSON.stringify(server), {
          headers: { 'Content-Type': 'application/json' },
        });
      return new Promise<Response>((resolve) => {
        resolveLatest = resolve;
      });
    }),
  );
  vi.useFakeTimers();
  await act(async () => {
    open();
  });
  await act(async () => {
    vi.advanceTimersByTime(5000);
  });
  fireEvent.click(screen.getByTestId('candidate-risk'));
  fireEvent.click(screen.getByText('调整候选测试'));
  await act(async () => {
    resolveLatest(
      new Response(JSON.stringify({ ...server, revision: 5 }), {
        headers: { 'Content-Type': 'application/json' },
      }),
    );
  });
  expect((screen.getByText('调整后接受') as HTMLButtonElement).disabled).toBe(false);
});
it('reuses the generation request key after a network failure, using the current saved revision', async () => {
  server = { ...server, candidates: [] };
  let attempts = 0;
  const keys: string[] = [];
  vi.stubGlobal(
    'fetch',
    vi.fn(async (path: string, options?: RequestInit) => {
      if (options?.method === 'POST') {
        keys.push(new Headers(options.headers).get('Idempotency-Key')!);
        expect(JSON.parse(options.body as string)).toEqual({ expectedRevision: 0 });
        if (++attempts === 1) throw new Error('offline');
        return new Response(
          JSON.stringify({ id: 'job', kind: 'preannotation', status: 'queued' }),
          { headers: { 'Content-Type': 'application/json' } },
        );
      }
      return new Response(JSON.stringify(server), {
        headers: { 'Content-Type': 'application/json' },
      });
    }),
  );
  open();
  await screen.findByText('AI 复核');
  fireEvent.click(screen.getByText('重新生成本帧 AI 候选'));
  await screen.findByText(/无法连接服务/);
  fireEvent.click(screen.getByText('重新生成本帧 AI 候选'));
  await waitFor(() => expect(keys).toHaveLength(2));
  expect(keys[0]).toBe(keys[1]);
});
it('does not allow decisions while a canvas gesture is unfinished', async () => {
  open();
  await screen.findByText('AI 复核');
  fireEvent.click(screen.getByText('开始画框测试'));
  expect((screen.getByText('接受无风险待审框') as HTMLButtonElement).disabled).toBe(true);
  fireEvent.click(screen.getByText('结束画框测试'));
  expect((screen.getByText('接受无风险待审框') as HTMLButtonElement).disabled).toBe(false);
});
it('preserves manual draft when a revision response arrives after a local edit', async () => {
  vi.spyOn(document, 'hidden', 'get').mockReturnValue(false);
  let resolveLatest!: (r: Response) => void;
  let reads = 0;
  vi.stubGlobal(
    'fetch',
    vi.fn(async () => {
      reads++;
      if (reads === 1)
        return new Response(JSON.stringify(server), {
          headers: { 'Content-Type': 'application/json' },
        });
      return new Promise<Response>((resolve) => {
        resolveLatest = resolve;
      });
    }),
  );
  vi.useFakeTimers();
  await act(async () => {
    open();
  });
  expect(screen.getByText('AI 复核')).toBeTruthy();
  await act(async () => {
    vi.advanceTimersByTime(5000);
  });
  fireEvent.click(screen.getByText('手工补框测试'));
  await act(async () => {
    resolveLatest(
      new Response(JSON.stringify({ ...server, revision: 5, annotations: [] }), {
        headers: { 'Content-Type': 'application/json' },
      }),
    );
  });
  expect(screen.getByText('学生人头 1')).toBeTruthy();
  expect(screen.getByText(/本地草稿已保留/)).toBeTruthy();
  cleanup();
  vi.useRealTimers();
});
it('does not refresh server annotations into an unfinished canvas gesture', async () => {
  vi.spyOn(document, 'hidden', 'get').mockReturnValue(false);
  let resolveLatest!: (r: Response) => void;
  let reads = 0;
  vi.stubGlobal(
    'fetch',
    vi.fn(async () => {
      if (++reads === 1)
        return new Response(JSON.stringify(server), {
          headers: { 'Content-Type': 'application/json' },
        });
      return new Promise<Response>((resolve) => {
        resolveLatest = resolve;
      });
    }),
  );
  vi.useFakeTimers();
  await act(async () => {
    open();
  });
  await act(async () => {
    vi.advanceTimersByTime(5000);
  });
  fireEvent.click(screen.getByText('开始画框测试'));
  await act(async () => {
    resolveLatest(
      new Response(
        JSON.stringify({
          ...server,
          revision: 5,
          annotations: [{ id: 'other', bbox: [1, 1, 5, 5], source: 'manual' }],
        }),
        { headers: { 'Content-Type': 'application/json' } },
      ),
    );
  });
  expect(screen.queryByText('学生人头 1')).toBeNull();
});
