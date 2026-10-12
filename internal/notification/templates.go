package notification

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"

	"retrotide/internal/email"
)

type (
	VerificationEmailData struct {
		To           string
		SiteName     string
		LinkURL      string
		ChangesEmail bool
	}

	PasswordResetEmailData struct {
		To       string
		SiteName string
		LinkURL  string
	}

	EmailChangedEmailData struct {
		To       string
		SiteName string
		NewEmail string
		LinkURL  string
	}

	InvitationEmailData struct {
		To       string
		SiteName string
		LinkURL  string
	}

	notificationEmailData struct {
		To       string
		SiteName string
		Message  string
		LinkURL  string
	}

	renderedEmail struct {
		to       string
		subject  string
		template string
		data     any
	}
)

var (
	//go:embed templates/*.tmpl
	templateFS embed.FS

	emailTemplates = template.Must(template.ParseFS(templateFS, "templates/*.tmpl"))
)

func VerificationEmail(data VerificationEmailData) (email.Message, error) {
	subject := fmt.Sprintf("Confirm your email address for %s", data.SiteName)
	if data.ChangesEmail {
		subject = fmt.Sprintf("Confirm your new email address for %s", data.SiteName)
	}

	return render(renderedEmail{to: data.To, subject: subject, template: "verify_email.tmpl", data: data})
}

func PasswordResetEmail(data PasswordResetEmailData) (email.Message, error) {
	subject := fmt.Sprintf("Reset your %s password", data.SiteName)

	return render(renderedEmail{to: data.To, subject: subject, template: "password_reset.tmpl", data: data})
}

func EmailChangedEmail(data EmailChangedEmailData) (email.Message, error) {
	subject := fmt.Sprintf("Your %s email address is being changed", data.SiteName)

	return render(renderedEmail{to: data.To, subject: subject, template: "email_changed.tmpl", data: data})
}

func InvitationEmail(data InvitationEmailData) (email.Message, error) {
	subject := fmt.Sprintf("You are invited to %s", data.SiteName)

	return render(renderedEmail{to: data.To, subject: subject, template: "invitation.tmpl", data: data})
}

func notificationEmail(data notificationEmailData) (email.Message, error) {
	subject := fmt.Sprintf("You have a new notification on %s", data.SiteName)

	return render(renderedEmail{to: data.To, subject: subject, template: "notification.tmpl", data: data})
}

func render(r renderedEmail) (email.Message, error) {
	var body bytes.Buffer
	if err := emailTemplates.ExecuteTemplate(&body, r.template, r.data); err != nil {
		return email.Message{}, fmt.Errorf("render the %s email: %w", r.template, err)
	}

	return email.Message{To: r.to, Subject: r.subject, HTML: body.String()}, nil
}
