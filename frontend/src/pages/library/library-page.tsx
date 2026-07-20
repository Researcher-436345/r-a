import { Link } from '@tanstack/react-router';
import { FilePlus2, LoaderCircle, RotateCcw } from 'lucide-react';
import { useEffect, useState } from 'react';

import { fetchLibrary, retryPdf, type LibraryItem } from '../../features/library/api';
import { ApiError } from '../../shared/api/client';

const STATUS_LABELS: Record<string, string> = {
  ready: 'PDF готов',
  processing: 'Обрабатывается',
  uploading: 'Загрузка',
  failed: 'Ошибка PDF',
};

export function LibraryPage() {
  const [items, setItems] = useState<LibraryItem[]>([]);
  const [total, setTotal] = useState(0);
  const [error, setError] = useState<string | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [retryingId, setRetryingId] = useState<string | null>(null);

  const loadLibrary = async () => {
    setIsLoading(true);
    setError(null);
    try {
      const data = await fetchLibrary();
      setItems(data.items);
      setTotal(data.total);
    } catch (err) {
      setError(err instanceof ApiError ? err.detail : 'Не удалось загрузить библиотеку');
    } finally {
      setIsLoading(false);
    }
  };

  useEffect(() => {
    void loadLibrary();
  }, []);

  const handleRetry = async (paperId: string) => {
    setRetryingId(paperId);
    setError(null);
    try {
      await retryPdf(paperId);
      await loadLibrary();
    } catch (err) {
      setError(err instanceof ApiError ? err.detail : 'Не удалось перезапустить обработку PDF');
    } finally {
      setRetryingId(null);
    }
  };

  return (
    <div className="library-page">
      <div className="library-page__header">
        <div>
          <h1>Моя библиотека</h1>
          <p>{total > 0 ? `${total} статей` : 'Пока пусто — добавьте первую статью'}</p>
        </div>
        <Link className="library-page__add" to="/library/add">
          <FilePlus2 size={16} strokeWidth={2} />
          Добавить статью
        </Link>
      </div>

      {isLoading ? (
        <div className="library-page__state">
          <LoaderCircle className="spin" size={18} />
          Загружаем…
        </div>
      ) : null}

      {error ? <div className="library-page__error">{error}</div> : null}

      {!isLoading && !error && items.length === 0 ? (
        <div className="library-page__empty">
          <p>Здесь будут ваши статьи.</p>
          <Link to="/library/add">Добавить arXiv / DOI / PDF</Link>
        </div>
      ) : null}

      <div className="library-list">
        {items.map((item) => {
          const authors = item.paper.authors.map((author) => author.name).join(', ');
          const version = item.paper.latest_version;
          const status = version?.status ?? 'processing';
          const statusLabel = STATUS_LABELS[status] ?? status;

          return (
            <article className="library-card" key={item.id}>
              <div className="library-card__body">
                <Link className="library-card__title" to="/reader/$paperId" params={{ paperId: item.paper.id }}>
                  {item.paper.title}
                </Link>
                {authors ? <div className="library-card__authors">{authors}</div> : null}
                <div className="library-card__meta">
                  {item.paper.year ? <span>{item.paper.year}</span> : null}
                  {item.paper.arxiv_id ? <span>arXiv:{item.paper.arxiv_id}</span> : null}
                  {item.paper.doi ? <span>DOI:{item.paper.doi}</span> : null}
                  <span
                    className={`library-card__status library-card__status--${status}`}
                    title={version?.error_message ?? undefined}
                  >
                    {statusLabel}
                  </span>
                  {status === 'failed' ? (
                    <button
                      type="button"
                      className="library-card__retry"
                      disabled={retryingId === item.paper.id}
                      onClick={() => void handleRetry(item.paper.id)}
                    >
                      <RotateCcw size={14} strokeWidth={2} />
                      {retryingId === item.paper.id ? 'Повтор…' : 'Повторить'}
                    </button>
                  ) : null}
                </div>
                {status === 'failed' && version?.error_message ? (
                  <p className="library-card__error">{version.error_message}</p>
                ) : null}
                {item.paper.abstract ? (
                  <p className="library-card__abstract">{item.paper.abstract}</p>
                ) : null}
              </div>
            </article>
          );
        })}
      </div>
    </div>
  );
}
