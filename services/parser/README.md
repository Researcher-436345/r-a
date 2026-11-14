# Парсер PDF

FastAPI-сервис: `POST /v1/parse` возвращает Markdown, обычный текст, страницы и фрагменты.

Режим выбирает `PARSER_ENGINE`: `pymupdf` по умолчанию; `auto` использует Docling,
если он установлен, с переходом к PyMuPDF при ошибке. OCR задаёт `PARSER_OCR`.
Общие значения по умолчанию находятся в корневом `settings.py`; переменные
окружения имеют приоритет. Подробности: [`docs/SETTINGS.md`](../../docs/SETTINGS.md).

Локальный запуск из корня репозитория после установки зависимостей:

```bash
PYTHONPATH=. python3 -m uvicorn app.main:app --app-dir services/parser --env-file .env --host 127.0.0.1 --port 8091
```
