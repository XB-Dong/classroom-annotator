FROM python:3.12.14-slim-bookworm
ENV PYTHONDONTWRITEBYTECODE=1 PYTHONUNBUFFERED=1 OMP_NUM_THREADS=1 OPENBLAS_NUM_THREADS=1
WORKDIR /app
COPY ai-service/requirements.lock /app/requirements.lock
RUN pip install --no-cache-dir --require-hashes -r /app/requirements.lock \
    && groupadd --gid 10001 ai && useradd --uid 10001 --gid ai --no-create-home ai
COPY ai-service/src /app/src
COPY ai-service/model-manifest.json ai-service/MODEL-LICENSE /app/
ENV PYTHONPATH=/app/src AI_MODEL_PATH=/models/yolov9_n_wholebody34_0100_1x3x640x640.onnx
USER 10001:10001
EXPOSE 8090
HEALTHCHECK --interval=20s --timeout=3s --retries=3 CMD python -c "import urllib.request; urllib.request.urlopen('http://127.0.0.1:8090/health/ready', timeout=2)"
CMD ["uvicorn", "classroom_ai.app:create_app", "--factory", "--host", "0.0.0.0", "--port", "8090", "--workers", "1", "--no-access-log"]
