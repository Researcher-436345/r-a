import { useCallback, useEffect, useState } from 'react';

import {
  fetchSessions,
  revokeOtherSessions,
  revokeSession,
  type SessionInfo,
} from '../../features/auth/auth-api';
import { clearTokens } from '../../features/auth/token-storage';
import { ApiError } from '../../shared/api/client';

function formatDateTime(iso: string): string {
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) {
    return iso;
  }
  return date.toLocaleString(undefined, {
    dateStyle: 'medium',
    timeStyle: 'short',
  });
}

function deviceLabel(userAgent: string): string {
  if (/iPhone|iPad|iOS/i.test(userAgent)) return 'iOS';
  if (/Android/i.test(userAgent)) return 'Android';
  if (/Macintosh|Mac OS X/i.test(userAgent)) return 'macOS';
  if (/Windows/i.test(userAgent)) return 'Windows';
  if (/Linux/i.test(userAgent)) return 'Linux';
  return 'Неизвестное устройство';
}

export function SessionsPage() {
  const [sessions, setSessions] = useState<SessionInfo[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    setIsLoading(true);
    setError(null);
    try {
      setSessions(await fetchSessions());
    } catch (err) {
      setError(
        err instanceof ApiError ? err.detail : 'Не удалось загрузить сессии.',
      );
    } finally {
      setIsLoading(false);
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  const handleRevoke = async (session: SessionInfo) => {
    setError(null);
    try {
      await revokeSession(session.id);
      if (session.current) {
        // Завершили собственную сессию — разлогиниваем вкладку.
        clearTokens();
        window.location.assign('/login');
        return;
      }
      setSessions((prev) => prev.filter((s) => s.id !== session.id));
    } catch (err) {
      setError(err instanceof ApiError ? err.detail : 'Не удалось завершить сессию.');
    }
  };

  const handleRevokeOthers = async () => {
    setError(null);
    try {
      await revokeOtherSessions();
      setSessions((prev) => prev.filter((s) => s.current));
    } catch (err) {
      setError(err instanceof ApiError ? err.detail : 'Не удалось завершить сессии.');
    }
  };

  return (
    <div className="sessions-page">
      <header className="sessions-page__header">
        <div>
          <h1>Активные сессии</h1>
          <p>Устройства, с которых выполнен вход в ваш аккаунт</p>
        </div>
        <button
          className="sessions-page__revoke-all"
          type="button"
          onClick={handleRevokeOthers}
          disabled={isLoading || sessions.filter((s) => !s.current).length === 0}
        >
          Выйти со всех устройств
        </button>
      </header>

      {error ? <div className="auth-error">{error}</div> : null}

      {isLoading ? (
        <p className="sessions-page__empty">Загружаем…</p>
      ) : sessions.length === 0 ? (
        <p className="sessions-page__empty">Активных сессий нет.</p>
      ) : (
        <ul className="sessions-list">
          {sessions.map((session) => (
            <li key={session.id} className="sessions-item">
              <div className="sessions-item__info">
                <span className="sessions-item__device">
                  {deviceLabel(session.user_agent)}
                  {session.current ? (
                    <span className="sessions-item__badge">текущая</span>
                  ) : null}
                </span>
                <span className="sessions-item__meta">
                  {session.ip || 'IP неизвестен'} · вход{' '}
                  {formatDateTime(session.created_at)} · активность{' '}
                  {formatDateTime(session.last_used_at)}
                </span>
              </div>
              <button
                className="sessions-item__revoke"
                type="button"
                onClick={() => handleRevoke(session)}
              >
                {session.current ? 'Выйти' : 'Завершить'}
              </button>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
