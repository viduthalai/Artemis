package notification

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

// DiscordNotificationService handles sending notifications to Discord
type DiscordNotificationService struct {
	webhookURL string
	enabled    bool
}

// DiscordWebhookPayload represents the payload sent to Discord webhook
type DiscordWebhookPayload struct {
	Content string `json:"content"`
}

// NewDiscordNotificationService creates a new Discord notification service
func NewDiscordNotificationService(webhookURL string) *DiscordNotificationService {
	return &DiscordNotificationService{
		webhookURL: webhookURL,
		enabled:    webhookURL != "",
	}
}

// sendNotification sends a notification to Discord
func (d *DiscordNotificationService) sendNotification(message string) error {
	if !d.enabled {
		log.Println("Discord notifications disabled (no webhook URL)")
		return nil
	}

	payload := DiscordWebhookPayload{
		Content: message,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal Discord payload: %w", err)
	}

	resp, err := http.Post(d.webhookURL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to send Discord notification: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("Discord webhook returned status %d", resp.StatusCode)
	}

	return nil
}

// NotifySignalBought sends a notification when a signal is bought
func (d *DiscordNotificationService) NotifySignalBought(ticker string, shares float64, price float64, buyDate, sellDate time.Time) error {
	message := fmt.Sprintf("🛒 **Signal Bought**\n"+
		"Successfully bought **%s**\n"+
		"Shares: %.4f\n"+
		"Price: $%.2f\n"+
		"Total Value: $%.2f\n"+
		"Buy Date: %s\n"+
		"Sell Date: %s",
		ticker, shares, price, shares*price, buyDate.Format("2006-01-02"), sellDate.Format("2006-01-02"))

	return d.sendNotification(message)
}

// NotifySignalSold sends a notification when a signal is sold
func (d *DiscordNotificationService) NotifySignalSold(ticker string, shares float64, sellPrice, buyPrice float64, profitLoss float64, profitLossPct float64, duration int) error {
	var message string
	if profitLoss >= 0 {
		message = fmt.Sprintf("🤑 Bagged Win 🤑\n"+
			"Sold **%s**\n"+
			"Shares: %.4f\n"+
			"Sell Price: $%.2f\n"+
			"Buy Price: $%.2f\n"+
			"Profit: $%.2f (%.2f%%)\n"+
			"Duration: %d days",
			ticker, shares, sellPrice, buyPrice, profitLoss, profitLossPct, duration)
	} else {
		message = fmt.Sprintf("💸 Took Loss 💸\n"+
			"Sold **%s**\n"+
			"Shares: %.4f\n"+
			"Sell Price: $%.2f\n"+
			"Buy Price: $%.2f\n"+
			"Loss: $%.2f (%.2f%%)\n"+
			"Duration: %d days",
			ticker, shares, sellPrice, buyPrice, profitLoss, profitLossPct, duration)
	}

	return d.sendNotification(message)
}

// NotifyAccountStatus sends a notification with account information
func (d *DiscordNotificationService) NotifyAccountStatus(accountValue float64, cashBalance float64, totalSignals int) error {
	message := fmt.Sprintf("📊 **Account Status**\n"+
		"Account Value: $%.2f\n"+
		"Cash Balance: $%.2f\n"+
		"Active Signals: %d\n@everyone",
		accountValue, cashBalance, totalSignals)

	return d.sendNotification(message)
}

// NotifyError sends a notification for errors
func (d *DiscordNotificationService) NotifyError(errorType string, message string, details string) error {
	errorMessage := fmt.Sprintf("⚠️ **Error Alert**\n"+
		"**%s**\n"+
		"%s\n"+
		"Details: %s",
		errorType, message, details)

	return d.sendNotification(errorMessage)
}

// NotifyBotStart sends a notification when the bot starts
func (d *DiscordNotificationService) NotifyBotStart() error {
	message := "🚀 **Artemis Bot Started**\nTrading bot is now running and monitoring signals\n@everyone"
	return d.sendNotification(message)
}

// NotifyBotComplete sends a notification when the bot completes its run
func (d *DiscordNotificationService) NotifyBotComplete(processedSignals int, errors int, accountValue float64, cashBalance float64, totalSignals int) error {
	message := fmt.Sprintf("✅ **Bot Run Complete**\n"+
		"Signals Processed: %d\n"+
		"Errors: %d\n"+
		"Account Value: $%.2f\n"+
		"Cash Balance: $%.2f\n"+
		"Active Signals: %d\n@everyone",
		processedSignals, errors, accountValue, cashBalance, totalSignals)

	return d.sendNotification(message)
}

// NotifyMarketClosed sends a notification when the market is closed
func (d *DiscordNotificationService) NotifyMarketClosed() error {
	message := "🏛️ Market Closed\nTrading bot detected that the market is currently closed\n@everyone"
	return d.sendNotification(message)
}
