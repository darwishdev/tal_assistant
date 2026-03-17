package config

import (
	"log"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

type Config struct {
	GoogleProjectID string `mapstructure:"GOOGLE_PROJECT_ID"`
	GoogleAPIKey    string `mapstructure:"GOOGLE_API_KEY"`

	RedisHost     string `mapstructure:"REDIS_HOST"`
	RedisPort     string `mapstructure:"REDIS_PORT"`
	RedisDB       int    `mapstructure:"REDIS_DB"`
	RedisPassword string `mapstructure:"REDIS_PASSWORD"`

	SignalingAgentURL    string `mapstructure:"SIGNALING_AGENT_URL"`
	NextQuestionAgentURL string `mapstructure:"NEXT_QUESTION_AGENT_URL"`

	RedisAddress string
}

var AppConfig Config

func Load() *Config {

	viper.SetConfigType("env")
	viper.AutomaticEnv()

	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Fatalf("cannot determine home directory: %v", err)
	}

	paths := []string{
		"./config/app.env",
		filepath.Join(homeDir, ".config", "tal_assistant", "app.env"),
	}

	var loadedPath string

	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			viper.SetConfigFile(p)

			if err := viper.ReadInConfig(); err != nil {
				log.Fatalf("failed reading config %s: %v", p, err)
			}

			loadedPath = p
			break
		}
	}

	if loadedPath == "" {
		log.Println("No app.env file found, relying only on environment variables")
	} else {
		log.Println("Loaded config from:", loadedPath)
	}

	if err := viper.Unmarshal(&AppConfig); err != nil {
		log.Fatalf("Unable to decode config: %v", err)
	}

	// build derived values
	AppConfig.RedisAddress = AppConfig.RedisHost + ":" + AppConfig.RedisPort

	return &AppConfig
}
