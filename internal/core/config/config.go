package config

import (
	"log/slog"
	"os"

	"github.com/ilyakaznacheev/cleanenv"
)

type Config struct {
	Env      string `env:"ENV" envDefault:"local"`
	Telegram TelegramConfig
	Redis    RedisConfig
	Site     SiteConfig
}
type TelegramConfig struct {
	Token string `env:"TELEGRAM_TOKEN" env-required:"true"`
}
type RedisConfig struct {
	URL string `env:"REDIS_URL" env-required:"true"`
}
type SiteConfig struct {
	WgGeshuchtURL    string `env:"WGGESHUCHT_URL" env-required:"true"`
	KleinanzeigenURL string `env:"KLEINANZEIGEN_URL" env-required:"true"`
}

func MustLoad(logger *slog.Logger) *Config {
	cfgPath := ".env"
	var cfg Config
	if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
		logger.Info("config file not found")
		if err := cleanenv.ReadEnv(&cfg); err != nil {
			logger.Error("failed to read env", "err", err)
			os.Exit(1)
		}
		return &cfg

	}
	if err := cleanenv.ReadConfig(cfgPath, &cfg); err != nil {
		logger.Error("failed to read config", "err", err)
		os.Exit(1)
	}
	return &cfg
}
