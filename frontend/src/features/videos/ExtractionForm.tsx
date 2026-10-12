import { useState } from 'react';
import { idPath, json, request } from '../../shared/api/client';
import type { Job, Video } from '../../shared/api/types';
export function ExtractionForm({video,onJob}: {video:Video;onJob:(j:Job)=>void}) {
  const [interval,setInterval]=useState('5');const [start,setStart]=useState('0');const [end,setEnd]=useState('');const [busy,setBusy]=useState(false);const [error,setError]=useState<string>();
  async function submit() {setError(undefined);const i=Number(interval),s=Number(start),e=end===''?Math.ceil(video.durationSeconds*1000)/1000:Number(end);const precise=(n:number)=>Math.abs(n*1000-Math.round(n*1000))<1e-7;
    if(![i,s,e].every(Number.isFinite)||i<.001||s<0||s>=e||e>Math.ceil(video.durationSeconds*1000)/1000||![i,s,e].every(precise)){setError('请输入有效区间与正数间隔，最多三位小数；结束时间须大于开始时间且不超过视频时长。');return;}
    setBusy(true);try{onJob(await request<Job>(`/videos/${idPath(video.id)}/extractions`,json({intervalSeconds:i,startSeconds:s,endSeconds:e})));}catch(e){setError((e as Error).message);}finally{setBusy(false);}
  }
  return <section className="extraction-form"><div className="panel-title"><h2>抽帧设置</h2><span className="eyebrow">02 / 批次</span></div><p className="muted small">{video.width} × {video.height} · {video.durationSeconds.toFixed(3)} 秒</p><div className="fields"><label>间隔（秒）<input data-testid="interval-seconds" type="number" min="0.001" step="0.001" value={interval} onChange={e=>setInterval(e.target.value)}/></label><label>开始（秒）<input data-testid="start-seconds" type="number" min="0" step="0.001" value={start} onChange={e=>setStart(e.target.value)}/></label><label>结束（秒）<input data-testid="end-seconds" type="number" step="0.001" placeholder={String(Math.ceil(video.durationSeconds*1000)/1000)} value={end} onChange={e=>setEnd(e.target.value)}/></label></div><button data-testid="extract-submit" className="primary full" disabled={busy} onClick={()=>void submit()}>{busy?'正在提交…':'创建新抽帧批次'}</button><p className="muted small">每次抽帧生成新批次，保留已有标注。</p>{error&&<p className="notice error" role="alert">{error}</p>}</section>;
}
