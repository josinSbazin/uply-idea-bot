package config

import (
	"log"
	"strconv"
	"strings"
	"sync"

	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

type Config struct {
	Telegram struct {
		BotToken      string  `mapstructure:"bot_token"`
		AllowedGroups []int64 `mapstructure:"-"`
	} `mapstructure:"telegram"`

	Claude struct {
		APIKey           string `mapstructure:"api_key"`
		Model            string `mapstructure:"model"`
		SystemPromptFile string `mapstructure:"system_prompt_file"`
	} `mapstructure:"claude"`

	Web struct {
		Port     string `mapstructure:"port"`
		Username string `mapstructure:"username"`
		Password string `mapstructure:"password"`
		BaseURL  string `mapstructure:"base_url"`
	} `mapstructure:"web"`

	SQLite struct {
		Path string `mapstructure:"path"`
	} `mapstructure:"sqlite"`

	RateLimit struct {
		PerUser int `mapstructure:"per_user"`
		Global  int `mapstructure:"global"`
	} `mapstructure:"rate_limit"`

	Env string `mapstructure:"env"`
}

var (
	instance *Config
	once     sync.Once
)

func Load() {
	once.Do(func() {
		_ = godotenv.Load()

		viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
		viper.AutomaticEnv()

		// Defaults
		viper.SetDefault("web.port", "8080")
		viper.SetDefault("sqlite.path", "./ideas.db")
		viper.SetDefault("claude.model", "claude-sonnet-4-20250514")
		viper.SetDefault("rate_limit.per_user", 5)
		viper.SetDefault("rate_limit.global", 50)
		viper.SetDefault("env", "prod")
		viper.SetDefault("web.base_url", "http://localhost:8080")

		// Bind environment variables
		viper.BindEnv("telegram.bot_token", "TELEGRAM_BOT_TOKEN")
		viper.BindEnv("claude.api_key", "ANTHROPIC_API_KEY")
		viper.BindEnv("claude.model", "CLAUDE_MODEL")
		viper.BindEnv("claude.system_prompt_file", "SYSTEM_PROMPT_FILE")
		viper.BindEnv("web.port", "WEB_PORT")
		viper.BindEnv("web.username", "WEB_USERNAME")
		viper.BindEnv("web.password", "WEB_PASSWORD")
		viper.BindEnv("web.base_url", "WEB_BASE_URL")
		viper.BindEnv("sqlite.path", "SQLITE_PATH")
		viper.BindEnv("rate_limit.per_user", "RATE_LIMIT_PER_USER")
		viper.BindEnv("rate_limit.global", "RATE_LIMIT_GLOBAL")
		viper.BindEnv("env", "GO_ENV")

		instance = &Config{}
		if err := viper.Unmarshal(instance); err != nil {
			log.Fatalf("Failed to unmarshal config: %v", err)
		}

		// Parse allowed groups
		groupsStr := viper.GetString("TELEGRAM_ALLOWED_GROUPS")
		if groupsStr != "" {
			for _, g := range strings.Split(groupsStr, ",") {
				g = strings.TrimSpace(g)
				if g == "" {
					continue
				}
				id, err := strconv.ParseInt(g, 10, 64)
				if err != nil {
					log.Printf("Warning: invalid group ID %q: %v", g, err)
					continue
				}
				instance.Telegram.AllowedGroups = append(instance.Telegram.AllowedGroups, id)
			}
		}
	})
}

func Get() *Config {
	return instance
}

func (c *Config) IsProd() bool {
	return c.Env == "prod"
}

func (c *Config) IsDev() bool {
	return c.Env == "dev"
}
