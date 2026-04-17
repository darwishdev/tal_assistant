package config

import (
	"bytes"
	_ "embed"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

//go:embed app.env
var embeddedConfig []byte

//go:embed application_default_credentials.json
var embeddedCredentials []byte

type Config struct {
	GoogleProjectID       string `mapstructure:"GOOGLE_PROJECT_ID"`
	GoogleAPIKey          string `mapstructure:"GOOGLE_API_KEY"`
	GoogleCredentialsPath string `mapstructure:"GOOGLE_CREDENTIALS_PATH"`
	DevMode               bool   `mapstructure:"DEV_MODE"`

	RedisHost     string `mapstructure:"REDIS_HOST"`
	RedisPort     string `mapstructure:"REDIS_PORT"`
	RedisDB       int    `mapstructure:"REDIS_DB"`
	RedisPassword string `mapstructure:"REDIS_PASSWORD"`

	SignalingAgentURL    string `mapstructure:"SIGNALING_AGENT_URL"`
	NextQuestionAgentURL string `mapstructure:"NEXT_QUESTION_AGENT_URL"`

	ATSBaseURL string `mapstructure:"ATS_BASE_URL"`

	WorkableSubdomain string `mapstructure:"WORKABLE_SUBDOMAIN"`
	WorkableToken     string `mapstructure:"WORKABLE_TOKEN"`

	RedisAddress string `mapstructure:"REDIS_ADDRESS"`
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
	// homeDir, _ := os.UserHomeDir()

	p := "./config/app.env"
	viper.SetConfigFile(p)
	if err := viper.ReadInConfig(); err != nil {
		log.Panic(fmt.Errorf("warning: could not read override config %s: %v", p, err))
	}

	if err := viper.Unmarshal(&AppConfig); err != nil {
		log.Fatalf("Unable to decode config: %v", err)
	}

	// AppConfig.RedisAddress = AppConfig.RedisHost + ":" + AppConfig.RedisPort

	// Resolve credentials path relative to executable if it's a relative path
	if AppConfig.GoogleCredentialsPath != "" && !filepath.IsAbs(AppConfig.GoogleCredentialsPath) {
		exePath, err := os.Executable()
		if err == nil {
			exeDir := filepath.Dir(exePath)
			resolvedPath := filepath.Join(exeDir, AppConfig.GoogleCredentialsPath)
			if _, err := os.Stat(resolvedPath); err == nil {
				AppConfig.GoogleCredentialsPath = resolvedPath
				log.Printf("Resolved credentials path to: %s", resolvedPath)
			} else {
				log.Printf("Warning: credentials file not found at %s", resolvedPath)
			}
		}
	}

	return &AppConfig
}

// GetEmbeddedCredentials returns the embedded Google Cloud credentials JSON.
// Returns nil if no credentials were embedded.
func GetEmbeddedCredentials() []byte {
	if len(embeddedCredentials) == 0 {
		return nil
	}
	return embeddedCredentials
}
