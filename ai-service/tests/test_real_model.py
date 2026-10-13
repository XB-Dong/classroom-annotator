"""Actual external ONNX/JPEG/PNG checks, distinct from fixture coordinate tests."""
import hashlib
import io
import json
import os
import sys
import time
from pathlib import Path

import pytest
from fastapi.testclient import TestClient
from PIL import Image

from classroom_ai.app import create_app
from classroom_ai.model import MODEL_SHA256, Detector
from classroom_ai.settings import Settings

pytestmark = pytest.mark.real_model


def artifact_and_samples():
    model = os.getenv("AI_TEST_MODEL_PATH")
    directory = os.getenv("AI_TEST_SAMPLE_DIRECTORY")
    if not model or not directory:
        pytest.skip("Actual external model/sample paths are not configured; not_run")
    samples = sorted(p for p in Path(directory).iterdir() if p.suffix.lower() in {".jpg", ".jpeg", ".png"})
    assert len(samples) >= 8, "At least8 actual images required"
    return model, samples[:8]


def test_pinned_real_model_with_eight_actual_jpegs_and_png_http():
    model, samples = artifact_and_samples()
    assert Path(model).stat().st_size == 3221705
    assert hashlib.sha256(Path(model).read_bytes()).hexdigest() == MODEL_SHA256
    detector = Detector(model)
    token = "real-test-internal-token-with-enough-entropy"
    settings = Settings(model_path=model, internal_token=token)
    evidence = []
    with TestClient(create_app(settings, detector)) as api:
        assert api.get("/health/ready").status_code == 200
        for index, path in enumerate(samples):
            with Image.open(path) as image:
                width,height=image.size
                buffer=io.BytesIO()
                image.convert("RGB").save(buffer,"JPEG",quality=95)
            jpeg=buffer.getvalue()
            request_start=time.perf_counter()
            response=api.post("/v1/detections", headers={"Authorization":f"Bearer {token}"},
                              files={"image":("frame.jpg",jpeg,"image/jpeg")})
            request_ms=(time.perf_counter()-request_start)*1000
            assert response.status_code==200
            result=response.json()
            assert result["image"]=={"width":width,"height":height}
            assert result["modelSHA256"]==MODEL_SHA256
            assert result["pipelineVersion"]=="classroom-head-v1-fullframe-2x2-15"
            for detection in result["detections"]:
                x,y,w,h=detection["bbox"]
                assert 0<=x<x+w<=width and 0<=y<y+h<=height
                assert detection["classId"]==7
            evidence.append({"sampleId":f"sample-{index+1:02d}","jpegSHA256":hashlib.sha256(jpeg).hexdigest(),
                             "httpTestClientMs":round(request_ms,3),
                             "source":"actual image; PNG source converted to JPEG in memory" if path.suffix.lower()==".png" else "actual JPEG",
                             **result})
        first=samples[0]
        response=api.post("/v1/detections",headers={"Authorization":f"Bearer {token}"},
                          files={"image":("frame.png",first.read_bytes(),"image/png")})
        assert response.status_code==200
        assert response.json()["modelSHA256"]==MODEL_SHA256
    evidence_path=os.getenv("AI_TEST_EVIDENCE_PATH")
    if evidence_path:
        resource={}
        if sys.platform=="win32":
            import ctypes
            from ctypes import wintypes
            class Counters(ctypes.Structure):
                _fields_=[("cb",wintypes.DWORD),("PageFaultCount",wintypes.DWORD),
                          ("PeakWorkingSetSize",ctypes.c_size_t),("WorkingSetSize",ctypes.c_size_t),
                          ("QuotaPeakPagedPoolUsage",ctypes.c_size_t),("QuotaPagedPoolUsage",ctypes.c_size_t),
                          ("QuotaPeakNonPagedPoolUsage",ctypes.c_size_t),("QuotaNonPagedPoolUsage",ctypes.c_size_t),
                          ("PagefileUsage",ctypes.c_size_t),("PeakPagefileUsage",ctypes.c_size_t)]
            counters=Counters();counters.cb=ctypes.sizeof(counters)
            ctypes.windll.kernel32.GetCurrentProcess.restype=wintypes.HANDLE
            handle=ctypes.windll.kernel32.GetCurrentProcess()
            ctypes.windll.psapi.GetProcessMemoryInfo.argtypes=[wintypes.HANDLE,ctypes.POINTER(Counters),wintypes.DWORD]
            if ctypes.windll.psapi.GetProcessMemoryInfo(handle,ctypes.byref(counters),counters.cb):
                resource={"peakWorkingSetMiB":round(counters.PeakWorkingSetSize/(1024*1024),3),
                          "scope":"entire pytest process including unit tests, not isolated service RSS"}
        Path(evidence_path).write_text(json.dumps({"samples":evidence,"resource":resource},indent=2),encoding="utf-8")


def test_model_signature_gate_rejects_unapproved_bytes(tmp_path):
    path=tmp_path/"unapproved.onnx"
    path.write_bytes(b"invalid")
    with pytest.raises(ValueError,match="approved manifest"):
        Detector(path)
