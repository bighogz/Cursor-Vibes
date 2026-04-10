from langchain_core.prompts import ChatPromptTemplate
from langchain_ollama import ChatOllama

from .schemas import AnomalyInput

SYSTEM_PROMPT = """\
You explain insider-selling anomalies using only the provided structured data.

Rules:
- Do not speculate about illegality.
- Do not speculate about insider intent.
- Do not predict future stock performance.
- Do not give investment advice.
- If context is limited, say so plainly.
- Be concise and concrete.

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
        parts = [e.date, e.insider_name]
        if e.role:
            parts.append(f"({e.role})")
        if e.shares_sold is not None:
            parts.append(f"{e.shares_sold:,.0f} shares")
        if e.value_usd is not None:
            parts.append(f"${e.value_usd:,.0f}")
        lines.append(" | ".join(parts))
    return "\n".join(lines)


def build_chain(model: str = "qwen3.5"):
    llm = ChatOllama(model=model, temperature=0, format="json")
    prompt = ChatPromptTemplate.from_messages(
        [("system", SYSTEM_PROMPT), ("human", HUMAN_TEMPLATE)]
    )
    return prompt | llm


async def explain_anomaly(req: AnomalyInput, model: str = "qwen3.5"):
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
