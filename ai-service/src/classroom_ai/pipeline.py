import io
import math

import numpy as np
from PIL import Image

from .preprocess import prepare_image, prepare_rgb
from .postprocess import decode_heads, iou

PIPELINE_VERSION = "classroom-head-v1-fullframe-2x2-15"


def infer_pipeline(data, predict, confidence, nms):
    tensor, geometry = prepare_image(data)
    detections = decode_heads(predict(tensor), geometry, confidence, nms)
    width, height = geometry.width, geometry.height
    mode = "fullframe"
    if max(width, height) >= 1280:
        mode = "fullframe_plus_2x2"
        tile_width, tile_height = math.ceil(width / 1.85), math.ceil(height / 1.85)
        with Image.open(io.BytesIO(data)) as source:
            source.load()
            rgb = source.convert("RGB")
        for offset_x, offset_y in ((0, 0), (width - tile_width, 0),
                                    (0, height - tile_height),
                                    (width - tile_width, height - tile_height)):
            crop = rgb.crop((offset_x, offset_y, offset_x + tile_width, offset_y + tile_height))
            tile_tensor, tile_geometry = prepare_rgb(crop)
            candidates = decode_heads(predict(tile_tensor), tile_geometry, confidence, nms)
            for detection in candidates:
                x, y, w, h = detection["bbox"]
                detection["tileBoundary"] = bool(detection["clipped"] or
                    (offset_x > 0 and x <= 1) or (offset_y > 0 and y <= 1) or
                    (offset_x + tile_width < width and x + w >= tile_width - 1) or
                    (offset_y + tile_height < height and y + h >= tile_height - 1))
                detection["bbox"] = [x + offset_x, y + offset_y, w, h]
            detections.extend(candidates)
        detections = global_nms(detections, nms)
    return {"image": {"width": width, "height": height}, "detections": detections,
            "pipelineVersion": PIPELINE_VERSION, "mode": mode}


def global_nms(detections, threshold):
    if not detections:
        return []
    boxes = np.array([[d["bbox"][0], d["bbox"][1],
                       d["bbox"][0] + d["bbox"][2], d["bbox"][1] + d["bbox"][3]]
                      for d in detections])
    order = np.argsort([-d["confidence"] for d in detections], kind="stable")
    selected = []
    while len(order):
        index = order[0]
        selected.append(detections[index])
        remaining = order[1:]
        order = remaining[iou(boxes[index], boxes[remaining]) <= threshold]
    return selected
