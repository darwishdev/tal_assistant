package config

import (
	"bytes"
	_ "embed"
	"log"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

//go:embed app.env
var embeddedConfig []byte

type Config struct {
	GoogleProjectID string `mapstructure:"GOOGLE_PROJECT_ID"`
	GoogleAPIKey    string `mapstructure:"GOOGLE_API_KEY"`

	RedisHost     string `mapstructure:"REDIS_HOST"`
	RedisPort     string `mapstructure:"REDIS_PORT"`
	RedisDB       int    `mapstructure:"REDIS_DB"`
	RedisPassword string `mapstructure:"REDIS_PASSWORD"`

	SignalingAgentURL    string `mapstructure:"SIGNALING_AGENT_URL"`
	NextQuestionAgentURL string `mapstructure:"NEXT_QUESTION_AGENT_URL"`

	ATSBaseURL string `mapstructure:"ATS_BASE_URL"`

	RedisAddress string
}

var AppConfig Config

func Load() *Config {
	viper.SetConfigType("env")
	viper.AutomaticEnv()

	// Load embedded defaults — binary works standalone without any file.
	if err := viper.ReadConfig(bytes.NewReader(embeddedConfig)); err != nil {
		log.Printf("warning: could not parse embedded config: %v", err)
	}

	// Optionally override with a local file (useful during development).
	homeDir, _ := os.UserHomeDir()
	overrides := []string{
		"./config/app.env",
		filepath.Join(homeDir, ".config", "tal_assistant", "app.env"),
	}
	for _, p := range overrides {
		if _, err := os.Stat(p); err == nil {
			viper.SetConfigFile(p)
			if err := viper.ReadInConfig(); err != nil {
				log.Printf("warning: could not read override config %s: %v", p, err)
			} else {
				log.Println("Config overridden from:", p)
			}
			break
		}
	}

	if err := viper.Unmarshal(&AppConfig); err != nil {
		log.Fatalf("Unable to decode config: %v", err)
	}

	AppConfig.RedisAddress = AppConfig.RedisHost + ":" + AppConfig.RedisPort
	return &AppConfig
}
