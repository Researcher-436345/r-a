import { Link } from '@tanstack/react-router';
import { useState, type FormEvent } from 'react';

import { forgotPassword } from '../../features/auth/auth-api';
import { ApiError } from '../../shared/api/client';
import { LogoMark } from '../../shared/ui/logo-mark';

export function ForgotPasswordPage() {
  const [email, setEmail] = useState('');
  const [error, setError] = useState<string | null>(null);
  const [sent, setSent] = useState(false);
  const [isSubmitting, setIsSubmitting] = useState(false);

  const onSubmit = async (event: FormEvent) => {
    event.preventDefault();
    setError(null);
    setIsSubmitting(true);

    try {
      await forgotPassword(email.trim());
      setSent(true);
    } catch (err) {
      if (err instanceof ApiError) {
        setError(err.detail);
      } else {
        setError('Не удалось отправить запрос. Проверьте, что API запущен.');
      }
    } finally {
      setIsSubmitting(false);
    }
  };

  return (
    <div className="auth-page">
      {sent ? (
        <div className="auth-card">
          <div className="auth-card__brand">
            <LogoMark />
            <div>
              <h1>Проверьте почту</h1>
              <p>Ссылка для сброса пароля отправлена</p>
            </div>
          </div>
          <p className="auth-note">
            Если аккаунт с email <strong>{email.trim()}</strong> существует, мы
            отправили письмо со ссылкой для сброса пароля. Ссылка действует 1 час.
          </p>
          <p className="auth-switch">
            Вспомнили пароль? <Link to="/login">Войти</Link>
          </p>
        </div>
      ) : (
        <form className="auth-card" onSubmit={onSubmit}>
          <div className="auth-card__brand">
            <LogoMark />
            <div>
              <h1>Сброс пароля</h1>
              <p>Отправим ссылку для смены пароля</p>
            </div>
          </div>

          <label className="auth-field">
            <span>Email</span>
            <input
              type="email"
              autoComplete="email"
              required
              value={email}
              onChange={(event) => setEmail(event.target.value)}
              placeholder="you@example.com"
            />
          </label>

          {error ? <div className="auth-error">{error}</div> : null}

          <button className="auth-submit" type="submit" disabled={isSubmitting}>
            {isSubmitting ? 'Отправляем…' : 'Отправить письмо'}
          </button>
          <p className="auth-switch">
            <Link to="/login">Вернуться ко входу</Link>
          </p>
        </form>
      )}
    </div>
  );
}
