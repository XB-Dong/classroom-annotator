import type { Snapshot } from './types';
import { authSession } from '../../features/auth/session';
export class ApiError extends Error {
  constructor(
    public status: number,
    public code: string,
    message: string,
    public requestId?: string,
  ) {
    super(message);
  }
}
export async function request<T>(path: string, options?: RequestInit): Promise<T> {
  const generation = authSession.generation;
  const controller = new AbortController();
  const untrack = path === '/auth/logout' ? () => {} : authSession.track(controller);
  const abort = () => controller.abort();
  options?.signal?.addEventListener('abort', abort, { once: true });
  if (options?.signal?.aborted) controller.abort();
  const headers = new Headers(options?.headers);
  if (!['GET', 'HEAD'].includes(options?.method?.toUpperCase() ?? 'GET') && authSession.current)
    headers.set('X-CSRF-Token', authSession.current.csrfToken);
  const assertCurrent = () => {
    if (path === '/auth/logout' && !authSession.current) return;
    if (generation !== authSession.generation)
      throw new ApiError(0, 'STALE_AUTH_RESPONSE', '账号已切换，已忽略旧请求。');
  };
  try {
    let response: Response;
    try {
      response = await fetch(`/api/v1${path}`, {
        ...options,
        headers,
        credentials: 'same-origin',
        signal: controller.signal,
      });
    } catch {
      assertCurrent();
      throw new ApiError(0, 'NETWORK_ERROR', '无法连接服务，请检查网络后重试。');
    }
    assertCurrent();
    if (!response.ok) {
      const error = await response.json().catch(() => ({}));
      assertCurrent();
      if (
        response.status === 401 &&
        authSession.current &&
        !['/auth/login', '/auth/register'].includes(path)
      )
        authSession.clear();
      throw new ApiError(
        response.status,
        error.code ?? 'HTTP_ERROR',
        error.code === 'DATASET_NOT_CONFIGURED'
          ? '公司数据接口尚未配置，请联系部署管理员。'
          : error.code === 'MIXED_MEDIA_PROVIDERS'
            ? '所选图片位于不同存储位置，请按存储位置分开选择后导出。'
            : error.code === 'MEDIA_SOURCE_UNAVAILABLE'
              ? '素材暂不可用，请检查当前部署主机的媒体服务。'
              : error.code === 'SAMPLE_UNAVAILABLE'
                ? '示例尚未缓存或暂不可用，请稍后重试。'
                : (error.message ?? `请求失败 (${response.status})`),
        error.requestId,
      );
    }
    const result =
      response.status === 204
        ? undefined
        : response.headers.get('content-type')?.includes('application/zip')
          ? await response.blob()
          : await response.json();
    assertCurrent();
    return result as T;
  } finally {
    untrack();
    options?.signal?.removeEventListener('abort', abort);
  }
}
export const json = (body: unknown): RequestInit => ({
  method: 'POST',
  headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify(body),
});
export const idPath = (id: string) => encodeURIComponent(id);
export function saveSnapshot(snapshot: Snapshot) {
  return request<Snapshot>(`/frames/${idPath(snapshot.frameId)}/annotations`, {
    ...json({
      expectedRevision: snapshot.revision,
      status: snapshot.status,
      annotations: snapshot.annotations,
    }),
    method: 'PUT',
  });
}
export function catalogId(id: number) {
  if (!Number.isSafeInteger(id) || id < 0) throw new Error('公司目录 ID 无效，无法安全导入。');
  return String(id);
}
