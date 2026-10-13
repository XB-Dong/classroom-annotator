import asyncio
import hmac
import math
import re
import threading
import uuid
from contextlib import asynccontextmanager

from fastapi import FastAPI, File, Form, Request, UploadFile
from fastapi.exceptions import RequestValidationError
from fastapi.responses import JSONResponse
from starlette.formparsers import MultiPartParser

from .model import Detector
from .preprocess import MAX_IMAGE_BYTES, ImageError, prepare_image
from .schemas import DetectionResponse
from .settings import Settings

MAX_BODY_BYTES = MAX_IMAGE_BYTES + 64 * 1024
# The boundary middleware rejects larger bodies before multipart parsing. Keeping
# the spool threshold above that bound prevents image data from reaching disk.
MultiPartParser.spool_max_size = MAX_BODY_BYTES + 1


def error(code, message, status, request_id=""):
    return JSONResponse({"code": code, "message": message, "requestId": request_id},
                        status_code=status, headers={"X-Request-ID": request_id})


class RequestBoundary:
    def __init__(self, app, token):
        self.app, self.token = app, token

    async def __call__(self, scope, receive, send):
        if scope["type"] != "http" or scope["path"] != "/v1/detections":
            return await self.app(scope, receive, send)
        headers = dict(scope["headers"])
        request_id = headers.get(b"x-request-id", b"").decode("ascii", "ignore")
        if not re.fullmatch(r"[A-Za-z0-9._:-]{1,128}", request_id):
            request_id = str(uuid.uuid4())
        scope.setdefault("state", {})["request_id"] = request_id
        expected = ("Bearer " + self.token).encode()
        if not self.token or not hmac.compare_digest(headers.get(b"authorization", b""), expected):
            return await error("UNAUTHORIZED", "Internal authentication required", 401, request_id)(scope, receive, send)
        content_length = headers.get(b"content-length")
        if content_length is not None:
            try:
                length = int(content_length)
                if length < 0:
                    raise ValueError
            except ValueError:
                return await error("INVALID_REQUEST", "Invalid content length", 400, request_id)(scope, receive, send)
            if length > MAX_BODY_BYTES:
                return await error("IMAGE_TOO_LARGE", "Request exceeds image size limit", 413, request_id)(scope, receive, send)
        chunks, size = [], 0
        while True:
            event = await receive()
            if event["type"] == "http.disconnect":
                return
            chunk = event.get("body", b"")
            size += len(chunk)
            if size > MAX_BODY_BYTES:
                return await error("IMAGE_TOO_LARGE", "Request exceeds image size limit", 413, request_id)(scope, receive, send)
            chunks.append(chunk)
            if not event.get("more_body", False):
                break
        body, delivered = b"".join(chunks), False
        async def replay():
            nonlocal delivered
            if not delivered:
                delivered = True
                return {"type": "http.request", "body": body, "more_body": False}
            return await receive()
        await self.app(scope, replay, send)


def create_app(settings=None, engine=None):
    settings = settings or Settings.from_env()
    slot = threading.Lock()

    @asynccontextmanager
    async def lifespan(app):
        app.state.engine = engine
        app.state.timeout_pending = False
        if engine is None and settings.model_path:
            try:
                app.state.engine = Detector(settings.model_path, settings.timeout_seconds)
            except Exception:
                app.state.engine = None
        yield

    app = FastAPI(lifespan=lifespan, docs_url=None, redoc_url=None, openapi_url=None)
    app.add_middleware(RequestBoundary, token=settings.internal_token)

    @app.exception_handler(RequestValidationError)
    async def validation_error(request, exc):
        return error("INVALID_REQUEST", "Image and numeric thresholds are required", 422,
                     getattr(request.state, "request_id", ""))

    @app.get("/health/live")
    def live():
        return {"status": "live"}

    @app.get("/health/ready")
    def ready():
        current = getattr(app.state, "engine", None)
        if (not current or not current.ready or not settings.internal_token
                or app.state.timeout_pending):
            return error("MODEL_NOT_READY", "Detection model is unavailable", 503)
        return {"status": "ready"}

    @app.post("/v1/detections", response_model=DetectionResponse)
    async def detect(request: Request, image: UploadFile = File(...),
                     confidenceThreshold: float = Form(.25), nmsThreshold: float = Form(.5)):
        request_id = request.state.request_id
        if any(not math.isfinite(x) or not 0 <= x <= 1 for x in (confidenceThreshold, nmsThreshold)):
            return error("INVALID_REQUEST", "Thresholds must be finite values between 0 and 1", 422, request_id)
        current = getattr(app.state, "engine", None)
        if not current or not current.ready:
            return error("MODEL_NOT_READY", "Detection model is unavailable", 503, request_id)
        if not slot.acquire(blocking=False):
            return error("BUSY", "Inference slot is busy", 429, request_id)
        try:
            data = await image.read(MAX_IMAGE_BYTES + 1)
            await image.close()
        except BaseException:
            slot.release()
            raise
        if len(data) > MAX_IMAGE_BYTES:
            slot.release()
            return error("IMAGE_TOO_LARGE", "Image exceeds 15 MiB", 413, request_id)
        def infer():
            try:
                prepare_image(data)
                return current.infer(data, confidenceThreshold, nmsThreshold)
            finally:
                slot.release()
        future = asyncio.get_running_loop().run_in_executor(None, infer)
        try:
            result = await asyncio.wait_for(asyncio.shield(future), settings.timeout_seconds)
            return JSONResponse(DetectionResponse.model_validate(result).model_dump(),
                                headers={"X-Request-ID": request_id})
        except asyncio.TimeoutError:
            app.state.timeout_pending = True
            def finished(task):
                app.state.timeout_pending = False
                if not task.cancelled():
                    task.exception()
            future.add_done_callback(finished)
            return error("INFERENCE_TIMEOUT", "Inference exceeded its deadline", 504, request_id)
        except asyncio.CancelledError:
            future.add_done_callback(lambda task: task.exception() if not task.cancelled() else None)
            raise
        except ImageError:
            return error("INVALID_IMAGE", "Image must be a valid JPEG or PNG within 15 MiB and 16 megapixels", 422, request_id)
        except Exception:
            return error("INFERENCE_FAILED", "Detection inference failed", 503, request_id)
    return app
