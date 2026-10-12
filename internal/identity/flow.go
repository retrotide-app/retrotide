package identity

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"

	"retrotide/internal/dao"
	"retrotide/internal/model"
	"retrotide/internal/model/spec"
	"retrotide/internal/session"
	"retrotide/internal/token"

	"github.com/google/uuid"
)

type (
	flowSecrets struct {
		cookieToken   string
		authorisation Authorisation
	}
)

func drawFlowSecrets() (flowSecrets, error) {
	var drawn flowSecrets
	for _, secret := range []*string{&drawn.cookieToken, &drawn.authorisation.State, &drawn.authorisation.Nonce, &drawn.authorisation.Verifier} {
		value, err := token.New()
		if err != nil {
			return flowSecrets{}, fmt.Errorf("draw the flow secrets: %w", err)
		}

		*secret = value
	}

	return drawn, nil
}

func sameSecret(presented string, stored string) bool {
	if presented == "" || stored == "" {
		return false
	}

	return subtle.ConstantTimeCompare([]byte(presented), []byte(stored)) == 1
}

func (s *service) claimFlow(ctx context.Context, callback spec.IdentityCallback) (*model.OIDCFlow, error) {
	if callback.FlowToken == "" {
		return nil, errFlowExpired
	}

	flow, err := s.flows.Consume(ctx, spec.OIDCFlowKey{TokenHash: token.Hash(callback.FlowToken), Provider: callback.Provider})
	if errors.Is(err, dao.ErrNotFound) {
		return nil, fmt.Errorf("%w: %w", errFlowExpired, err)
	}
	if err != nil {
		return nil, fmt.Errorf("consume the %s flow: %w", callback.Provider, err)
	}

	if !sameSecret(callback.State, flow.State) {
		return nil, errFlowExpired
	}

	return flow, nil
}

func (s *service) boundUser(ctx context.Context, flow model.OIDCFlow, presented string) (uuid.UUID, error) {
	if flow.UserID == nil || flow.SessionTokenHash == nil || presented == "" {
		return uuid.Nil, errSessionChanged
	}

	if !sameSecret(token.Hash(presented), *flow.SessionTokenHash) {
		return uuid.Nil, errSessionChanged
	}

	userID, err := s.sessions.Validate(ctx, presented)
	if errors.Is(err, session.ErrInvalidSession) {
		return uuid.Nil, fmt.Errorf("%w: %w", errSessionChanged, err)
	}
	if err != nil {
		return uuid.Nil, fmt.Errorf("validate the session that started the link: %w", err)
	}

	if userID != *flow.UserID {
		return uuid.Nil, errSessionChanged
	}

	return userID, nil
}
