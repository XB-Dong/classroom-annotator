"""Runs inside the actual image; no fixture ONNX or external HTTP is accepted."""
import hashlib
import io
import json
import os
import platform
import time
import urllib.error
import urllib.request

from PIL import Image, ImageDraw

assert platform.machine() == {"amd64": "x86_64", "arm64": "aarch64"}[os.environ["P1_EXPECTED_ARCH"]]

started = time.monotonic()
ready = False
for attempt in range(90):
    try:
        with urllib.request.urlopen("http://127.0.0.1:8090/health/ready", timeout=2) as response:
            assert response.status == 200
            ready = True
            break
    except (OSError, urllib.error.URLError):
        time.sleep(1)
assert ready, "Actual model container did not become ready"
image = Image.new("RGB", (320, 240), (230, 235, 240))
ImageDraw.Draw(image).ellipse((100, 40, 150, 90), fill=(85, 55, 25))
output = io.BytesIO()
image.save(output, "JPEG")
jpeg = output.getvalue()
boundary = "p1-synthetic-model-boundary"
body = (f'--{boundary}\r\nContent-Disposition: form-data; name="image"; filename="synthetic.jpg"\r\n'
        'Content-Type: image/jpeg\r\n\r\n').encode() + jpeg + f"\r\n--{boundary}--\r\n".encode()
inference_start = time.monotonic()
request = urllib.request.Request("http://127.0.0.1:8090/v1/detections", data=body, headers={
    "Authorization": "Bearer synthetic-ci-container-token-with-enough-entropy",
    "Content-Type": f"multipart/form-data; boundary={boundary}"})
with urllib.request.urlopen(request, timeout=45) as response:
    assert response.status == 200
    value = json.load(response)
assert value["modelSHA256"] == "a5ece478fc918787258afccd0b313e54319f8f39aeb1ceca11f27dbd269e78bc"
assert value["image"] == {"width": 320, "height": 240}
assert value["contractVersion"] == "1.0.0"
assert value["modelId"] == "yolov9_n_wholebody34_0100_1x3x640x640"
assert value["pipelineVersion"] == "classroom-head-v1-fullframe-2x2-15"
for detection in value["detections"]:
    x, y, width, height = detection["bbox"]
    assert detection["classId"] == 7 and 0 <= x < x + width <= 320 and 0 <= y < y + height <= 240
print(json.dumps({"architecture": platform.machine(), "modelSHA256": value["modelSHA256"],
    "jpegSHA256": hashlib.sha256(jpeg).hexdigest(), "inputKind": "synthetic JPEG",
    "candidateCount": len(value["detections"]), "containerInferenceMs": round((time.monotonic() - inference_start) * 1000, 3),
    "containerReadinessAndInferenceMs": round((time.monotonic() - started) * 1000, 3),
    "macHardwareVerification": "not_run", "accuracyVerification": "not_run"}, indent=2))
