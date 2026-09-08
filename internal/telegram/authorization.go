package telegram

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/dakalab/tg-cleaner/internal/config"
	"github.com/zelenin/go-tdlib/client"
)

type authorizationHandler struct {
	ctx         context.Context
	parameters  *client.SetTdlibParametersRequest
	credentials config.TelegramConfig
	input       *bufio.Reader
	output      io.Writer
}

func newAuthorizationHandler(ctx context.Context, parameters *client.SetTdlibParametersRequest, credentials config.TelegramConfig, input io.Reader, output io.Writer) *authorizationHandler {
	return &authorizationHandler{ctx: ctx, parameters: parameters, credentials: credentials, input: bufio.NewReader(input), output: output}
}

func (handler *authorizationHandler) Handle(tdlibClient *client.Client, state client.AuthorizationState) error {
	switch state.AuthorizationStateConstructor() {
	case client.ConstructorAuthorizationStateWaitTdlibParameters:
		_, err := tdlibClient.SetTdlibParameters(handler.ctx, handler.parameters)
		return err
	case client.ConstructorAuthorizationStateWaitPhoneNumber:
		_, err := tdlibClient.SetAuthenticationPhoneNumber(handler.ctx, &client.SetAuthenticationPhoneNumberRequest{
			PhoneNumber: handler.credentials.Phone,
			Settings:    &client.PhoneNumberAuthenticationSettings{},
		})
		return err
	case client.ConstructorAuthorizationStateWaitCode:
		code, err := handler.authenticationValue("Enter Telegram verification code: ", handler.credentials.Phone, true)
		if err != nil {
			return err
		}
		_, err = tdlibClient.CheckAuthenticationCode(handler.ctx, &client.CheckAuthenticationCodeRequest{Code: code})
		return err
	case client.ConstructorAuthorizationStateWaitPassword:
		password, err := handler.authenticationValue("Enter Telegram 2FA password: ", handler.credentials.Password, false)
		if err != nil {
			return err
		}
		_, err = tdlibClient.CheckAuthenticationPassword(handler.ctx, &client.CheckAuthenticationPasswordRequest{Password: password})
		return err
	case client.ConstructorAuthorizationStateReady, client.ConstructorAuthorizationStateClosing, client.ConstructorAuthorizationStateClosed:
		return nil
	default:
		return client.NotSupportedAuthorizationState(state)
	}
}

func (handler *authorizationHandler) authenticationValue(prompt, configured string, alwaysPrompt bool) (string, error) {
	if configured != "" && !alwaysPrompt {
		return configured, nil
	}
	if _, err := fmt.Fprint(handler.output, prompt); err != nil {
		return "", fmt.Errorf("write Telegram authentication prompt: %w", err)
	}

	type readResult struct {
		value string
		err   error
	}
	result := make(chan readResult, 1)
	go func() {
		value, err := handler.input.ReadString('\n')
		result <- readResult{value: value, err: err}
	}()

	select {
	case <-handler.ctx.Done():
		return "", handler.ctx.Err()
	case read := <-result:
		if read.err != nil && !errors.Is(read.err, io.EOF) {
			return "", fmt.Errorf("read Telegram authentication value: %w", read.err)
		}
		value := strings.TrimSpace(read.value)
		if value == "" {
			return "", errors.New("Telegram authentication value is required")
		}
		return value, nil
	}
}

func (*authorizationHandler) Close() {}
