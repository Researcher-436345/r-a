//go:build integration

package integration

import (
	"strings"
	"testing"
	"time"

	"github.com/centraluniversity/researcher/tests/client"
	"github.com/stretchr/testify/require"
)

// F02 — регистрация, вход, сессии. Ист.: HANDOFF §4 «Auth», §10.
// Кейсы: docs/testing/flows/F02-auth.md.

// F02-01 Регистрация: generic-ответ без токенов, письмо с ссылкой, вход до подтверждения запрещён.
func TestF02_RegistrationUntilFirstLogin(t *testing.T) {
	stand.ResetThrottle(t)
	s := guest(t)
	email := client.UniqueEmail("f02-register")
	mail := stand.Mailpit()

	resp := s.Register(email, client.DefaultPassword)
	require.Equal(t, 200, resp.Status, resp)
	body := resp.Map(t)
	require.Equal(t, "check your email", body["message"])
	require.NotContains(t, body, "access_token", "registration must not log the user in")
	require.Empty(t, s.RefreshCookie(), "no session cookie before verification")

	msg := mail.WaitMessage(t, email, 0, 15*time.Second)
	require.Contains(t, msg.Subject, "подтвердите")
	token := msg.Token(t, "verify-email")
	require.NotEmpty(t, token)

	login := s.Login(email, client.DefaultPassword)
	require.Equal(t, 403, login.Status, login)
	require.Equal(t, "email_not_verified", login.Code())
	resent := mail.WaitMessage(t, email, 1, 15*time.Second)
	require.NotEmpty(t, resent.Token(t, "verify-email"), "a failed login re-sends the confirmation link")

	verify := s.VerifyEmail(token)
	require.Equal(t, 200, verify.Status, verify)
	require.NotEmpty(t, s.Token, "verification logs the user in")
	require.NotEmpty(t, s.RefreshCookie(), "and sets the refresh cookie")
	require.Contains(t, verify.Header.Get("Set-Cookie"), "HttpOnly")

	me := s.Get("/auth/me")
	require.Equal(t, 200, me.Status, me)
	require.Equal(t, email, me.Map(t)["email"])
	require.Equal(t, true, me.Map(t)["email_verified"])

	again := s.Clone().VerifyEmail(token)
	require.Equal(t, 400, again.Status, "verification token is single-use: %s", again)
}

// F02-02 Регистрация не выдаёт, существует ли адрес, и требует пароль ≥ 8 символов.
func TestF02_RegisterValidation(t *testing.T) {
	stand.ResetThrottle(t)
	t.Run("taken email gets the same reply", func(t *testing.T) {
		u := user(t, "f02-generic")
		again := guest(t).Register(u.Email, "another-password-123")
		require.Equal(t, 200, again.Status, again)
		require.Equal(t, "check your email", again.Map(t)["message"], "same body for a taken email")
	})
	t.Run("short password", func(t *testing.T) {
		short := guest(t).Register(client.UniqueEmail("f02-short"), "1234567")
		require.Equal(t, 400, short.Status, short)
	})
	t.Run("empty email", func(t *testing.T) {
		noMail := guest(t).Register("", client.DefaultPassword)
		require.Equal(t, 400, noMail.Status, noMail)
	})
}

// F02-03 Вход: верные данные → токен и cookie; неверный пароль и неизвестный email неотличимы.
func TestF02_Login(t *testing.T) {
	u := user(t, "f02-login")
	t.Run("success", func(t *testing.T) {
		dev := u.Clone()
		ok := dev.Login(u.Email, u.Password)
		require.Equal(t, 200, ok.Status, ok)
		require.Equal(t, "bearer", ok.Map(t)["token_type"])
		require.NotEmpty(t, dev.RefreshCookie())
	})
	t.Run("wrong password and unknown email are indistinguishable", func(t *testing.T) {
		wrong := u.Clone().Login(u.Email, "definitely-wrong-pass")
		require.Equal(t, 401, wrong.Status, wrong)
		unknown := u.Clone().Login(client.UniqueEmail("f02-nobody"), "definitely-wrong-pass")
		require.Equal(t, 401, unknown.Status, unknown)
		require.Equal(t, wrong.Detail(), unknown.Detail(), "responses must not reveal whether the account exists")
	})
}

// F02-04 Email нормализуется: регистр и пробелы не мешают войти.
func TestF02_LoginNormalizesEmail(t *testing.T) {
	u := user(t, "f02-case")
	resp := u.Clone().Login("  "+strings.ToUpper(u.Email)+" ", u.Password)
	require.Equal(t, 200, resp.Status, resp)
}

// F02-05 Silent refresh: cookie даёт новый access, cookie ротируется, старая cookie → 401 и отзыв всех сессий.
func TestF02_RefreshReuseDetection(t *testing.T) {
	u := user(t, "f02-refresh")
	oldCookie := u.RefreshCookie()
	oldToken := u.Token

	refreshed := u.Refresh()
	require.Equal(t, 200, refreshed.Status, refreshed)
	require.NotEqual(t, oldToken, u.Token, "new access token")
	require.NotEqual(t, oldCookie, u.RefreshCookie(), "refresh cookie is rotated")
	newCookie := u.RefreshCookie()
	require.Equal(t, 200, u.Get("/auth/me").Status, "new access token works")

	other := u.LoginAs(t) // a second device that must survive an honest refresh
	require.Equal(t, 200, other.Refresh().Status)

	replay := u.Clone()
	replay.SetRefreshCookie(oldCookie)
	resp := replay.Refresh()
	require.Equal(t, 401, resp.Status, "replaying a rotated cookie is refused: %s", resp)

	// Reuse detection: every session of the user is revoked, including the honest ones.
	current := u.Clone()
	current.SetRefreshCookie(newCookie)
	require.Equal(t, 401, current.Refresh().Status, "the current cookie is revoked too")
	require.Equal(t, 401, other.Refresh().Status, "the other device is logged out as well")
	sessions := u.Get("/auth/sessions")
	require.Equal(t, 200, sessions.Status, sessions)
	require.Empty(t, sessions.Map(t)["sessions"], "no active sessions remain")
}

// F02-06 Выход: cookie очищена, refresh больше не работает, другая вкладка не затронута.
func TestF02_LogoutEndsOnlyThisDevice(t *testing.T) {
	u := user(t, "f02-logout")
	other := u.LoginAs(t)

	resp := u.Logout()
	require.Equal(t, 200, resp.Status, resp)
	require.Contains(t, resp.Header.Get("Set-Cookie"), "researcher_refresh=;", "cookie is cleared")
	require.Equal(t, 401, u.Refresh().Status, "logged-out device cannot refresh")
	require.Equal(t, 200, other.Refresh().Status, "the other device keeps working")
}

// F02-07 Refresh без cookie и с выдуманной cookie → 401.
func TestF02_RefreshWithoutSession(t *testing.T) {
	g := guest(t)
	require.Equal(t, 401, g.Refresh().Status)
	g.SetRefreshCookie("made-up-token")
	require.Equal(t, 401, g.Refresh().Status)
}

// F02-08 Сброс пароля: письмо → новый пароль → старый не работает, все сессии отозваны, токен одноразовый.
func TestF02_PasswordReset(t *testing.T) {
	stand.ResetThrottle(t)
	u := user(t, "f02-reset")
	other := u.LoginAs(t)
	mail := stand.Mailpit()
	before := mail.Count(t, u.Email)

	forgot := guest(t).Post("/auth/forgot-password", map[string]string{"email": u.Email})
	require.Equal(t, 200, forgot.Status, forgot)
	nobody := guest(t).Post("/auth/forgot-password", map[string]string{"email": client.UniqueEmail("f02-nobody")})
	require.Equal(t, 200, nobody.Status, nobody)
	require.Equal(t, forgot.Body, nobody.Body, "forgot-password does not reveal whether the account exists")

	msg := mail.WaitMessage(t, u.Email, before, 15*time.Second)
	require.Contains(t, msg.Subject, "пароля")
	token := msg.Token(t, "reset-password")

	short := guest(t).Post("/auth/reset-password", map[string]string{"token": token, "new_password": "short"})
	require.Equal(t, 400, short.Status, short)

	newPassword := "brand-new-password-42"
	reset := guest(t).Post("/auth/reset-password", map[string]string{"token": token, "new_password": newPassword})
	require.Equal(t, 200, reset.Status, reset)

	require.Equal(t, 401, u.Clone().Login(u.Email, u.Password).Status, "old password is gone")
	require.Equal(t, 200, u.Clone().Login(u.Email, newPassword).Status, "new password works")
	require.Equal(t, 401, u.Refresh().Status, "sessions from before the reset are revoked")
	require.Equal(t, 401, other.Refresh().Status)

	reuse := guest(t).Post("/auth/reset-password", map[string]string{"token": token, "new_password": "yet-another-pass-1"})
	require.Equal(t, 400, reuse.Status, "reset token is single-use: %s", reuse)
}

// F02-09 Просроченные одноразовые токены не принимаются.
func TestF02_ExpiredTokensAreRejected(t *testing.T) {
	stand.ResetThrottle(t)
	s := guest(t)
	email := client.UniqueEmail("f02-expired")
	require.Equal(t, 200, s.Register(email, client.DefaultPassword).Status)
	msg := stand.Mailpit().WaitMessage(t, email, 0, 15*time.Second)
	token := msg.Token(t, "verify-email")

	stand.ExpireAuthTokens(t, email) // «прошли 24 часа»
	resp := s.VerifyEmail(token)
	require.Equal(t, 400, resp.Status, resp)
	require.Empty(t, s.Token)
}

// F02-10 Повторная отправка письма: generic-ответ; уже подтверждённому письмо не приходит.
func TestF02_ResendVerification(t *testing.T) {
	stand.ResetThrottle(t)
	s := guest(t)
	email := client.UniqueEmail("f02-resend")
	require.Equal(t, 200, s.Register(email, client.DefaultPassword).Status)
	mail := stand.Mailpit()
	mail.WaitMessage(t, email, 0, 15*time.Second)

	resend := s.Post("/auth/resend-verification", map[string]string{"email": email})
	require.Equal(t, 200, resend.Status, resend)
	mail.WaitMessage(t, email, 1, 15*time.Second)

	u := user(t, "f02-resend-verified")
	before := mail.Count(t, u.Email)
	require.Equal(t, 200, guest(t).Post("/auth/resend-verification", map[string]string{"email": u.Email}).Status)
	time.Sleep(1500 * time.Millisecond)
	require.Equal(t, before, mail.Count(t, u.Email), "a verified account gets no new confirmation mail")
}

// F02-11 Активные сессии: список, текущая помечена, отзыв одной и всех остальных.
func TestF02_ActiveSessions(t *testing.T) {
	u := user(t, "f02-sessions")
	phone := u.LoginAs(t)
	tablet := u.LoginAs(t)

	resp := u.Get("/auth/sessions")
	require.Equal(t, 200, resp.Status, resp)
	var out struct {
		Sessions []struct {
			ID      string `json:"id"`
			Current bool   `json:"current"`
		} `json:"sessions"`
	}
	resp.JSON(t, &out)
	require.Len(t, out.Sessions, 3)
	currentCount, otherID := 0, ""
	for _, s := range out.Sessions {
		if s.Current {
			currentCount++
		} else if otherID == "" {
			otherID = s.ID
		}
	}
	require.Equal(t, 1, currentCount, "exactly one session is marked current")

	require.Equal(t, 204, u.Delete("/auth/sessions/"+otherID).Status)
	require.Equal(t, 404, u.Delete("/auth/sessions/"+otherID).Status, "already revoked → 404")
	require.Equal(t, 404, u.Delete("/auth/sessions/00000000-0000-0000-0000-000000000001").Status)

	require.Equal(t, 204, u.Delete("/auth/sessions").Status, "log out everywhere else")
	require.Equal(t, 200, u.Refresh().Status, "the current device stays signed in")
	list := u.Get("/auth/sessions").Map(t)["sessions"].([]any)
	require.Len(t, list, 1)
	require.Equal(t, 401, phone.Refresh().Status, "a logged-out device cannot refresh")

	// [assumption] Отозванное явно устройство, пытаясь обновить сессию, не должно
	// выбивать текущее: это не кража токена, а обычный «телефон открыл приложение». Сейчас выбивает — см. D-12.
	require.Equal(t, 200, u.Refresh().Status, "the current device still works after a logged-out device retried")
	require.Equal(t, 401, tablet.Refresh().Status)
	require.Equal(t, 200, u.Refresh().Status)

	stranger := user(t, "f02-stranger")
	require.Equal(t, 404, stranger.Delete("/auth/sessions/"+list[0].(map[string]any)["id"].(string)).Status, "cannot revoke someone else's session")
}

// F02-12 Троттлинг регистрации: после лимита 429 с Retry-After.
func TestF02_RegisterThrottle(t *testing.T) {
	stand.ResetThrottle(t)
	t.Cleanup(func() { stand.ResetThrottle(t) })
	var last client.Response
	for i := 0; i < 5; i++ {
		last = guest(t).Register(client.UniqueEmail("f02-throttle"), client.DefaultPassword)
		require.Equalf(t, 200, last.Status, "attempt %d: %s", i+1, last)
	}
	last = guest(t).Register(client.UniqueEmail("f02-throttle"), client.DefaultPassword)
	require.Equal(t, 429, last.Status, last)
	require.NotEmpty(t, last.Header.Get("Retry-After"))
}

// F02-13 Троттлинг входа по email: после лимита даже верный пароль → 429.
func TestF02_LoginThrottlePerEmail(t *testing.T) {
	stand.ResetThrottle(t)
	t.Cleanup(func() { stand.ResetThrottle(t) })
	u := user(t, "f02-login-throttle")
	for i := 0; i < 5; i++ {
		require.Equal(t, 401, u.Clone().Login(u.Email, "wrong-password-xx").Status)
	}
	resp := u.Clone().Login(u.Email, u.Password)
	require.Equal(t, 429, resp.Status, resp)
	require.NotEmpty(t, resp.Header.Get("Retry-After"))
}

// F02-14 Мусор в теле → 400 с текстом, а не 500; неизвестные поля отклоняются.
func TestF02_MalformedBodies(t *testing.T) {
	g := guest(t)
	require.Equal(t, 400, g.Post("/auth/login", "{not json").Status)
	require.Equal(t, 400, g.Post("/auth/login", map[string]any{"email": "a@b.c", "password": "12345678", "remember": true}).Status, "unknown fields are rejected")
	require.Equal(t, 400, g.Post("/auth/verify-email", map[string]string{"token": ""}).Status)
	require.Equal(t, 400, g.Post("/auth/forgot-password", map[string]string{"email": ""}).Status)
}
