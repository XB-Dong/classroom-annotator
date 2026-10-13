import numpy as np
import pytest

from classroom_ai.preprocess import prepare_image, ImageError
from classroom_ai.postprocess import decode_heads
from test_coordinates import encoded_image, output


@pytest.mark.parametrize("shape", [(1, 37, 8400), (1, 8400, 38), (2, 38, 8400)])
def test_wrong_raw_model_shape_is_rejected(shape):
    _, geometry = prepare_image(encoded_image(640, 640))
    with pytest.raises(ValueError):
        decode_heads(np.zeros(shape), geometry, .25, .5)


def test_nonfinite_invalid_scores_and_dimensions_are_discarded():
    _, geometry = prepare_image(encoded_image(640, 640))
    raw = output((100, 100, 20, 20, .9), (100, 100, 20, 20, float("nan")),
                 (100, 100, -20, 20, .9), (100, 100, 20, 20, 1.2))
    raw[0, 0, 0] = float("inf")
    assert decode_heads(raw, geometry, .25, .5) == []


@pytest.mark.parametrize("value", [float("nan"), -1, 1.1])
def test_invalid_thresholds_are_rejected(value):
    _, geometry = prepare_image(encoded_image(640, 640))
    with pytest.raises(ValueError):
        decode_heads(output(), geometry, value, .5)


def test_image_bounds_and_invalid_encoding_are_rejected():
    with pytest.raises(ImageError):
        prepare_image(b"not an image")
    with pytest.raises(ImageError):
        prepare_image(encoded_image(4001, 4000, "PNG"))
    with pytest.raises(ImageError):
        prepare_image(b"x" * (15 * 1024 * 1024 + 1))
