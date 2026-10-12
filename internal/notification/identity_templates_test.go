package notification

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testSignInMethodsURL = "https://retrotide.test/account#sign-in-methods"
	testResetURL         = "https://retrotide.test/forgot-password"
	passwordWarning      = "If you did not set a password here, reset it now."
)

func linkedData() IdentityLinkedEmailData {
	return IdentityLinkedEmailData{
		To:               "sam@gmail.com",
		SiteName:         testSiteName,
		Provider:         "Google",
		IdentityEmail:    "sam@gmail.com",
		AccountCreatedOn: "3 March 2026",
		SignInMethodsURL: testSignInMethodsURL,
		ResetPasswordURL: testResetURL,
	}
}

func TestIdentityLinkedEmail_ForAnAutomaticLinkOnAnAccountWithAPassword(t *testing.T) {
	// given
	data := linkedData()
	data.Automatic = true
	data.HasPassword = true

	// when
	message, err := IdentityLinkedEmail(data)

	// then
	require.NoError(t, err)
	assertCompleteDocument(t, message)
	assert.Equal(t, "sam@gmail.com", message.To)
	assert.Equal(t, "Google sign-in was linked to your RetroTide account", message.Subject)
	assert.Contains(t, message.HTML, "<bdi>sam@gmail.com</bdi>")
	assert.Contains(t, message.HTML, "linked automatically")
	assert.Contains(t, message.HTML, "every other device signed in to your account was signed out")
	assert.Contains(t, message.HTML, `href="`+testSignInMethodsURL+`"`)
	assert.Contains(t, message.HTML, "Your account also has a password.")
	assert.Contains(t, message.HTML, "3 March 2026")
	assert.Contains(t, message.HTML, passwordWarning)
	assert.Contains(t, message.HTML, `href="`+testResetURL+`"`)
}

func TestIdentityLinkedEmail_FromAccountSettings(t *testing.T) {
	// given
	data := linkedData()
	data.HasPassword = true

	// when
	message, err := IdentityLinkedEmail(data)

	// then
	require.NoError(t, err)
	assertCompleteDocument(t, message)
	assert.Contains(t, message.HTML, "from your account settings")
	assert.NotContains(t, message.HTML, "linked automatically")
	assert.Contains(t, message.HTML, `href="`+testSignInMethodsURL+`"`)
	assert.Contains(t, message.HTML, passwordWarning)
}

func TestIdentityLinkedEmail_WithoutAPasswordCarriesNoPasswordWarning(t *testing.T) {
	// given
	data := linkedData()
	data.Automatic = true

	// when
	message, err := IdentityLinkedEmail(data)

	// then
	require.NoError(t, err)
	assert.Contains(t, message.HTML, `href="`+testSignInMethodsURL+`"`)
	assert.NotContains(t, message.HTML, "also has a password")
	assert.NotContains(t, message.HTML, passwordWarning)
	assert.NotContains(t, message.HTML, testResetURL)
	assert.NotContains(t, message.HTML, "3 March 2026")
}

func TestIdentityLinkedEmail_EscapesTheIdentityAddress(t *testing.T) {
	// given
	data := linkedData()
	data.IdentityEmail = `<script>alert(1)</script>@gmail.com`

	// when
	message, err := IdentityLinkedEmail(data)

	// then
	require.NoError(t, err)
	assert.NotContains(t, message.HTML, "<script>")
	assert.Contains(t, message.HTML, "&lt;script&gt;alert(1)&lt;/script&gt;@gmail.com")
}

func TestIdentityUnlinkedEmail(t *testing.T) {
	// given
	data := IdentityUnlinkedEmailData{To: "sam@example.com", SiteName: testSiteName, Provider: "Google", IdentityEmail: "sam@gmail.com", ResetPasswordURL: testResetURL}

	// when
	message, err := IdentityUnlinkedEmail(data)

	// then
	require.NoError(t, err)
	assertCompleteDocument(t, message)
	assert.Equal(t, "sam@example.com", message.To)
	assert.Equal(t, "Google sign-in was removed from your RetroTide account", message.Subject)
	assert.Contains(t, message.HTML, "<bdi>sam@gmail.com</bdi>")
	assert.Contains(t, message.HTML, "can no longer sign in to it")
	assert.Contains(t, message.HTML, "Reset your password now, which signs you out everywhere.")
	assert.Contains(t, message.HTML, `href="`+testResetURL+`"`)
}
