import { Link } from '@tanstack/react-router';
import { useState, type FormEvent } from 'react';

import { resetPassword } from '../../features/auth/auth-api';
import { ApiError } from '../../shared/api/client';
import { LogoMark } from '../../shared/ui/logo-mark';

export function ResetPasswordPage() {
  const [token] = useState(
    () => new URLSearchParams(window.location.search).get('token') ?? '',
  );
  const [password, setPassword] = useState('');
  const [confirm, setConfirm] = useState('');
  const [error, setError] = useState<string | null>(null);
  const [done, setDone] = useState(false);
  const [isSubmitting, setIsSubmitting] = useState(false);

  const onSubmit = async (event: FormEvent) => {
    event.preventDefault();
    setError(null);

    if (!token) {
      setError('В ссылке нет токена сброса. Запросите новое письмо.');
      return;
    }
    if (password !== confirm) {
      setError('Пароли не совпадают.');
      return;
    }

    setIsSubmitting(true);
    try {
      await resetPassword(token, password);
      setDone(true);
    } catch (err) {
      if (err instanceof ApiError) {
        setError(err.detail);
      } else {
        setError('Не удалось обновить пароль. Проверьте, что API запущен.');
      }
    } finally {
      setIsSubmitting(false);
    }
  };

  return (
    <div className="auth-page">
      <form className="auth-card" onSubmit={onSubmit}>
        <div className="auth-card__brand">
          <LogoMark />
          <div>
            <h1>Новый пароль</h1>
            <p>Придумайте новый пароль для входа</p>
          </div>
        </div>

        {done ? (
          <>
            <p className="auth-note">
              Пароль обновлён, все активные сессии завершены. Войдите с новым
              паролем.
            </p>
            <button
              className="auth-submit"
              type="button"
              onClick={() => window.location.assign('/login?reset=1')}
            >
              Войти
            </button>
          </>
        ) : (
          <>
            <label className="auth-field">
              <span>Новый пароль</span>
              <input
                type="password"
                autoComplete="new-password"
                required
                minLength={8}
                value={password}
                onChange={(event) => setPassword(event.target.value)}
                placeholder="Минимум 8 символов"
              />
            </label>
            <label className="auth-field">
              <span>Повторите пароль</span>
              <input
                type="password"
                autoComplete="new-password"
                required
                minLength={8}
                value={confirm}
                onChange={(event) => setConfirm(event.target.value)}
                placeholder="Ещё раз"
              />
            </label>

            {error ? <div className="auth-error">{error}</div> : null}

            <button className="auth-submit" type="submit" disabled={isSubmitting}>
              {isSubmitting ? 'Сохраняем…' : 'Сохранить пароль'}
            </button>
            <p className="auth-switch">
              <Link to="/login">Вернуться ко входу</Link>
            </p>
          </>
        )}
      </form>
    </div>
  );
}
