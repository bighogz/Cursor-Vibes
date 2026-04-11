from typing import List, Optional

from pydantic import BaseModel, Field


class InsiderEvent(BaseModel):
    date: str
    insider_name: str
    role: Optional[str] = None
    shares_sold: Optional[float] = None
    value_usd: Optional[float] = None


class AnomalyInput(BaseModel):
    ticker: str
    company_name: str
    sector: Optional[str] = None
    composite_score: float = 0.0
    volume_z_score: Optional[float] = None
    breadth_z_score: Optional[float] = None
    acceleration_score: Optional[float] = None
    unique_insiders: Optional[int] = None
    trend_summary: Optional[str] = None
    coverage_window: Optional[str] = None
    source_notes: Optional[str] = None
    recent_events: List[InsiderEvent] = Field(default_factory=list)


class AnomalyExplanation(BaseModel):
    summary: str
    drivers: List[str]
    caveats: List[str]
