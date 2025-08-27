package internal

// Config holds the application configuration
type Config struct {
	AlpacaAPIKey    string
	AlpacaSecretKey string
	IsPaperTrading  bool

	// DynamoDB configuration
	DynamoDBRegion string
	TableName      string

	// Trading configuration
	MaxSignalsPerWindow     int
	WindowDurationDays      int
	DefaultAllocationAmount float64

	// Discord notifications
	DiscordWebhookURL string
}
