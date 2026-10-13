import hashlib
import threading
import time
from pathlib import Path

import onnxruntime as ort

from .pipeline import infer_pipeline

MODEL_SHA256 = "a5ece478fc918787258afccd0b313e54319f8f39aeb1ceca11f27dbd269e78bc"
MODEL_ID = "yolov9_n_wholebody34_0100_1x3x640x640"


class Detector:
    ready = False

    def __init__(self, model_path, timeout_seconds=30):
        self.timeout_seconds = timeout_seconds
        path = Path(model_path)
        if path.stat().st_size != 3_221_705 or hashlib.sha256(path.read_bytes()).hexdigest() != MODEL_SHA256:
            raise ValueError("Model artifact does not match the approved manifest")
        options = ort.SessionOptions()
        options.intra_op_num_threads = 1
        options.inter_op_num_threads = 1
        options.execution_mode = ort.ExecutionMode.ORT_SEQUENTIAL
        options.add_session_config_entry("session.intra_op.allow_spinning", "0")
        options.add_session_config_entry("session.inter_op.allow_spinning", "0")
        self.session = ort.InferenceSession(str(path), sess_options=options,
                                           providers=["CPUExecutionProvider"])
        inputs, outputs = self.session.get_inputs(), self.session.get_outputs()
        if (len(inputs) != 1 or inputs[0].shape != [1, 3, 640, 640]
                or inputs[0].type != "tensor(float)" or len(outputs) != 1
                or outputs[0].shape != [1, 38, 8400]):
            raise ValueError("Model signature does not match the approved manifest")
        self.input_name = inputs[0].name
        self.ready = True

    def infer(self, data, confidence, nms):
        start = time.perf_counter()
        run_options = ort.RunOptions()
        timer = threading.Timer(self.timeout_seconds, lambda: setattr(run_options, "terminate", True))
        timer.daemon = True
        timer.start()
        try:
            def predict(tensor):
                if time.perf_counter() - start >= self.timeout_seconds:
                    raise TimeoutError("Inference deadline exceeded")
                return self.session.run(None, {self.input_name: tensor}, run_options=run_options)[0]
            result = infer_pipeline(data, predict, confidence, nms)
        except Exception:
            if time.perf_counter() - start >= self.timeout_seconds:
                raise TimeoutError("Inference deadline exceeded") from None
            raise
        finally:
            timer.cancel()
        return {"contractVersion": "1.0.0", "modelId": MODEL_ID, "modelSHA256": MODEL_SHA256,
                **result, "timingMs": round((time.perf_counter() - start) * 1000, 3)}
