import threading
import time
from concurrent.futures import ThreadPoolExecutor

from fastapi.testclient import TestClient
from classroom_ai.app import create_app
from classroom_ai.settings import Settings
from test_coordinates import encoded_image

TOKEN = "internal-test-token-with-sufficient-entropy"
AUTH = {"Authorization": f"Bearer {TOKEN}", "X-Request-ID": "test-request"}


class Engine:
    ready = True
    def infer(self, data, confidence, nms):
        return {"contractVersion": "1.0", "modelId": "test-engine",
                "pipelineVersion": "classroom-head-v1-fullframe-2x2-15",
                "modelSHA256": "a" * 64, "image": {"width": 100, "height": 50},
                "detections": [], "timingMs": 1.0}


def client(engine=None):
    return TestClient(create_app(Settings(internal_token=TOKEN), engine))


def test_liveness_and_readiness_are_distinct():
    with client() as api:
        assert api.get("/health/live").status_code == 200
        assert api.get("/health/ready").status_code == 503


def test_internal_auth_is_required_before_reading_image():
    with client(Engine()) as api:
        response = api.post("/v1/detections", content=b"bad image")
        assert response.status_code == 401
        assert response.json()["code"] == "UNAUTHORIZED"


def test_multipart_success_with_original_dimensions_and_request_id():
    with client(Engine()) as api:
        response = api.post("/v1/detections", headers=AUTH,
                            files={"image": ("frame.jpg", encoded_image(100, 50), "image/jpeg")},
                            data={"confidenceThreshold": ".25", "nmsThreshold": ".5"})
        assert response.status_code == 200
        assert response.json()["image"] == {"width": 100, "height": 50}
        assert response.headers["X-Request-ID"] == "test-request"


def test_bounded_payload_is_rejected_before_multipart_parse():
    with client(Engine()) as api:
        response = api.post("/v1/detections", headers=AUTH,
                            content=b"x" * (15 * 1024 * 1024 + 65537))
        assert response.status_code == 413
        assert response.json()["code"] == "IMAGE_TOO_LARGE"


def test_image_part_limit_excludes_multipart_overhead():
    with client(Engine()) as api:
        response=api.post("/v1/detections",headers=AUTH,
                          files={"image":("frame.jpg",b"x"*(15*1024*1024+1))})
        assert response.status_code==413


def test_chunked_body_limit_is_enforced_without_content_length():
    import asyncio
    app=create_app(Settings(internal_token=TOKEN),Engine())
    calls=[]
    events=[]
    async def receive():
        calls.append(1)
        return {"type":"http.request","body":b"x"*(8*1024*1024),"more_body":True}
    async def send(event):
        events.append(event)
    scope={"type":"http","asgi":{"version":"3.0"},"method":"POST","path":"/v1/detections",
           "raw_path":b"/v1/detections","query_string":b"","http_version":"1.1",
           "headers":[(b"authorization",f"Bearer {TOKEN}".encode())]}
    asyncio.run(app(scope,receive,send))
    assert len(calls)==2
    assert events[0]["status"]==413


def test_model_not_ready_has_safe_error_without_path():
    with client() as api:
        response = api.post("/v1/detections", headers=AUTH,
                            files={"image": ("frame.jpg", encoded_image(100, 50))})
        assert response.status_code == 503
        assert response.json()["code"] == "MODEL_NOT_READY"


def test_invalid_image_and_nonfinite_threshold_return_422():
    with client(Engine()) as api:
        response = api.post("/v1/detections", headers=AUTH, files={"image": ("bad.jpg", b"invalid")})
        assert response.status_code == 422
        response = api.post("/v1/detections", headers=AUTH,
                            files={"image": ("frame.jpg", encoded_image(100, 50))},
                            data={"confidenceThreshold": "nan"})
        assert response.status_code == 422


def test_busy_slot_rejects_second_request():
    entered, release = threading.Event(), threading.Event()
    class BlockingEngine(Engine):
        def infer(self, *args):
            entered.set()
            assert release.wait(5)
            return super().infer(*args)
    with client(BlockingEngine()) as api, ThreadPoolExecutor(max_workers=1) as pool:
        first = pool.submit(api.post, "/v1/detections", headers=AUTH,
                            files={"image": ("frame.jpg", encoded_image(100, 50))})
        assert entered.wait(2)
        try:
            second = api.post("/v1/detections", headers=AUTH,
                              files={"image": ("frame.jpg", encoded_image(100, 50))})
            assert second.status_code == 429
            assert second.json()["code"] == "BUSY"
        finally:
            release.set()
        assert first.result().status_code == 200


def test_timeout_keeps_slot_and_marks_not_ready_until_worker_really_ends():
    entered, release = threading.Event(), threading.Event()
    class SlowEngine(Engine):
        def infer(self, *args):
            entered.set()
            release.wait(3)
            return super().infer(*args)
    settings = Settings(internal_token=TOKEN, timeout_seconds=.05)
    with TestClient(create_app(settings, SlowEngine())) as api:
        try:
            response = api.post("/v1/detections", headers=AUTH,
                                files={"image": ("frame.jpg", encoded_image(100, 50))})
            assert entered.is_set()
            assert response.status_code == 504
            assert api.get("/health/ready").status_code == 503
            response = api.post("/v1/detections", headers=AUTH,
                                files={"image": ("frame.jpg", encoded_image(100, 50))})
            assert response.status_code == 429
        finally:
            release.set()
        for _ in range(50):
            if api.get("/health/ready").status_code == 200:
                break
            time.sleep(.01)
        assert api.get("/health/ready").status_code == 200


def test_png_is_supported_and_image_spool_does_not_roll_to_disk(monkeypatch):
    import tempfile
    def forbidden_disk(*args, **kwargs):
        raise AssertionError("Image multipart spool must stay in memory")
    monkeypatch.setattr(tempfile, "TemporaryFile", forbidden_disk)
    with client(Engine()) as api:
        response = api.post("/v1/detections", headers=AUTH,
                            files={"image": ("frame.png", encoded_image(100, 50, "PNG"), "image/png")})
        assert response.status_code == 200
        data = encoded_image(100,50) + b"\0" * (2 * 1024 * 1024)
        response = api.post("/v1/detections", headers=AUTH, files={"image": ("frame.jpg", data)})
        assert response.status_code == 200


def test_internal_exception_is_safe_and_releases_slot():
    class BrokenEngine(Engine):
        def infer(self, *args):
            raise RuntimeError("secret path /private/model")
    with client(BrokenEngine()) as api:
        for _ in range(2):
            response = api.post("/v1/detections", headers=AUTH,
                                files={"image": ("frame.jpg", encoded_image(100,50))})
            assert response.status_code == 503
            assert response.json()["code"] == "INFERENCE_FAILED"
            assert "private" not in response.text
