package config

import (
	"log"

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

	ATSBaseURL string `mapstructure:"ATS_BASE_URL"`

	RedisAddress string
}

var AppConfig Config

func Load() *Config {

	viper.SetConfigType("env")
	viper.SetConfigFile("./config/app.env")

	if err := viper.ReadInConfig(); err != nil {
		log.Fatalf("failed reading config ./config/app.env: %v", err)
	}
	log.Println("Loaded config from: ./config/app.env")

	if err := viper.Unmarshal(&AppConfig); err != nil {
		log.Fatalf("Unable to decode config: %v", err)
	}

	// build derived values
	AppConfig.RedisAddress = AppConfig.RedisHost + ":" + AppConfig.RedisPort

	return &AppConfig
}
