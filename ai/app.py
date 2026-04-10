import os

from fastapi import FastAPI, HTTPException

from .chains import explain_anomaly
from .schemas import AnomalyExplanation, AnomalyInput

app = FastAPI(title="Vibes AI", version="0.1.0")

MODEL = os.getenv("OLLAMA_MODEL", "qwen3.5")


@app.post("/explain-anomaly", response_model=AnomalyExplanation)
async def post_explain_anomaly(req: AnomalyInput) -> AnomalyExplanation:
    try:
        return await explain_anomaly(req, model=MODEL)
    except Exception as exc:
        raise HTTPException(status_code=502, detail=f"LLM error: {exc}") from exc
