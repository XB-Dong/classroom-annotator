import type { Snapshot } from './types';
export class ApiError extends Error {
  constructor(public status: number, public code: string, message: string, public requestId?: string) { super(message); }
}
export async function request<T>(path: string, options?: RequestInit): Promise<T> {
  let response: Response;
  try { response=await fetch(`/api/v1${path}`,options); }
  catch { throw new ApiError(0,'NETWORK_ERROR','无法连接服务，请检查网络后重试。'); }
  if (!response.ok) {
    const error=await response.json().catch(() => ({}));
    throw new ApiError(response.status,error.code ?? 'HTTP_ERROR',error.code==='DATASET_NOT_CONFIGURED' ? '公司数据接口尚未配置，请联系部署管理员。' : error.message ?? `请求失败 (${response.status})`,error.requestId);
  }
  return response.headers.get('content-type')?.includes('application/zip') ? await response.blob() as T : await response.json() as T;
}
export const json = (body: unknown): RequestInit => ({method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(body)});
export const idPath = (id: string) => encodeURIComponent(id);
export function saveSnapshot(snapshot: Snapshot) { return request<Snapshot>(`/frames/${idPath(snapshot.frameId)}/annotations`,{...json({expectedRevision:snapshot.revision,status:snapshot.status,annotations:snapshot.annotations}),method:'PUT'}); }
export function catalogId(id: number) { if (!Number.isSafeInteger(id) || id<0) throw new Error('公司目录 ID 无效，无法安全导入。'); return String(id); }
