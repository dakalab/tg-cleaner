package config

import (
	"errors"
	"fmt"

	"github.com/spf13/viper"
)

type Config struct {
	Telegram TelegramConfig `mapstructure:"telegram"`
}

type TelegramConfig struct {
	AppID    int32  `mapstructure:"app_id"`
	AppHash  string `mapstructure:"app_hash"`
	Phone    string `mapstructure:"phone"`
	Password string `mapstructure:"password"`
}

func Load(path string) (Config, error) {
	instance := viper.New()
	instance.SetConfigFile(path)
	instance.SetConfigType("yaml")
	if err := instance.ReadInConfig(); err != nil {
		return Config{}, fmt.Errorf("read configuration %q: %w", path, err)
	}

	var configuration Config
	if err := instance.Unmarshal(&configuration); err != nil {
		return Config{}, fmt.Errorf("decode configuration %q: %w", path, err)
	}
	return configuration, nil
}

func (configuration Config) Validate() error {
	switch {
	case configuration.Telegram.AppID <= 0:
		return errors.New("telegram.app_id must be a positive integer")
	case configuration.Telegram.AppHash == "":
		return errors.New("telegram.app_hash is required")
	case configuration.Telegram.Phone == "":
		return errors.New("telegram.phone is required")
	default:
		return nil
	}
}
