import { useEffect, useRef, useState } from 'react';
import { Stage, Layer, Image as CanvasImage, Rect, Transformer } from 'react-konva';
import Konva from 'konva';
import type { Annotation, Candidate, Frame } from '../../shared/api/types';
import { normalizeBox, originalPoint, type Box } from './geometry';
import { applyNodeBox } from './nodeGeometry';
import { authSession } from '../auth/session';
interface Props {
  frame: Frame;
  annotations: Annotation[];
  selected?: string;
  mode: 'draw' | 'select';
  disabled: boolean;
  select: (id?: string) => void;
  change: (annotations: Annotation[]) => void;
  candidates?: Candidate[];
  selectedCandidate?: string;
  adjustedCandidateBox?: Box;
  selectCandidate?: (id: string) => void;
  changeCandidate?: (box: Box) => void;
  interactionChanged?: (active: boolean) => void;
}
export function HeadBox({
  annotation,
  selected,
  enabled,
  onSelect,
  onChange,
  candidate = false,
  risky = false,
  interactionChanged,
}: {
  annotation: Annotation;
  selected: boolean;
  enabled: boolean;
  onSelect: () => void;
  onChange: (box: Box, node: Konva.Rect) => boolean;
  candidate?: boolean;
  risky?: boolean;
  interactionChanged?: (active: boolean) => void;
}) {
  const rect = useRef<Konva.Rect>(null);
  const transformer = useRef<Konva.Transformer>(null);
  useEffect(() => {
    if (selected && rect.current && transformer.current) {
      transformer.current.nodes([rect.current]);
      transformer.current.getLayer()?.batchDraw();
    }
  }, [selected, enabled]);
  const [x, y, width, height] = annotation.bbox;
  return (
    <>
      <Rect
        ref={rect}
        x={x}
        y={y}
        width={width}
        height={height}
        stroke={selected ? '#4c9aff' : candidate ? (risky ? '#f2bc55' : '#a6c4e8') : '#29d6ad'}
        dash={candidate ? [6, 4] : undefined}
        strokeWidth={2}
        strokeScaleEnabled={false}
        fill={
          selected
            ? 'rgba(76,154,255,.12)'
            : candidate
              ? 'rgba(242,188,85,.05)'
              : 'rgba(41,214,173,.07)'
        }
        draggable={enabled}
        onClick={onSelect}
        onTap={onSelect}
        onDragStart={() => interactionChanged?.(true)}
        onTransformStart={() => interactionChanged?.(true)}
        onDragEnd={(e) => {
          if (!onChange([e.target.x(), e.target.y(), width, height], rect.current!))
            e.target.position({ x, y });
          interactionChanged?.(false);
        }}
        onTransformEnd={() => {
          const node = rect.current!;
          const box: Box = [
            node.x(),
            node.y(),
            node.width() * node.scaleX(),
            node.height() * node.scaleY(),
          ];
          node.scaleX(1);
          node.scaleY(1);
          if (!onChange(box, node)) node.position({ x, y });
          interactionChanged?.(false);
        }}
      />
      {selected && enabled && (
        <Transformer
          ref={transformer}
          rotateEnabled={false}
          flipEnabled={false}
          keepRatio={false}
          anchorSize={8}
          borderStroke="#4c9aff"
          anchorStroke="#4c9aff"
        />
      )}
    </>
  );
}
export function AnnotationCanvas({
  frame,
  annotations,
  selected,
  mode,
  disabled,
  select,
  change,
  candidates = [],
  selectedCandidate,
  adjustedCandidateBox,
  selectCandidate,
  changeCandidate,
  interactionChanged,
}: Props) {
  const container = useRef<HTMLDivElement>(null);
  const stage = useRef<Konva.Stage>(null);
  const [size, setSize] = useState({ width: 700, height: 500 });
  const [image, setImage] = useState<HTMLImageElement>();
  const [imageError, setImageError] = useState(false);
  const [imageRetry, setImageRetry] = useState(0);
  const loadedFrameId = useRef<string>(undefined);
  const [draft, setDraft] = useState<Box>();
  const origin = useRef<{ x: number; y: number }>(undefined);
  useEffect(() => {
    const el = container.current!;
    const observer = new ResizeObserver((entries) => {
      const r = entries[0].contentRect;
      setSize({ width: r.width, height: Math.max(240, r.height) });
    });
    observer.observe(el);
    return () => observer.disconnect();
  }, []);
  useEffect(() => {
    let active = true;
    const generation = authSession.generation;
    if (loadedFrameId.current !== frame.id) setImage(undefined);
    setImageError(false);
    const img = new Image();
    img.onload = () => {
      if (active && generation === authSession.generation) {
        loadedFrameId.current = frame.id;
        setImage(img);
        setImageError(false);
      }
    };
    img.onerror = () => {
      if (active && generation === authSession.generation) setImageError(true);
    };
    img.src = frame.imageUrl;
    return () => {
      active = false;
      img.onload = null;
      img.onerror = null;
      img.src = '';
    };
  }, [frame.id, frame.imageUrl, imageRetry]);
  const scale = Math.min(size.width / frame.width, size.height / frame.height);
  const width = frame.width * scale;
  const height = frame.height * scale;
  function point() {
    const p = stage.current?.getPointerPosition();
    return p ? originalPoint(p, scale) : undefined;
  }
  function start(event: Konva.KonvaEventObject<MouseEvent | TouchEvent>) {
    if (disabled || !image) return;
    if (mode === 'draw') {
      const p = point();
      origin.current = p;
      if (p) interactionChanged?.(true);
      if (p) setDraft([p.x, p.y, 0, 0]);
    } else if (event.target === event.target.getStage() || event.target.getClassName() === 'Image')
      select(undefined);
  }
  function move() {
    if (!origin.current) return;
    const p = point();
    if (p)
      setDraft([
        origin.current.x,
        origin.current.y,
        p.x - origin.current.x,
        p.y - origin.current.y,
      ]);
  }
  function finish() {
    if (!origin.current) return;
    origin.current = undefined;
    const box = draft && normalizeBox(draft, frame.width, frame.height, true);
    setDraft(undefined);
    if (box) {
      const id = crypto.randomUUID();
      change([...annotations, { id, bbox: box, source: 'manual' }]);
      select(id);
    }
    interactionChanged?.(false);
  }
  const preview = draft && normalizeBox(draft, frame.width, frame.height);
  return (
    <div ref={container} className={`canvas-shell ${mode}`} data-testid="annotation-canvas">
      {imageError && image && (
        <div className="canvas-message" role="status">
          原图连接中断，已显示已加载的原图。标注可继续保存。{' '}
          <button onClick={() => setImageRetry((value) => value + 1)}>重试加载原图</button>
        </div>
      )}
      {imageError && !image ? (
        <div className="canvas-message" role="alert">
          原图暂不可用，媒体服务可能离线。
          <button onClick={() => setImageRetry((value) => value + 1)}>重试加载原图</button>
        </div>
      ) : !image ? (
        <div className="canvas-message">正在加载原图…</div>
      ) : (
        <div
          role="img"
          aria-label="当前标注原图"
          data-testid="annotation-image-bounds"
          style={{ width, height, flexShrink: 0 }}
        >
          <Stage
            ref={stage}
            width={width}
            height={height}
            scaleX={scale}
            scaleY={scale}
            onMouseDown={start}
            onTouchStart={start}
            onMouseMove={move}
            onTouchMove={move}
            onMouseUp={finish}
            onTouchEnd={finish}
            onMouseLeave={finish}
          >
            <Layer>
              <CanvasImage image={image} width={frame.width} height={frame.height} />
              {annotations.map((a) => (
                <HeadBox
                  key={a.id}
                  annotation={a}
                  selected={a.id === selected}
                  enabled={!disabled && mode === 'select'}
                  interactionChanged={interactionChanged}
                  onSelect={() => {
                    if (mode === 'select') select(a.id);
                  }}
                  onChange={(box, node) => {
                    const normalized = applyNodeBox(node, box, frame.width, frame.height);
                    if (normalized)
                      change(
                        annotations.map((item) =>
                          item.id === a.id ? { ...item, bbox: normalized } : item,
                        ),
                      );
                    return normalized !== null;
                  }}
                />
              ))}
              {candidates.map((candidate) => (
                <HeadBox
                  key={`candidate-${candidate.id}`}
                  annotation={{
                    id: candidate.id,
                    bbox:
                      candidate.id === selectedCandidate && adjustedCandidateBox
                        ? adjustedCandidateBox
                        : candidate.bbox,
                    source: 'manual',
                  }}
                  candidate
                  risky={candidate.riskReasons.length > 0}
                  selected={candidate.id === selectedCandidate}
                  enabled={
                    !disabled &&
                    mode === 'select' &&
                    candidate.id === selectedCandidate &&
                    !candidate.riskReasons.includes('OVERLAP_EXISTING')
                  }
                  interactionChanged={interactionChanged}
                  onSelect={() => {
                    if (!disabled && mode === 'select') selectCandidate?.(candidate.id);
                  }}
                  onChange={(box, node) => {
                    const normalized = applyNodeBox(node, box, frame.width, frame.height);
                    if (normalized) changeCandidate?.(normalized);
                    return normalized !== null;
                  }}
                />
              ))}
              {preview && (
                <Rect
                  x={preview[0]}
                  y={preview[1]}
                  width={preview[2]}
                  height={preview[3]}
                  stroke="#f59e0b"
                  strokeWidth={2}
                  strokeScaleEnabled={false}
                />
              )}
            </Layer>
          </Stage>
        </div>
      )}
    </div>
  );
}
