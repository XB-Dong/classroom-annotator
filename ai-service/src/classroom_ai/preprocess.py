import io
import warnings
from dataclasses import dataclass

import numpy as np
from PIL import Image, UnidentifiedImageError

MAX_IMAGE_BYTES = 15 * 1024 * 1024
MAX_PIXELS = 16_000_000


class ImageError(ValueError):
    pass


@dataclass(frozen=True)
class Geometry:
    width: int
    height: int
    scale_x: float
    scale_y: float
    pad_x: int
    pad_y: int


def prepare_image(data):
    if not data or len(data) > MAX_IMAGE_BYTES:
        raise ImageError("Image must be nonempty and at most 15 MiB")
    try:
        with warnings.catch_warnings():
            warnings.simplefilter("error", Image.DecompressionBombWarning)
            with Image.open(io.BytesIO(data)) as source:
                if source.format not in {"JPEG", "PNG"}:
                    raise ImageError("Only JPEG and PNG images are supported")
                width, height = source.size
                if width < 1 or height < 1 or width * height > MAX_PIXELS:
                    raise ImageError("Image exceeds 16 megapixels")
                # Extracted JPEG dimensions define coordinates; do not apply EXIF rotation.
                source.load()
                rgb = source.convert("RGB")
        return prepare_rgb(rgb)
    except (UnidentifiedImageError, OSError, Image.DecompressionBombWarning,
            Image.DecompressionBombError, ValueError) as exc:
        if isinstance(exc, ImageError):
            raise
        raise ImageError("Image decoding failed") from None


def prepare_rgb(rgb):
    width, height = rgb.size
    ratio = min(640 / width, 640 / height)
    resized_width = max(1, round(width * ratio))
    resized_height = max(1, round(height * ratio))
    pad_x = (640 - resized_width) // 2
    pad_y = (640 - resized_height) // 2
    canvas = Image.new("RGB", (640, 640), (114, 114, 114))
    canvas.paste(rgb.resize((resized_width, resized_height), Image.Resampling.BILINEAR),
                 (pad_x, pad_y))
    tensor = np.asarray(canvas, dtype=np.float32).transpose(2, 0, 1)[None] / 255.0
    return np.ascontiguousarray(tensor), Geometry(width, height,
        resized_width / width, resized_height / height, pad_x, pad_y)
