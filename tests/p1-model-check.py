"""True pinned ONNX execution on synthetic pixels; this is not accuracy evidence."""
import hashlib
import io
import json
import os
import time
from pathlib import Path

from fastapi.testclient import TestClient
from PIL import Image, ImageDraw

from classroom_ai.app import create_app
from classroom_ai.model import Detector, MODEL_SHA256
from classroom_ai.settings import Settings

model = Path(os.environ["P1_MODEL_PATH"])
assert model.stat().st_size == 3221705
assert hashlib.sha256(model.read_bytes()).hexdigest() == MODEL_SHA256
detector = Detector(str(model))
token = "synthetic-ci-internal-token-with-enough-entropy"
records = []
with TestClient(create_app(Settings(model_path=str(model), internal_token=token), detector)) as client:
    assert client.get("/health/ready").status_code == 200
    for width, height in [(320, 240), (240, 320), (640, 640), (1920, 1080)]:
        image = Image.new("RGB", (width, height), (235, 232, 225))
        draw = ImageDraw.Draw(image)
        draw.ellipse((width // 3, height // 4, width // 2, height // 2), fill=(120, 75, 50))
        buffer = io.BytesIO()
        image.save(buffer, "JPEG")
        started = time.monotonic()
        response = client.post("/v1/detections", headers={"Authorization": f"Bearer {token}"},
                               files={"image": ("synthetic.jpg", buffer.getvalue(), "image/jpeg")})
        assert response.status_code == 200, response.text
        value = response.json()
        assert value["image"] == {"width": width, "height": height}
        assert value["modelSHA256"] == MODEL_SHA256
        for detection in value["detections"]:
            x, y, w, h = detection["bbox"]
            assert detection["classId"] == 7 and 0 <= x < x + w <= width and 0 <= y < y + h <= height
        records.append({"width": width, "height": height, "jpegSHA256": hashlib.sha256(buffer.getvalue()).hexdigest(),
                        "elapsedMs": round((time.monotonic() - started) * 1000, 3), "candidateCount": len(value["detections"])})
Path(os.environ["P1_MODEL_EVIDENCE"]).write_text(json.dumps({"modelSHA256": MODEL_SHA256,
    "inputKind": "synthetic JPEG", "accuracyVerification": "not_run", "records": records}, indent=2), encoding="utf-8")
print("P1 real ONNX synthetic execution passed; classroom accuracy is not assessed.")
