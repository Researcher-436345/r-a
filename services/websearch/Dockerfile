FROM python:3.11-slim-bookworm

RUN apt-get update && apt-get install -y --no-install-recommends curl \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt
COPY app ./app

ENV WEBSEARCH_HTTP_ADDR=0.0.0.0:8092
EXPOSE 8092
CMD ["sh", "-c", "uvicorn app.main:app --host 0.0.0.0 --port 8092"]
