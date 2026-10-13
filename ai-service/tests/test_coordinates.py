import io

import numpy as np
import pytest
from PIL import Image

from classroom_ai.preprocess import prepare_image
from classroom_ai.postprocess import decode_heads


def encoded_image(width, height, format="JPEG"):
    buffer = io.BytesIO()
    Image.new("RGB", (width, height), (30, 90, 180)).save(buffer, format)
    return buffer.getvalue()


def output(*rows):
    raw = np.zeros((1, 38, 8400), dtype=np.float32)
    for i, (cx, cy, width, height, score) in enumerate(rows):
        raw[0, :4, i] = [cx, cy, width, height]
        raw[0, 11, i] = score
    return raw


def test_landscape_roundtrip_preserves_original_pixels():
    tensor, geometry = prepare_image(encoded_image(1280, 720))
    assert tensor.shape == (1, 3, 640, 640)
    detections = decode_heads(output((100, 220, 40, 20, .9)), geometry, .25, .5)
    assert detections[0]["bbox"] == pytest.approx([160, 140, 80, 40])


def test_portrait_and_rounding_use_actual_dimensions():
    _, geometry = prepare_image(encoded_image(333, 1000))
    x, y, width, height = 20, 100, 50, 40
    cx = (x + width / 2) * geometry.scale_x + geometry.pad_x
    cy = (y + height / 2) * geometry.scale_y + geometry.pad_y
    result = decode_heads(output((cx, cy, width * geometry.scale_x,
                                  height * geometry.scale_y, .8)), geometry, .25, .5)
    assert result[0]["bbox"] == pytest.approx([x, y, width, height], abs=1e-4)


def test_clipping_and_nms_remove_duplicate_and_degenerate_boxes():
    _, geometry = prepare_image(encoded_image(640, 640))
    result = decode_heads(output((10, 10, 40, 40, .9), (11, 11, 40, 40, .8),
                                 (700, 700, 10, 10, .9), (20, 20, 0, 10, .9)),
                          geometry, .25, .5)
    assert len(result) == 1
    assert result[0]["bbox"] == pytest.approx([0, 0, 30, 30])
    assert result[0]["classId"] == 7


def test_head_channel_is_independent_of_body_or_face_scores():
    _, geometry = prepare_image(encoded_image(640, 640))
    raw = output((100, 100, 20, 20, .6), (200, 200, 20, 20, .1))
    raw[0, 4, 0] = .99
    raw[0, 20, 1] = .99
    result = decode_heads(raw, geometry, .25, .5)
    assert len(result) == 1
    assert result[0]["confidence"] == pytest.approx(.6)
