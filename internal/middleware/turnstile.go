package middleware

import (
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"retrotide/internal/config"
	"retrotide/internal/dto"
	"retrotide/internal/logger"
	"retrotide/internal/settings"

	"github.com/gin-gonic/gin"
)

type (
	turnstileBody struct {
		TurnstileToken string `json:"turnstile_token"`
	}

	turnstileVerdict struct {
		Success    bool     `json:"success"`
		ErrorCodes []string `json:"error-codes"`
		Hostname   string   `json:"hostname"`
	}
)

const (
	TurnstileVerifyURL = "https://challenges.cloudflare.com/turnstile/v0/siteverify"

	turnstileTimeout            = 5 * time.Second
	turnstileRequiredMessage    = "verification required"
	turnstileFailedMessage      = "verification failed, please try again"
	turnstileUnavailableMessage = "verification is temporarily unavailable, please try again later"
	bodyTooLargeMessage         = "request body too large"
)

var (
	turnstileServerErrorCodes = []string{"missing-input-secret", "invalid-input-secret", "internal-error"}
)

func RequireTurnstile(settingsSvc settings.Service, verifyURL string) gin.HandlerFunc {
	client := &http.Client{Timeout: turnstileTimeout}

	return func(c *gin.Context) {
		ctx := c.Request.Context()

		enabled, err := settingsSvc.GetBool(ctx, config.SettingTurnstileEnabled)
		if err != nil {
			abortUnreadableSetting(c, err)
			return
		}

		if !enabled {
			return
		}

		secret, err := settingsSvc.Get(ctx, config.SettingTurnstileSecretKey)
		if err != nil {
			abortUnreadableSetting(c, err)
			return
		}

		if secret == "" {
			c.Error(errTurnstileSecretMissing)
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, dto.ErrorResponse{Error: turnstileUnavailableMessage})
			return
		}

		var body turnstileBody
		err = c.ShouldBindBodyWithJSON(&body)
		if _, tooLarge := errors.AsType[*http.MaxBytesError](err); tooLarge {
			c.AbortWithStatusJSON(http.StatusRequestEntityTooLarge, dto.ErrorResponse{Error: bodyTooLargeMessage})
			return
		}

		if err != nil || body.TurnstileToken == "" {
			c.AbortWithStatusJSON(http.StatusBadRequest, dto.ErrorResponse{Error: turnstileRequiredMessage})
			return
		}

		verdict, err := verifyTurnstile(ctx, client, verifyURL, secret, body.TurnstileToken)
		if err != nil {
			c.Error(err)
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, dto.ErrorResponse{Error: turnstileUnavailableMessage})
			return
		}

		if verdict.Success {
			return
		}

		serverSide := slices.ContainsFunc(verdict.ErrorCodes, func(code string) bool {
			return slices.Contains(turnstileServerErrorCodes, code)
		})
		if serverSide {
			c.Error(fmt.Errorf("turnstile refused the check on its own side: %s", strings.Join(verdict.ErrorCodes, ", ")))
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, dto.ErrorResponse{Error: turnstileUnavailableMessage})
			return
		}

		logger.Ctx(ctx).Warn().Strs("error_codes", verdict.ErrorCodes).Str("hostname", verdict.Hostname).Str("route", routeLabel(c)).Msg("turnstile verification refused")
		c.AbortWithStatusJSON(http.StatusBadRequest, dto.ErrorResponse{Error: turnstileFailedMessage})
	}
}

func verifyTurnstile(ctx context.Context, client *http.Client, verifyURL string, secret string, token string) (turnstileVerdict, error) {
	form := url.Values{"secret": {secret}, "response": {token}}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, verifyURL, strings.NewReader(form.Encode()))
	if err != nil {
		return turnstileVerdict{}, fmt.Errorf("build the turnstile siteverify request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := client.Do(req)
	if err != nil {
		return turnstileVerdict{}, fmt.Errorf("call turnstile siteverify: %w", err)
	}

	verdict, err := readTurnstileVerdict(resp)
	if closeErr := resp.Body.Close(); closeErr != nil {
		err = errors.Join(err, fmt.Errorf("close the turnstile siteverify answer: %w", closeErr))
	}

	return verdict, err
}

func readTurnstileVerdict(resp *http.Response) (turnstileVerdict, error) {
	if resp.StatusCode != http.StatusOK {
		return turnstileVerdict{}, fmt.Errorf("turnstile siteverify answered %s", resp.Status)
	}

	var verdict turnstileVerdict
	if err := json.UnmarshalRead(resp.Body, &verdict); err != nil {
		return turnstileVerdict{}, fmt.Errorf("decode the turnstile siteverify answer: %w", err)
	}

	return verdict, nil
}
