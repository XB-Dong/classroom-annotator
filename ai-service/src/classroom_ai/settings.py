from dataclasses import dataclass
import os


@dataclass(frozen=True)
class Settings:
    internal_token: str = ""
    model_path: str = ""
    timeout_seconds: float = 30

    @classmethod
    def from_env(cls):
        return cls(internal_token=os.getenv("AI_INTERNAL_TOKEN", ""),
                   model_path=os.getenv("AI_MODEL_PATH", ""))
