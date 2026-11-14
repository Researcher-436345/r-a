FROM python:3.11-slim-bookworm

RUN apt-get update && apt-get install -y --no-install-recommends \
    libgl1 \
    libglib2.0-0 \
    curl \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY services/parser/requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

COPY services/parser/app ./app
COPY settings.py ./

ENV PARSER_HTTP_ADDR=0.0.0.0:8091

EXPOSE 8091
CMD ["sh", "-c", "uvicorn app.main:app --host 0.0.0.0 --port 8091"]
