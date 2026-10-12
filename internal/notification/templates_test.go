package notification

import (
	"strings"
	"testing"

	"retrotide/internal/email"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testSiteName = "RetroTide"
	testLink     = "https://retrotide.test/verify-email?token=abc123&source=email"
	escapedLink  = "https://retrotide.test/verify-email?token=abc123&amp;source=email"
)

func assertCompleteDocument(t *testing.T, message email.Message) {
	t.Helper()

	assert.True(t, strings.HasPrefix(message.HTML, "<!DOCTYPE html>"), "the email starts with its doctype")
	assert.Contains(t, message.HTML, `<html lang="en-GB">`)
	assert.True(t, strings.HasSuffix(strings.TrimSpace(message.HTML), "</html>"), "the email ends with its closing html tag")
	assert.Contains(t, message.HTML, "<bdi>"+testSiteName+"</bdi>")
}

func TestVerificationEmail_ForARegistration(t *testing.T) {
	// given
	data := VerificationEmailData{To: "sam@example.com", SiteName: testSiteName, LinkURL: testLink}

	// when
	message, err := VerificationEmail(data)

	// then
	require.NoError(t, err)
	assertCompleteDocument(t, message)
	assert.Equal(t, "sam@example.com", message.To)
	assert.Equal(t, "Confirm your email address for RetroTide", message.Subject)
	assert.Contains(t, message.HTML, `href="`+escapedLink+`"`)
	assert.Contains(t, message.HTML, "This link expires in 24 hours.")
	assert.Contains(t, message.HTML, "If you did not create this account, ignore this email.")
	assert.NotContains(t, message.HTML, "change your email address")
}

func TestVerificationEmail_ForAChangeOfAddress(t *testing.T) {
	// given
	data := VerificationEmailData{To: "new@example.com", SiteName: testSiteName, LinkURL: testLink, ChangesEmail: true}

	// when
	message, err := VerificationEmail(data)

	// then
	require.NoError(t, err)
	assertCompleteDocument(t, message)
	assert.Equal(t, "new@example.com", message.To)
	assert.Equal(t, "Confirm your new email address for RetroTide", message.Subject)
	assert.Contains(t, message.HTML, `href="`+escapedLink+`"`)
	assert.Contains(t, message.HTML, "The change takes effect once you confirm it.")
	assert.Contains(t, message.HTML, "If you did not ask to change your email address, ignore this email")
	assert.NotContains(t, message.HTML, "create this account")
}

func TestPasswordResetEmail(t *testing.T) {
	// given
	data := PasswordResetEmailData{To: "sam@example.com", SiteName: testSiteName, LinkURL: testLink}

	// when
	message, err := PasswordResetEmail(data)

	// then
	require.NoError(t, err)
	assertCompleteDocument(t, message)
	assert.Equal(t, "sam@example.com", message.To)
	assert.Equal(t, "Reset your RetroTide password", message.Subject)
	assert.Contains(t, message.HTML, `href="`+escapedLink+`"`)
	assert.Contains(t, message.HTML, "This link expires in one hour.")
	assert.Contains(t, message.HTML, "If you did not ask to reset your password, ignore this email")
}

func TestEmailChangedEmail(t *testing.T) {
	// given
	data := EmailChangedEmailData{To: "old@example.com", SiteName: testSiteName, NewEmail: "new@example.com", LinkURL: "https://retrotide.test/forgot-password"}

	// when
	message, err := EmailChangedEmail(data)

	// then
	require.NoError(t, err)
	assertCompleteDocument(t, message)
	assert.Equal(t, "old@example.com", message.To)
	assert.Equal(t, "Your RetroTide email address is being changed", message.Subject)
	assert.Contains(t, message.HTML, "<bdi>new@example.com</bdi>")
	assert.Contains(t, message.HTML, "until then your account keeps this one")
	assert.Contains(t, message.HTML, "Reset your password now, which also cancels the change.")
	assert.Contains(t, message.HTML, `href="https://retrotide.test/forgot-password"`)
	assert.Contains(t, message.HTML, "Reset your password")
}

func TestEmailChangedEmail_EscapesTheNewAddress(t *testing.T) {
	// given
	data := EmailChangedEmailData{To: "old@example.com", SiteName: testSiteName, NewEmail: `<script>alert(1)</script>@example.com`, LinkURL: "https://retrotide.test/forgot-password"}

	// when
	message, err := EmailChangedEmail(data)

	// then
	require.NoError(t, err)
	assert.NotContains(t, message.HTML, "<script>")
	assert.Contains(t, message.HTML, "&lt;script&gt;alert(1)&lt;/script&gt;@example.com")
}

func TestInvitationEmail(t *testing.T) {
	// given
	data := InvitationEmailData{To: "sam@example.com", SiteName: testSiteName, LinkURL: testLink}

	// when
	message, err := InvitationEmail(data)

	// then
	require.NoError(t, err)
	assertCompleteDocument(t, message)
	assert.Equal(t, "sam@example.com", message.To)
	assert.Equal(t, "You are invited to RetroTide", message.Subject)
	assert.Contains(t, message.HTML, `href="`+escapedLink+`"`)
	assert.Contains(t, message.HTML, "Create your account")
	assert.Contains(t, message.HTML, "This link expires in 7 days and works once.")
	assert.Contains(t, message.HTML, "If you were not expecting this invitation, ignore this email.")
}

func TestInvitationEmail_NeutralisesAScriptLink(t *testing.T) {
	// given
	data := InvitationEmailData{To: "sam@example.com", SiteName: testSiteName, LinkURL: "javascript:alert(1)"}

	// when
	message, err := InvitationEmail(data)

	// then
	require.NoError(t, err)
	assert.NotContains(t, message.HTML, `href="javascript:`)
}

func TestPasswordResetEmail_NeutralisesAScriptLink(t *testing.T) {
	// given
	data := PasswordResetEmailData{To: "sam@example.com", SiteName: testSiteName, LinkURL: "javascript:alert(1)"}

	// when
	message, err := PasswordResetEmail(data)

	// then
	require.NoError(t, err)
	assert.NotContains(t, message.HTML, `href="javascript:`)
}

func TestNotificationEmail(t *testing.T) {
	// given
	cases := []struct {
		name     string
		link     string
		wantLink bool
	}{
		{name: "with a link", link: "https://retrotide.test/actions", wantLink: true},
		{name: "without a link", link: "", wantLink: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			data := notificationEmailData{To: "sam@example.com", SiteName: testSiteName, Message: "You were assigned Fix the deploy script", LinkURL: tc.link}

			// when
			message, err := notificationEmail(data)

			// then
			require.NoError(t, err)
			assertCompleteDocument(t, message)
			assert.Equal(t, "sam@example.com", message.To)
			assert.Equal(t, "You have a new notification on RetroTide", message.Subject)
			assert.Contains(t, message.HTML, "You were assigned Fix the deploy script")

			if tc.wantLink {
				assert.Contains(t, message.HTML, `href="https://retrotide.test/actions"`)

				return
			}

			assert.NotContains(t, message.HTML, "href=")
		})
	}
}
