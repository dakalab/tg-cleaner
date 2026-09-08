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
	}

	run := func(confirm *bool) func(*cobra.Command, []string) error {
		return func(command *cobra.Command, _ []string) error {
			ctx, stop := signal.NotifyContext(command.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()

			return runTelegram(ctx, configuration.Telegram, *confirm, command.InOrStdin(), command.OutOrStdout())
		}
	}

	command.PersistentFlags().StringVar(&configPath, "config", defaultConfigPath, "path to the YAML configuration file")

	var rootConfirm bool
	command.RunE = run(&rootConfirm)
	command.Flags().BoolVar(&rootConfirm, "confirm", false, "leave all discovered non-admin channels and groups")

	var leaveConfirm bool
	leaveCommand := &cobra.Command{
		Use:   "leave",
		Short: "Leave channels and groups where this account is not an administrator",
		Args:  cobra.NoArgs,
		RunE:  run(&leaveConfirm),
	}
	leaveCommand.Flags().BoolVar(&leaveConfirm, "confirm", false, "leave all discovered non-admin channels and groups")
	command.AddCommand(leaveCommand)

	return command
}
