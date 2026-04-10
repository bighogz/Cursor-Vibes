from functools import lru_cache

from langchain_core.prompts import ChatPromptTemplate
from langchain_ollama import ChatOllama

from .schemas import AnomalyInput

DEFAULT_MODEL = "qwen3.5"

SYSTEM_PROMPT = """\
You explain insider-selling anomalies using only the provided structured data.

Rules:
- Do not speculate about illegality.
- Do not speculate about insider intent.
- Do not predict future stock performance.
- Do not give investment advice.
- If context is limited, say so plainly.
- If fields are missing or marked N/A, mention that in caveats instead of inferring details.
- Be concise and concrete.
- Return only valid JSON.
- Do not wrap the JSON in markdown.
- Do not include any text before or after the JSON.

Return valid JSON with exactly these keys:
- "summary": string
- "drivers": string[]
- "caveats": string[]"""

HUMAN_TEMPLATE = """\
Ticker: {ticker}
Company: {company_name}
Sector: {sector}
Anomaly score: {anomaly_score:.2f}
Z-score: {z_score}
Coverage window: {coverage_window}
Trend summary: {trend_summary}

Recent insider events:
{events_block}

Source notes: {source_notes}"""


def _format_events(req: AnomalyInput) -> str:
    if not req.recent_events:
        return "(none)"

    lines = []
    for e in req.recent_events:
        line = f"- date: {e.date}; insider: {e.insider_name}"
        if e.role:
            line += f"; role: {e.role}"
        if e.shares_sold is not None:
            line += f"; shares_sold: {e.shares_sold:,.0f}"
        if e.value_usd is not None:
            line += f"; value_usd: ${e.value_usd:,.0f}"
        lines.append(line)
    return "\n".join(lines)


@lru_cache(maxsize=4)
def build_chain(model: str = DEFAULT_MODEL):
    llm = ChatOllama(model=model, temperature=0, format="json")
    prompt = ChatPromptTemplate.from_messages(
        [("system", SYSTEM_PROMPT), ("human", HUMAN_TEMPLATE)]
    )
    return prompt | llm


async def explain_anomaly(req: AnomalyInput, model: str = DEFAULT_MODEL):
    chain = build_chain(model)
    return await chain.ainvoke(
        {
            "ticker": req.ticker,
            "company_name": req.company_name,
            "sector": req.sector or "Unknown",
            "anomaly_score": req.anomaly_score,
            "z_score": f"{req.z_score:.2f}" if req.z_score is not None else "N/A",
            "coverage_window": req.coverage_window or "N/A",
            "trend_summary": req.trend_summary or "N/A",
            "events_block": _format_events(req),
            "source_notes": req.source_notes or "N/A",
        }
    )
