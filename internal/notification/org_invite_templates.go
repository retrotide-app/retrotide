package notification

import (
	"fmt"

	"retrotide/internal/email"
)

type (
	OrgInviteEmailData struct {
		To           string
		SiteName     string
		Organisation string
		InviterName  string
		Team         string
		Admin        bool
		LinkURL      string
	}
)

func OrgInviteEmail(data OrgInviteEmailData) (email.Message, error) {
	subject := fmt.Sprintf("You are invited to join %s on %s", data.Organisation, data.SiteName)

	return render(renderedEmail{to: data.To, subject: subject, template: "org_invite.tmpl", data: data})
}
