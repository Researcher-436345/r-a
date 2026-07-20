import { Link, useNavigate } from '@tanstack/react-router';
import { useState, type FormEvent } from 'react';

import { addByArxiv, addByDoi, uploadPdf } from '../../features/library/api';
import { ApiError } from '../../shared/api/client';

type Mode = 'arxiv' | 'doi' | 'pdf';

export function AddPaperPage() {
  const navigate = useNavigate();
  const [mode, setMode] = useState<Mode>('arxiv');
  const [arxivId, setArxivId] = useState('');
  const [doi, setDoi] = useState('');
  const [file, setFile] = useState<File | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [isSubmitting, setIsSubmitting] = useState(false);

  const onSubmit = async (event: FormEvent) => {
    event.preventDefault();
    setError(null);
    setIsSubmitting(true);

    try {
      let paperId: string;
      if (mode === 'arxiv') {
        const paper = await addByArxiv(arxivId.trim());
        paperId = paper.id;
      } else if (mode === 'doi') {
        const paper = await addByDoi(doi.trim());
        paperId = paper.id;
      } else {
        if (!file) {
          throw new Error('Выберите PDF файл');
        }
        const paper = await uploadPdf(file);
        paperId = paper.id;
      }

      await navigate({ to: '/reader/$paperId', params: { paperId } });
    } catch (err) {
      if (err instanceof ApiError) {
        setError(err.detail);
      } else if (err instanceof Error) {
        setError(err.message);
      } else {
        setError('Не удалось добавить статью');
      }
    } finally {
      setIsSubmitting(false);
    }
  };

  return (
    <div className="add-paper-page">
      <div className="add-paper-page__header">
        <div>
          <h1>Добавить статью</h1>
          <p>PDF, arXiv ID/URL или DOI</p>
        </div>
        <Link to="/library">К библиотеке</Link>
      </div>

      <form className="add-paper-form" onSubmit={onSubmit}>
        <div className="add-paper-tabs" role="tablist">
          {(
            [
              ['arxiv', 'arXiv'],
              ['doi', 'DOI'],
              ['pdf', 'PDF'],
            ] as const
          ).map(([value, label]) => (
            <button
              key={value}
              type="button"
              className={mode === value ? 'is-active' : undefined}
              onClick={() => setMode(value)}
            >
              {label}
            </button>
          ))}
        </div>

        {mode === 'arxiv' ? (
          <label className="auth-field">
            <span>arXiv ID или URL</span>
            <input
              value={arxivId}
              onChange={(event) => setArxivId(event.target.value)}
              placeholder="1706.03762 или https://arxiv.org/abs/1706.03762"
              required
            />
          </label>
        ) : null}

        {mode === 'doi' ? (
          <label className="auth-field">
            <span>DOI</span>
            <input
              value={doi}
              onChange={(event) => setDoi(event.target.value)}
              placeholder="10.1038/nature14539"
              required
            />
          </label>
        ) : null}

        {mode === 'pdf' ? (
          <label className="auth-field">
            <span>PDF файл</span>
            <input
              type="file"
              accept="application/pdf,.pdf"
              required
              onChange={(event) => setFile(event.target.files?.[0] ?? null)}
            />
          </label>
        ) : null}

        {error ? <div className="auth-error">{error}</div> : null}

        <button className="auth-submit" type="submit" disabled={isSubmitting}>
          {isSubmitting ? 'Добавляем…' : 'Добавить в библиотеку'}
        </button>
      </form>
    </div>
  );
}
