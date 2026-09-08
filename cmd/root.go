package cmd

import (
	"context"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/dakalab/tg-cleaner/internal/config"
	"github.com/dakalab/tg-cleaner/internal/telegram"
	"github.com/spf13/cobra"
)

const defaultConfigPath = "config.yaml"

type telegramRunner func(context.Context, config.TelegramConfig, bool, io.Reader, io.Writer) error

func Execute() error {
	return newRootCommand().Execute()
}

func newRootCommand() *cobra.Command {
	return newRootCommandWithRunner(telegram.Run)
}

func newRootCommandWithRunner(runTelegram telegramRunner) *cobra.Command {
	var configPath string
	var confirm bool
	var configuration config.Config

	command := &cobra.Command{
		Use:           "tg-cleaner",
		Short:         "Leave Telegram channels and groups",
		SilenceErrors: true,
		SilenceUsage:  true,
		PersistentPreRunE: func(_ *cobra.Command, _ []string) error {
			loaded, err := config.Load(configPath)
			if err != nil {
				return err
			}
			if err := loaded.Validate(); err != nil {
				return err
			}
			configuration = loaded
			return nil
		},
		RunE: func(command *cobra.Command, _ []string) error {
			ctx, stop := signal.NotifyContext(command.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()

			return runTelegram(ctx, configuration.Telegram, confirm, command.InOrStdin(), command.OutOrStdout())
		},
	}

	command.PersistentFlags().StringVar(&configPath, "config", defaultConfigPath, "path to the YAML configuration file")
	command.Flags().BoolVar(&confirm, "confirm", false, "leave all discovered channels and groups")
	return command
}
