import math

import pytest

from classroom_ai.pipeline import infer_pipeline
from test_coordinates import encoded_image, output


def test_small_image_runs_once():
    calls = []
    def predict(tensor):
        calls.append(tensor)
        return output((100, 100, 20, 20, .8))
    result = infer_pipeline(encoded_image(640, 640), predict, .25, .5)
    assert len(calls) == 1
    assert result["mode"] == "fullframe"
    assert result["detections"][0]["bbox"] == pytest.approx([90, 90, 20, 20])


@pytest.mark.parametrize("scores", [[.9,.8,.7,.6,.5], [.5,.6,.7,.8,.9]])
def test_overlapping_tiles_map_to_original_and_global_nms_is_order_independent(scores):
    width, height = 1280, 720
    tw, th = math.ceil(width / 1.85), math.ceil(height / 1.85)
    windows = [(0,0,width,height),(0,0,tw,th),(width-tw,0,tw,th),
               (0,height-th,tw,th),(width-tw,height-th,tw,th)]
    original = [620, 340, 40, 20]
    rows = []
    for (ox,oy,w,h), score in zip(windows, scores):
        scale = min(640/w,640/h)
        rw,rh = round(w*scale),round(h*scale)
        sx,sy = rw/w,rh/h
        px,py = (640-rw)//2,(640-rh)//2
        x,y,bw,bh = original
        rows.append(output(((x+bw/2-ox)*sx+px,(y+bh/2-oy)*sy+py,bw*sx,bh*sy,score)))
    calls=[]
    def predict(tensor):
        calls.append(1)
        return rows[len(calls)-1]
    result=infer_pipeline(encoded_image(width,height),predict,.25,.5)
    assert len(calls)==5
    assert len(result["detections"])==1
    assert result["detections"][0]["bbox"]==pytest.approx(original,abs=1e-3)
    assert result["detections"][0]["confidence"]==pytest.approx(.9)


def test_internal_tile_edge_is_marked_for_explicit_review():
    calls=[]
    def predict(tensor):
        calls.append(1)
        return output((630,320,40,30,.9)) if len(calls)==2 else output()
    result=infer_pipeline(encoded_image(1280,720),predict,.25,.5)
    detection=result["detections"][0]
    assert detection["clipped"] is True
    assert detection["tileBoundary"] is True
    x,y,w,h=detection["bbox"]
    assert 0<=x<x+w<=1280
    assert 0<=y<y+h<=720
