package main

import (
	"context"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/vignesh-goutham/artemis/trading-bot/internal"
)

// Lambda handler for AWS Lambda triggered by EventBridge Scheduler
func handler(ctx context.Context, request events.CloudWatchEvent) error {
	log.Printf("Artemis Trading Bot triggered by EventBridge Scheduler: %s", request.ID)

	// Load configuration from environment variables
	config, err := loadConfigFromEnv()
	if err != nil {
		log.Printf("Failed to load configuration: %v", err)
		return err
	}

	// Create trading bot
	bot, err := internal.NewTradingBot(config)
	if err != nil {
		log.Printf("Failed to create trading bot: %v", err)
		return err
	}

	// Create context with timeout (5 minutes for Lambda)
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	// Run the trading bot
	err = bot.Run(ctx)
	if err != nil {
		log.Printf("Trading bot failed: %v", err)
		return err
	}

	log.Println("Artemis Trading Bot completed successfully")
	return nil
}

// loadConfigFromEnv loads configuration from environment variables
func loadConfigFromEnv() (*internal.Config, error) {
	config := &internal.Config{}

	// Required environment variables
	config.AlpacaAPIKey = getEnvOrFail("ALPACA_API_KEY")
	config.AlpacaSecretKey = getEnvOrFail("ALPACA_SECRET_KEY")

	// DynamoDB configuration
	config.DynamoDBRegion = getEnvOrDefault("DYNAMODB_REGION", "us-east-1")
	config.TableName = getEnvOrDefault("TABLE_NAME", "artemis-data")

	// Trading configuration
	config.MaxSignalsPerWindow = getEnvAsIntOrDefault("MAX_SIGNALS_PER_WINDOW", 39)
	config.WindowDurationDays = getEnvAsIntOrDefault("WINDOW_DURATION_DAYS", 90)
	config.DefaultAllocationAmount = getEnvAsFloatOrDefault("DEFAULT_ALLOCATION_AMOUNT", 1000.0)

	// Paper trading flag
	config.IsPaperTrading = getEnvAsBoolOrDefault("IS_PAPER_TRADING", true)

	// Discord notifications
	config.DiscordWebhookURL = getEnvOrDefault("DISCORD_WEBHOOK_URL", "")

	return config, nil
}

// getEnvOrFail gets an environment variable or fails if not found
func getEnvOrFail(key string) string {
	value := os.Getenv(key)
	if value == "" {
		log.Fatalf("Required environment variable %s is not set", key)
	}
	return value
}

// getEnvOrDefault gets an environment variable or returns a default value
func getEnvOrDefault(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

// getEnvAsIntOrDefault gets an environment variable as int or returns a default value
func getEnvAsIntOrDefault(key string, defaultValue int) int {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}

	intValue, err := strconv.Atoi(value)
	if err != nil {
		log.Printf("Warning: Invalid integer value for %s: %s, using default: %d", key, value, defaultValue)
		return defaultValue
	}
	return intValue
}

// getEnvAsFloatOrDefault gets an environment variable as float64 or returns a default value
func getEnvAsFloatOrDefault(key string, defaultValue float64) float64 {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}

	floatValue, err := strconv.ParseFloat(value, 64)
	if err != nil {
		log.Printf("Warning: Invalid float value for %s: %s, using default: %f", key, value, defaultValue)
		return defaultValue
	}
	return floatValue
}

// getEnvAsBoolOrDefault gets an environment variable as bool or returns a default value
func getEnvAsBoolOrDefault(key string, defaultValue bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}

	// Convert to lowercase for case-insensitive comparison
	lowerValue := strings.ToLower(value)
	return lowerValue == "true" || lowerValue == "1" || lowerValue == "yes"
}

func main() {
	lambda.Start(handler)
}
