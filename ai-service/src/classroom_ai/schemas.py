from pydantic import BaseModel, Field
from typing import Literal


class ImageSize(BaseModel):
    width: int = Field(gt=0)
    height: int = Field(gt=0)


class Detection(BaseModel):
    bbox: list[float] = Field(min_length=4, max_length=4)
    confidence: float = Field(ge=0, le=1)
    classId: Literal[7]
    clipped: bool = False
    tileBoundary: bool = False


class DetectionResponse(BaseModel):
    contractVersion: str
    modelId: str
    modelSHA256: str
    pipelineVersion: str
    mode: str = "fullframe"
    image: ImageSize
    detections: list[Detection]
    timingMs: float = Field(ge=0)
