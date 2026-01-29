package config

import (
	"crypto/rand"
	"encoding/base64"
	"log"
	"os"

	"github.com/kelseyhightower/envconfig"
)

// Config holds the application configuration
type Config struct {
	Host string `envconfig:"HOST" default:"0.0.0.0"`
	Port int    `envconfig:"PORT" default:"8080"`

	// Provider base URLs
	OpenAIBaseURL    string `envconfig:"OPENAI_BASE_URL" default:"https://api.openai.com/v1"`
	AnthropicBaseURL string `envconfig:"ANTHROPIC_BASE_URL" default:"https://api.anthropic.com/v1"`
	GeminiBaseURL    string `envconfig:"GEMINI_BASE_URL" default:"https://generativelanguage.googleapis.com/v1beta"`

	// Database
	DatabaseURL string `envconfig:"DATABASE_URL" default:"data/ai_gateway.db"`

	// Security
	JWTSecret     string `envconfig:"JWT_SECRET"`
	EncryptionKey string `envconfig:"ENCRYPTION_KEY"`

	// JWT expiration in minutes
	JWTExpiration int `envconfig:"JWT_EXPIRATION" default:"60"`

	// HTTP timeout configuration
	HTTPTimeout   int `envconfig:"HTTP_TIMEOUT_SECONDS" default:"600"`    // 10 minutes
	StreamTimeout int `envconfig:"STREAM_TIMEOUT_SECONDS" default:"1800"` // 30 minutes for streaming
}

// Load loads the configuration from environment variables
func Load() (*Config, error) {
	var cfg Config

	// Load from environment
	if err := envconfig.Process("", &cfg); err != nil {
		return nil, err
	}

	// Generate JWT secret if not set
	if cfg.JWTSecret == "" {
		secret, err := generateRandomString(32)
		if err != nil {
			return nil, err
		}
		cfg.JWTSecret = secret
	}

	// Generate encryption key if not set
	if cfg.EncryptionKey == "" {
		key, err := generateRandomBytes(32)
		if err != nil {
			return nil, err
		}
		cfg.EncryptionKey = base64.StdEncoding.EncodeToString(key)
	}

	// Ensure data directory exists
	if err := os.MkdirAll("data", 0755); err != nil {
		return nil, err
	}

	// Debug: Print encryption key and JWT secret
	log.Printf("[CONFIG] ENCRYPTION_KEY loaded: %s", cfg.EncryptionKey)
	log.Printf("[CONFIG] JWT_SECRET loaded: %s", cfg.JWTSecret)

	return &cfg, nil
}

// generateRandomString generates a random string of the specified length
func generateRandomString(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(bytes)[:length], nil
}

// generateRandomBytes generates random bytes of the specified length
func generateRandomBytes(length int) ([]byte, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return nil, err
	}
	return bytes, nil
}

// GetEncryptionKeyBytes returns the encryption key as bytes
func (c *Config) GetEncryptionKeyBytes() ([]byte, error) {
	return base64.StdEncoding.DecodeString(c.EncryptionKey)
}
