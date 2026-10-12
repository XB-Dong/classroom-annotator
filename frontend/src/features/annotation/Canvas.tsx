import { useEffect, useRef, useState } from 'react';
import { Stage, Layer, Image as CanvasImage, Rect, Transformer } from 'react-konva';
import Konva from 'konva';
import type { Annotation, Frame } from '../../shared/api/types';
import { normalizeBox, originalPoint, type Box } from './geometry';
interface Props { frame: Frame; annotations: Annotation[]; selected?: string; mode: 'draw'|'select'; disabled: boolean; select: (id?: string) => void; change: (annotations: Annotation[]) => void }
function HeadBox({annotation,selected,enabled,onSelect,onChange}: {annotation:Annotation;selected:boolean;enabled:boolean;onSelect:()=>void;onChange:(box:Box)=>void}) {
  const rect=useRef<Konva.Rect>(null); const transformer=useRef<Konva.Transformer>(null);
  useEffect(() => { if (selected && rect.current && transformer.current) {transformer.current.nodes([rect.current]); transformer.current.getLayer()?.batchDraw();} },[selected]);
  const [x,y,width,height]=annotation.bbox;
  return <><Rect ref={rect} x={x} y={y} width={width} height={height} stroke={selected?'#f59e0b':'#29d6ad'} strokeWidth={2} strokeScaleEnabled={false} fill={selected?'rgba(245,158,11,.12)':'rgba(41,214,173,.07)'} draggable={enabled} onClick={onSelect} onTap={onSelect} onDragEnd={e => onChange([e.target.x(),e.target.y(),width,height])} onTransformEnd={() => { const node=rect.current!; const box: Box=[node.x(),node.y(),node.width()*node.scaleX(),node.height()*node.scaleY()]; node.scaleX(1);node.scaleY(1);onChange(box); }}/>{selected && enabled && <Transformer ref={transformer} rotateEnabled={false} flipEnabled={false} keepRatio={false} anchorSize={8} borderStroke="#f59e0b" anchorStroke="#f59e0b"/>}</>;
}
export function AnnotationCanvas({frame,annotations,selected,mode,disabled,select,change}: Props) {
  const container=useRef<HTMLDivElement>(null); const stage=useRef<Konva.Stage>(null);
  const [size,setSize]=useState({width:700,height:500}); const [image,setImage]=useState<HTMLImageElement>(); const [imageError,setImageError]=useState(false);
  const [draft,setDraft]=useState<Box>(); const origin=useRef<{x:number;y:number}>(undefined);
  useEffect(() => { const el=container.current!; const observer=new ResizeObserver(entries => {const r=entries[0].contentRect;setSize({width:r.width,height:Math.max(240,r.height)});});observer.observe(el);return()=>observer.disconnect();},[]);
  useEffect(() => { let active=true;setImage(undefined);setImageError(false);const img=new Image();img.onload=()=>{if(active)setImage(img);};img.onerror=()=>{if(active)setImageError(true);};img.src=frame.imageUrl;return()=>{active=false;};},[frame.imageUrl]);
  const scale=Math.min(size.width/frame.width,size.height/frame.height); const width=frame.width*scale;const height=frame.height*scale;
  function point() {const p=stage.current?.getPointerPosition();return p?originalPoint(p,scale):undefined;}
  function start(event: Konva.KonvaEventObject<MouseEvent|TouchEvent>) {
    if (disabled || !image) return;
    if (mode==='draw') { const p=point();origin.current=p;if(p)setDraft([p.x,p.y,0,0]); }
    else if (event.target===event.target.getStage() || event.target.getClassName()==='Image') select(undefined);
  }
  function move() {if(!origin.current)return;const p=point();if(p)setDraft([origin.current.x,origin.current.y,p.x-origin.current.x,p.y-origin.current.y]);}
  function finish() {if(!origin.current)return;origin.current=undefined;const box=draft && normalizeBox(draft,frame.width,frame.height,true);setDraft(undefined);if(box){const id=crypto.randomUUID();change([...annotations,{id,bbox:box,source:'manual'}]);select(id);}}
  const preview=draft && normalizeBox(draft,frame.width,frame.height);
  return <div ref={container} className={`canvas-shell ${mode}`} data-testid="annotation-canvas">
    {imageError?<div className="canvas-message" role="alert">图片加载失败，请切换帧或刷新重试。</div>:!image?<div className="canvas-message">正在加载原图…</div>:<Stage ref={stage} width={width} height={height} scaleX={scale} scaleY={scale} onMouseDown={start} onTouchStart={start} onMouseMove={move} onTouchMove={move} onMouseUp={finish} onTouchEnd={finish}><Layer><CanvasImage image={image} width={frame.width} height={frame.height}/>{annotations.map(a=><HeadBox key={a.id} annotation={a} selected={a.id===selected} enabled={!disabled && mode==='select'} onSelect={()=>{if(mode==='select')select(a.id);}} onChange={box=>{const normalized=normalizeBox(box,frame.width,frame.height);if(normalized)change(annotations.map(item=>item.id===a.id?{...item,bbox:normalized}:item));}}/>)}{preview && <Rect x={preview[0]} y={preview[1]} width={preview[2]} height={preview[3]} stroke="#f59e0b" strokeWidth={2} strokeScaleEnabled={false}/>}</Layer></Stage>}
  </div>;
}
