import { Link, useNavigate } from '@tanstack/react-router';
import { useEffect, useRef, useState } from 'react';

import { verifyEmail } from '../../features/auth/auth-api';
import { ApiError } from '../../shared/api/client';
import { LogoMark } from '../../shared/ui/logo-mark';

export function VerifyEmailPage() {
  const navigate = useNavigate();
  const [status, setStatus] = useState<'pending' | 'error'>('pending');
  const [detail, setDetail] = useState<string | null>(null);
  const started = useRef(false);

  useEffect(() => {
    if (started.current) {
      return;
    }
    started.current = true;

    const token = new URLSearchParams(window.location.search).get('token') ?? '';
    if (!token) {
      setStatus('error');
      setDetail('В ссылке нет токена подтверждения.');
      return;
    }

    verifyEmail(token)
      .then(() => {
        // Авто-вход выполнен: access-токен уже в памяти.
        return navigate({ to: '/' });
      })
      .catch((err) => {
        setStatus('error');
        setDetail(
          err instanceof ApiError ? err.detail : 'Не удалось подтвердить email.',
        );
      });
  }, [navigate]);

  return (
    <div className="auth-page">
      <div className="auth-card">
        <div className="auth-card__brand">
          <LogoMark />
          <div>
            <h1>Подтверждение email</h1>
            <p>Проверяем ссылку из письма…</p>
          </div>
        </div>

        {status === 'pending' ? (
          <p className="auth-note">Подтверждаем ваш email, секунду…</p>
        ) : (
          <>
            <div className="auth-error">{detail}</div>
            <p className="auth-note">
              Ссылки действуют 24 часа и срабатывают один раз. Запросите новое
              письмо на странице входа или регистрации.
            </p>
            <Link className="auth-submit" to="/login">
              Вернуться ко входу
            </Link>
          </>
        )}
      </div>
    </div>
  );
}
