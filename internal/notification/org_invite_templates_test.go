package notification

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testInviteLink = "https://retrotide.test/invite#0f1e2d3c"
	inviteExpiry   = "This link expires in 7 days, works once and only for this email address."
)

func orgInviteData() OrgInviteEmailData {
	return OrgInviteEmailData{
		To:           "carol@example.com",
		SiteName:     testSiteName,
		Organisation: "Acme Retros",
		InviterName:  "Dana",
		LinkURL:      testInviteLink,
	}
}

func TestOrgInviteEmail_ForAMember(t *testing.T) {
	// given
	data := orgInviteData()

	// when
	message, err := OrgInviteEmail(data)

	// then
	require.NoError(t, err)
	assertCompleteDocument(t, message)
	assert.Equal(t, "carol@example.com", message.To)
	assert.Equal(t, "You are invited to join Acme Retros on RetroTide", message.Subject)
	assert.Contains(t, message.HTML, "<strong><bdi>Dana</bdi></strong> invited you to join <strong><bdi>Acme Retros</bdi></strong> on <bdi>RetroTide</bdi>.")
	assert.Contains(t, message.HTML, `href="`+testInviteLink+`"`)
	assert.Contains(t, message.HTML, "Accept the invite")
	assert.Contains(t, message.HTML, "you create one for this address when you accept")
	assert.Contains(t, message.HTML, inviteExpiry)
	assert.Contains(t, message.HTML, "If you were not expecting this invitation, ignore this email.")
	assert.NotContains(t, message.HTML, "as an admin")
	assert.NotContains(t, message.HTML, "in the team")
}

func TestOrgInviteEmail_ForAnAdminOfATeam(t *testing.T) {
	// given
	data := orgInviteData()
	data.Admin = true
	data.Team = "Platform"

	// when
	message, err := OrgInviteEmail(data)

	// then
	require.NoError(t, err)
	assertCompleteDocument(t, message)
	assert.Contains(t, message.HTML, "on <bdi>RetroTide</bdi> as an admin, in the team <strong><bdi>Platform</bdi></strong>.")
}

func TestOrgInviteEmail_WithoutAnInviterName(t *testing.T) {
	// given an inviter who signed up through a provider that sent no name
	data := orgInviteData()
	data.InviterName = ""

	// when
	message, err := OrgInviteEmail(data)

	// then
	require.NoError(t, err)
	assert.Contains(t, message.HTML, "You are invited to join <strong><bdi>Acme Retros</bdi></strong>")
	assert.NotContains(t, message.HTML, "invited you")
}

func TestOrgInviteEmail_EscapesWhatMembersTyped(t *testing.T) {
	// given
	data := orgInviteData()
	data.Organisation = "<script>alert(1)</script>"
	data.InviterName = "<img src=x onerror=alert(2)>"
	data.Team = "<b>Platform</b>"

	// when
	message, err := OrgInviteEmail(data)

	// then
	require.NoError(t, err)
	assert.NotContains(t, message.HTML, "<script>")
	assert.NotContains(t, message.HTML, "<img")
	assert.NotContains(t, message.HTML, "<b>Platform")
	assert.Contains(t, message.HTML, "&lt;script&gt;alert(1)&lt;/script&gt;")
	assert.Contains(t, message.HTML, "&lt;img src=x onerror=alert(2)&gt;")
	assert.Contains(t, message.HTML, "&lt;b&gt;Platform&lt;/b&gt;")
}

func TestOrgInviteEmail_NeutralisesAScriptLink(t *testing.T) {
	// given
	data := orgInviteData()
	data.LinkURL = "javascript:alert(1)"

	// when
	message, err := OrgInviteEmail(data)

	// then
	require.NoError(t, err)
	assert.NotContains(t, message.HTML, `href="javascript:`)
}
