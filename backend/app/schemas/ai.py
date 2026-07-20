from __future__ import annotations

from typing import Optional

from pydantic import BaseModel, Field


class ChatRequest(BaseModel):
    message: str = Field(min_length=1, max_length=10000)
    # Текст для контекста (например, развёрнутые фрагменты выделения)
    context_text: Optional[str] = Field(default=None, max_length=50000)


class ChatReply(BaseModel):
    reply: str


class ExplainRequest(BaseModel):
    # Как правило, это выделенный фрагмент
    text: str = Field(min_length=1, max_length=50000)
    question: Optional[str] = Field(default=None, max_length=10000)


class ExplainReply(BaseModel):
    reply: str


class TranslateRequest(BaseModel):
    text: str = Field(min_length=1, max_length=50000)
    target_lang: str = Field(default="ru", min_length=2, max_length=16)


class TranslateReply(BaseModel):
    translation: str
    detected_source: Optional[str] = None

