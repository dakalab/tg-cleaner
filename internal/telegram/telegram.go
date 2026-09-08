package telegram

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/dakalab/tg-cleaner/internal/config"
	"github.com/zelenin/go-tdlib/client"
)

const closeTimeout = 10 * time.Second

func Run(ctx context.Context, credentials config.TelegramConfig, confirm bool, input io.Reader, output io.Writer) error {
	databaseDirectory := filepath.Join(".tdlib", "database")
	filesDirectory := filepath.Join(".tdlib", "files")
	for _, directory := range []string{databaseDirectory, filesDirectory} {
		if err := os.MkdirAll(directory, 0o700); err != nil {
			return fmt.Errorf("create Telegram session directory %q: %w", directory, err)
		}
	}

	parameters := &client.SetTdlibParametersRequest{
		DatabaseDirectory: databaseDirectory, FilesDirectory: filesDirectory,
		UseFileDatabase: true, UseChatInfoDatabase: true, UseMessageDatabase: true,
		ApiId: credentials.AppID, ApiHash: credentials.AppHash, SystemLanguageCode: "en",
		DeviceModel: "tg-cleaner", SystemVersion: "1.0.0", ApplicationVersion: "1.0.0",
	}
	if _, err := client.SetLogVerbosityLevel(&client.SetLogVerbosityLevelRequest{NewVerbosityLevel: 1}); err != nil {
		return fmt.Errorf("set TDLib log verbosity: %w", err)
	}

	tdlibClient, err := client.NewClient(newAuthorizationHandler(ctx, parameters, credentials, input, output))
	if err != nil {
		return fmt.Errorf("authenticate Telegram account: %w", err)
	}

	if err := cleanChats(ctx, tdlibClient, confirm, output); err != nil {
		return closeAfterError(tdlibClient, err)
	}
	return closeClient(tdlibClient)
}

func closeClient(tdlibClient *client.Client) error {
	ctx, cancel := context.WithTimeout(context.Background(), closeTimeout)
	defer cancel()
	if _, err := tdlibClient.Close(ctx); err != nil {
		return fmt.Errorf("close Telegram client: %w", err)
	}
	return nil
}

func closeAfterError(tdlibClient *client.Client, runError error) error {
	if err := closeClient(tdlibClient); err != nil {
		return fmt.Errorf("%w; %v", runError, err)
	}
	return runError
}
