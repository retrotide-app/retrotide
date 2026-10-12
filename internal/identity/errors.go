package identity

import (
	"errors"

	"retrotide/internal/repository"
)

type (
	refusal struct {
		reason error
		code   string
	}
)

const (
	CodeCancelled              = "cancelled"
	CodeExpired                = "expired"
	CodeEmailUnverified        = "email_unverified"
	CodeEmailNotAuthoritative  = "email_not_authoritative"
	CodeLinkRequired           = "link_required"
	CodeIdentityInUse          = "identity_in_use"
	CodeAlreadyLinked          = "already_linked"
	CodeAccountBanned          = "account_banned"
	CodeAccountUnverified      = "account_unverified"
	CodeAccountNotAllowed      = "account_not_allowed"
	CodeReauthenticationFailed = "reauthentication_failed"
	CodeFailed                 = "failed"
)

var (
	ErrProviderNotConfigured = errors.New("the identity provider is not configured")
	ErrProviderUnavailable   = errors.New("the identity provider could not be discovered")
	ErrNoIDToken             = errors.New("the token response carried no id token")
	ErrNonceMismatch         = errors.New("the id token's nonce does not match the flow's")
	ErrDuplicateProvider     = errors.New("two identity providers share one id")
	ErrAccountNotAllowed     = errors.New("this account may not sign in to this site")

	ErrUnknownPurpose    = errors.New("the purpose must be sign_in or link")
	ErrSignInRequired    = errors.New("linking a sign-in method needs a signed-in account")
	ErrAccountLocked     = errors.New("your account is locked")
	ErrAccountUnverified = errors.New("verify your email address before linking a sign-in method")
	ErrIdentityNotFound  = errors.New("the account has no identity from that provider")
	ErrLastSignInMethod  = errors.New("that is the account's last way to sign in")
	ErrNotAudited        = errors.New("the change was made, but its audit entry was not written")
	ErrNoticeNotSent     = errors.New("the change was made, but its notice email was not sent")

	ErrReauthenticationRequired = errors.New("confirm the link by signing in with a sign-in method your account already has")

	errFlowExpired            = errors.New("the sign-in flow is missing, used, expired, for another provider or for another state")
	errSessionChanged         = errors.New("the browser no longer presents the session that started the link")
	errCancelled              = errors.New("the sign-in was cancelled at the identity provider")
	errEmailUnverified        = errors.New("the identity provider has not verified the email address")
	errEmailNotAuthoritative  = errors.New("the identity provider is not authoritative for the email address")
	errLinkRequired           = errors.New("the matching account must link the identity from its account settings")
	errAccountBanned          = errors.New("the account is banned")
	errAccountUnverified      = errors.New("the account's email address is not verified")
	errReauthenticationFailed = errors.New("the account that confirmed the link is not one linked to this account")

	refusals = []refusal{
		{reason: errFlowExpired, code: CodeExpired},
		{reason: errSessionChanged, code: CodeExpired},
		{reason: errCancelled, code: CodeCancelled},
		{reason: errEmailUnverified, code: CodeEmailUnverified},
		{reason: errEmailNotAuthoritative, code: CodeEmailNotAuthoritative},
		{reason: errLinkRequired, code: CodeLinkRequired},
		{reason: repository.ErrAccountChanged, code: CodeLinkRequired},
		{reason: repository.ErrIdentityInUse, code: CodeIdentityInUse},
		{reason: repository.ErrAlreadyLinked, code: CodeAlreadyLinked},
		{reason: errAccountBanned, code: CodeAccountBanned},
		{reason: repository.ErrAccountBanned, code: CodeAccountBanned},
		{reason: errAccountUnverified, code: CodeAccountUnverified},
		{reason: ErrAccountNotAllowed, code: CodeAccountNotAllowed},
		{reason: errReauthenticationFailed, code: CodeReauthenticationFailed},
	}
)
