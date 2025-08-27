package internal

import (
	"fmt"
	"os"
)

// Config holds the application configuration
type Config struct {
	// DynamoDB configuration
	DynamoDBRegion string
	TableName      string // Unified table name

	// Discord configuration
	DiscordPublicKey string
}

// LoadConfigFromEnv loads configuration from environment variables
func LoadConfigFromEnv() (*Config, error) {
	config := &Config{}

	// DynamoDB configuration
	config.DynamoDBRegion = getEnvOrDefault("DYNAMODB_REGION", "us-east-1")
	config.TableName = getEnvOrDefault("TABLE_NAME", "artemis-data")

	// Discord configuration
	config.DiscordPublicKey = getEnvOrFail("DISCORD_PUBLIC_KEY")

	return config, nil
}

// getEnvOrDefault gets an environment variable or returns a default value
func getEnvOrDefault(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

// getEnvOrFail gets an environment variable or fails if not found
func getEnvOrFail(key string) string {
	value := os.Getenv(key)
	if value == "" {
		panic(fmt.Sprintf("required environment variable %s not set", key))
	}
	return value
}
