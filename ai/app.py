import json
import logging
import os

from fastapi import FastAPI, HTTPException

from .chains import explain_anomaly
from .schemas import AnomalyExplanation, AnomalyInput

logger = logging.getLogger(__name__)

app = FastAPI(title="Vibes AI", version="0.1.0")

MODEL = os.getenv("OLLAMA_MODEL", "qwen3.5")


@app.post("/explain-anomaly", response_model=AnomalyExplanation)
async def post_explain_anomaly(req: AnomalyInput) -> AnomalyExplanation:
    logger.info("explain-anomaly called ticker=%s model=%s", req.ticker, MODEL)

    try:
        raw = await explain_anomaly(req, model=MODEL)
    except Exception as exc:
        logger.exception("LLM error for ticker=%s model=%s", req.ticker, MODEL)
        raise HTTPException(status_code=502, detail=f"LLM error: {exc}") from exc

    content = (raw.content if hasattr(raw, "content") else str(raw)).strip()

    try:
        parsed = json.loads(content)
        if not isinstance(parsed, dict):
            raise ValueError("Model did not return a JSON object")
        return AnomalyExplanation(**parsed)
    except Exception as exc:
        logger.warning(
            "Invalid JSON from model for ticker=%s model=%s content=%r",
            req.ticker,
            MODEL,
            content[:1000],
        )
        raise HTTPException(
            status_code=500,
            detail="Model returned invalid structured output",
        ) from exc
