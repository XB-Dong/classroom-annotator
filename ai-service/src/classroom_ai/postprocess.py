import math

import numpy as np


def iou(box, others):
    left = np.maximum(box[:2], others[:, :2])
    right = np.minimum(box[2:], others[:, 2:])
    overlap = np.maximum(0, right - left).prod(axis=1)
    area = (box[2] - box[0]) * (box[3] - box[1])
    other_areas = (others[:, 2] - others[:, 0]) * (others[:, 3] - others[:, 1])
    return overlap / (area + other_areas - overlap)


def decode_heads(raw, geometry, confidence_threshold, nms_threshold):
    if any(not math.isfinite(value) or not 0 <= value <= 1
           for value in (confidence_threshold, nms_threshold)):
        raise ValueError("Thresholds must be finite values between 0 and 1")
    raw = np.asarray(raw)
    if raw.shape != (1, 38, 8400):
        raise ValueError("Unexpected model output shape")
    # Wholebody34 is multi-label: head=7 is channel 4+7, with no objectness channel.
    rows = raw[0, [0, 1, 2, 3, 11], :].T.astype(np.float64)
    valid = (np.isfinite(rows).all(axis=1) & (rows[:, 2:4] > 0).all(axis=1)
             & (rows[:, 4] >= confidence_threshold) & (rows[:, 4] > 0)
             & (rows[:, 4] <= 1))
    rows = rows[valid]
    if len(rows) == 0:
        return []
    boxes = np.concatenate((rows[:, :2] - rows[:, 2:4] / 2,
                            rows[:, :2] + rows[:, 2:4] / 2), axis=1)
    boxes[:, [0, 2]] = (boxes[:, [0, 2]] - geometry.pad_x) / geometry.scale_x
    boxes[:, [1, 3]] = (boxes[:, [1, 3]] - geometry.pad_y) / geometry.scale_y
    unclipped = boxes.copy()
    boxes[:, [0, 2]] = np.clip(boxes[:, [0, 2]], 0, geometry.width)
    boxes[:, [1, 3]] = np.clip(boxes[:, [1, 3]], 0, geometry.height)
    valid = (boxes[:, 2:] > boxes[:, :2]).all(axis=1)
    clipped = np.any(unclipped != boxes, axis=1)[valid]
    boxes, scores = boxes[valid], rows[valid, 4]
    order = np.argsort(-scores, kind="stable")
    selected = []
    while len(order):
        index = order[0]
        box = boxes[index]
        selected.append({"bbox": [float(box[0]), float(box[1]),
                                   float(box[2] - box[0]), float(box[3] - box[1])],
                         "confidence": float(scores[index]), "classId": 7,
                         "clipped": bool(clipped[index]), "tileBoundary": False})
        remaining = order[1:]
        order = remaining[iou(box, boxes[remaining]) <= nms_threshold]
    return selected
