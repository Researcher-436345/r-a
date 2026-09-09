package identity

import (
	"fmt"
	"log"
	"time"

	"github.com/centraluniversity/researcher/internal/platform/mailer"
)

const verifyEmailTTL = 24 * time.Hour
const resetPasswordTTL = 1 * time.Hour

func (a API) sendVerifyEmail(email, token string) {
	if a.Mail == nil {
		return
	}
	link := fmt.Sprintf("%s/verify-email?token=%s", a.Config.FrontendURL, token)
	msg := mailer.Message{
		To:      email,
		Subject: "Researcher — подтвердите регистрацию",
		Text: fmt.Sprintf("Подтвердите регистрацию в Researcher, перейдя по ссылке:\n%s\n\n"+
			"Ссылка действует %d часов. Если вы не регистрировались — просто игнорируйте это письмо.",
			link, int(verifyEmailTTL.Hours())),
		HTML: fmt.Sprintf(
			"<p>Подтвердите регистрацию в <b>Researcher</b>, перейдя по ссылке:</p>"+
				"<p><a href=\"%s\">Подтвердить email</a></p>"+
				"<p style=\"color:#666\">Ссылка действует %d часов. Если вы не регистрировались — просто игнорируйте это письмо.</p>",
			link, int(verifyEmailTTL.Hours())),
	}
	if err := a.Mail.Send(msg); err != nil {
		log.Printf("identity: send verify email to %s: %v", email, err)
	}
}

func (a API) sendResetPasswordEmail(email, token string) {
	if a.Mail == nil {
		return
	}
	link := fmt.Sprintf("%s/reset-password?token=%s", a.Config.FrontendURL, token)
	msg := mailer.Message{
		To:      email,
		Subject: "Researcher — сброс пароля",
		Text: fmt.Sprintf("Сбросить пароль можно по ссылке:\n%s\n\n"+
			"Ссылка действует %d минут. Если вы не запрашивали сброс — игнорируйте это письмо, пароль не изменится.",
			link, int(resetPasswordTTL.Minutes())),
		HTML: fmt.Sprintf(
			"<p>Сбросить пароль можно по ссылке:</p>"+
				"<p><a href=\"%s\">Сбросить пароль</a></p>"+
				"<p style=\"color:#666\">Ссылка действует %d минут. Если вы не запрашивали сброс — игнорируйте это письмо, пароль не изменится.</p>",
			link, int(resetPasswordTTL.Minutes())),
	}
	if err := a.Mail.Send(msg); err != nil {
		log.Printf("identity: send reset email to %s: %v", email, err)
	}
}
