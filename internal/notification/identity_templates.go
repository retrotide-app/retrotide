package notification

import (
	"fmt"

	"retrotide/internal/email"
)

type (
	IdentityLinkedEmailData struct {
		To               string
		SiteName         string
		Provider         string
		IdentityEmail    string
		Automatic        bool
		HasPassword      bool
		AccountCreatedOn string
		SignInMethodsURL string
		ResetPasswordURL string
	}

	IdentityUnlinkedEmailData struct {
		To               string
		SiteName         string
		Provider         string
		IdentityEmail    string
		ResetPasswordURL string
	}
)

func IdentityLinkedEmail(data IdentityLinkedEmailData) (email.Message, error) {
	subject := fmt.Sprintf("%s sign-in was linked to your %s account", data.Provider, data.SiteName)

	return render(renderedEmail{to: data.To, subject: subject, template: "identity_linked.tmpl", data: data})
}

func IdentityUnlinkedEmail(data IdentityUnlinkedEmailData) (email.Message, error) {
	subject := fmt.Sprintf("%s sign-in was removed from your %s account", data.Provider, data.SiteName)

	return render(renderedEmail{to: data.To, subject: subject, template: "identity_unlinked.tmpl", data: data})
}
