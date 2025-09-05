package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/google/uuid"
	"github.com/vignesh-goutham/artemis/discord-bot/internal"
	"github.com/vignesh-goutham/artemis/pkg/discord"
	"github.com/vignesh-goutham/artemis/pkg/dynamodb"
	"github.com/vignesh-goutham/artemis/pkg/types"
)

// DiscordInteraction represents a Discord interaction
type DiscordInteraction struct {
	Type      int                     `json:"type"`
	Token     string                  `json:"token"`
	Member    *DiscordMember          `json:"member,omitempty"`
	Data      *DiscordInteractionData `json:"data,omitempty"`
	GuildID   string                  `json:"guild_id,omitempty"`
	ChannelID string                  `json:"channel_id,omitempty"`
}

// DiscordMember represents a Discord guild member
type DiscordMember struct {
	User DiscordUser `json:"user"`
}

// DiscordUser represents a Discord user
type DiscordUser struct {
	ID       string `json:"id"`
	Username string `json:"username"`
}

// DiscordInteractionData represents the data in a Discord interaction
type DiscordInteractionData struct {
	ID          string                         `json:"id"`
	Name        string                         `json:"name"`
	Type        int                            `json:"type"`
	CustomID    string                         `json:"custom_id,omitempty"`
	Components  []DiscordComponent             `json:"components,omitempty"`
	Values      []string                       `json:"values,omitempty"`
	ModalSubmit *DiscordModalSubmitInteraction `json:"modal_submit,omitempty"`
}

// DiscordComponent represents a Discord component
type DiscordComponent struct {
	Type        int                `json:"type"`
	CustomID    string             `json:"custom_id,omitempty"`
	Label       string             `json:"label,omitempty"`
	Style       int                `json:"style,omitempty"`
	Components  []DiscordComponent `json:"components,omitempty"`
	Value       string             `json:"value,omitempty"`
	Required    bool               `json:"required,omitempty"`
	MinLength   int                `json:"min_length,omitempty"`
	MaxLength   int                `json:"max_length,omitempty"`
	Placeholder string             `json:"placeholder,omitempty"`
}

// DiscordModalSubmitInteraction represents modal submit data
type DiscordModalSubmitInteraction struct {
	CustomID   string             `json:"custom_id"`
	Components []DiscordComponent `json:"components"`
}

// DiscordResponse represents a Discord interaction response
type DiscordResponse struct {
	Type int                  `json:"type"`
	Data *DiscordResponseData `json:"data,omitempty"`
}

// DiscordResponseData represents the data in a Discord response
type DiscordResponseData struct {
	Content    string             `json:"content,omitempty"`
	Components []DiscordComponent `json:"components,omitempty"`
	Flags      int                `json:"flags,omitempty"`
}

// DiscordModal represents a Discord modal
type DiscordModal struct {
	CustomID   string             `json:"custom_id"`
	Title      string             `json:"title"`
	Components []DiscordComponent `json:"components"`
}

// DiscordModalResponse represents a modal response
type DiscordModalResponse struct {
	Type int          `json:"type"`
	Data DiscordModal `json:"data"`
}

const (
	// Discord interaction types
	InteractionTypePing                           = 1
	InteractionTypeApplicationCommand             = 2
	InteractionTypeMessageComponent               = 3
	InteractionTypeApplicationCommandAutocomplete = 4
	InteractionTypeModalSubmit                    = 5

	// Discord response types
	ResponseTypePong                                 = 1
	ResponseTypeChannelMessageWithSource             = 4
	ResponseTypeDeferredChannelMessageWithSource     = 5
	ResponseTypeDeferredUpdateMessage                = 6
	ResponseTypeUpdateMessage                        = 7
	ResponseTypeApplicationCommandAutocompleteResult = 8
	ResponseTypeModal                                = 9

	// Component types
	ComponentTypeActionRow         = 1
	ComponentTypeButton            = 2
	ComponentTypeStringSelect      = 3
	ComponentTypeTextInput         = 4
	ComponentTypeUserSelect        = 5
	ComponentTypeRoleSelect        = 6
	ComponentTypeMentionableSelect = 7
	ComponentTypeChannelSelect     = 8

	// Text input styles
	TextInputStyleShort     = 1
	TextInputStyleParagraph = 2

	// Response flags
	ResponseFlagEphemeral = 64
)

var (
	config    *internal.Config
	dbService *dynamodb.Service
)

func init() {
	var err error
	config, err = internal.LoadConfigFromEnv()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	dbService, err = dynamodb.NewService(config.DynamoDBRegion, config.TableName)
	if err != nil {
		log.Fatalf("Failed to create DynamoDB service: %v", err)
	}
}

func handler(ctx context.Context, req events.LambdaFunctionURLRequest) (events.LambdaFunctionURLResponse, error) {
	// Get exact bytes Discord signed
	var raw []byte
	if req.IsBase64Encoded {
		b, err := base64.StdEncoding.DecodeString(req.Body)
		if err != nil {
			log.Printf("Failed to decode base64 body: %v", err)
			return events.LambdaFunctionURLResponse{
				StatusCode: http.StatusBadRequest,
				Headers: map[string]string{
					"Content-Type": "application/json",
				},
				Body: `{"error": "invalid body"}`,
			}, nil
		}
		raw = b
	} else {
		raw = []byte(req.Body)
	}

	log.Printf("Received request body: %s", string(raw))

	// Debug: Log all headers
	log.Printf("Received headers: %+v", req.Headers)

	// Step 1: Verify ALL requests with Discord signature
	signature, timestamp, err := discord.ExtractSignatureHeaders(req.Headers)
	if err != nil {
		log.Printf("Failed to extract signature headers: %v", err)
		return events.LambdaFunctionURLResponse{
			StatusCode: http.StatusUnauthorized,
			Headers: map[string]string{
				"Content-Type": "application/json",
			},
			Body: `{"error": "Missing signature headers"}`,
		}, nil
	}

	log.Printf("Extracted signature: %s, timestamp: %s", signature, timestamp)

	// Verify the Discord signature for ALL requests
	if err := discord.VerifyRequest(raw, signature, timestamp, config.DiscordPublicKey); err != nil {
		log.Printf("Failed to verify Discord signature: %v", err)
		return events.LambdaFunctionURLResponse{
			StatusCode: http.StatusUnauthorized,
			Headers: map[string]string{
				"Content-Type": "application/json",
			},
			Body: `{"error": "Invalid signature"}`,
		}, nil
	}

	log.Printf("Verified Discord signature")

	// Parse the Discord interaction
	var interaction DiscordInteraction
	if err := json.Unmarshal(raw, &interaction); err != nil {
		log.Printf("Failed to parse interaction: %v", err)
		return events.LambdaFunctionURLResponse{
			StatusCode: http.StatusBadRequest,
			Headers: map[string]string{
				"Content-Type": "application/json",
			},
			Body: `{"error": "Invalid request body"}`,
		}, nil
	}

	// Handle different interaction types
	switch interaction.Type {
	case InteractionTypePing:
		// Respond to Discord's ping with pong
		log.Printf("Received PING request, responding with PONG")
		return events.LambdaFunctionURLResponse{
			StatusCode: http.StatusOK,
			Headers: map[string]string{
				"Content-Type": "application/json",
			},
			Body: `{"type":1}`,
		}, nil

	case InteractionTypeApplicationCommand:
		// Handle slash commands
		return handleApplicationCommand(ctx, &interaction)

	case InteractionTypeModalSubmit:
		// Handle modal submissions
		return handleModalSubmit(ctx, &interaction)

	default:
		log.Printf("Unhandled interaction type: %d", interaction.Type)
		return events.LambdaFunctionURLResponse{
			StatusCode: http.StatusBadRequest,
			Headers: map[string]string{
				"Content-Type": "application/json",
			},
			Body: `{"error": "Unhandled interaction type"}`,
		}, nil
	}
}

func handleApplicationCommand(ctx context.Context, interaction *DiscordInteraction) (events.LambdaFunctionURLResponse, error) {
	if interaction.Data == nil {
		return events.LambdaFunctionURLResponse{
			StatusCode: http.StatusBadRequest,
			Headers: map[string]string{
				"Content-Type": "application/json",
			},
			Body: `{"error": "No command data"}`,
		}, nil
	}

	switch interaction.Data.Name {
	case "addsignal":
		return handleAddSignalCommand(ctx, interaction)
	default:
		return events.LambdaFunctionURLResponse{
			StatusCode: http.StatusBadRequest,
			Headers: map[string]string{
				"Content-Type": "application/json",
			},
			Body: `{"error": "Unknown command"}`,
		}, nil
	}
}

func handleAddSignalCommand(ctx context.Context, interaction *DiscordInteraction) (events.LambdaFunctionURLResponse, error) {
	// Create a modal for signal input
	modal := DiscordModal{
		CustomID: "signal_modal",
		Title:    "Add Trading Signal",
		Components: []DiscordComponent{
			{
				Type: ComponentTypeActionRow,
				Components: []DiscordComponent{
					{
						Type:        ComponentTypeTextInput,
						CustomID:    "ticker",
						Label:       "Stock Ticker",
						Style:       TextInputStyleShort,
						Required:    true,
						MinLength:   1,
						MaxLength:   10,
						Placeholder: "e.g., AAPL",
					},
				},
			},
			{
				Type: ComponentTypeActionRow,
				Components: []DiscordComponent{
					{
						Type:        ComponentTypeTextInput,
						CustomID:    "sell_date",
						Label:       "Sell Date (YYYY-MM-DD)",
						Style:       TextInputStyleShort,
						Required:    true,
						MinLength:   10,
						MaxLength:   10,
						Placeholder: "2024-02-15",
					},
				},
			},
		},
	}

	response := DiscordModalResponse{
		Type: ResponseTypeModal,
		Data: modal,
	}

	return createResponse(response)
}

func handleModalSubmit(ctx context.Context, interaction *DiscordInteraction) (events.LambdaFunctionURLResponse, error) {
	if interaction.Data == nil {
		return events.LambdaFunctionURLResponse{
			StatusCode: http.StatusBadRequest,
			Headers: map[string]string{
				"Content-Type": "application/json",
			},
			Body: `{"error": "No modal submit data"}`,
		}, nil
	}

	// For modal submits, the custom_id is in data.custom_id, not data.modal_submit.custom_id
	customID := interaction.Data.CustomID
	if customID == "" {
		return events.LambdaFunctionURLResponse{
			StatusCode: http.StatusBadRequest,
			Headers: map[string]string{
				"Content-Type": "application/json",
			},
			Body: `{"error": "No custom_id found"}`,
		}, nil
	}

	switch customID {
	case "signal_modal":
		return handleSignalModalSubmit(ctx, interaction)
	default:
		return events.LambdaFunctionURLResponse{
			StatusCode: http.StatusBadRequest,
			Headers: map[string]string{
				"Content-Type": "application/json",
			},
			Body: `{"error": "Unknown modal"}`,
		}, nil
	}
}

func handleSignalModalSubmit(ctx context.Context, interaction *DiscordInteraction) (events.LambdaFunctionURLResponse, error) {
	// Extract form data from data.components
	var ticker, sellDateStr string

	if interaction.Data == nil || len(interaction.Data.Components) == 0 {
		return events.LambdaFunctionURLResponse{
			StatusCode: http.StatusBadRequest,
			Headers: map[string]string{
				"Content-Type": "application/json",
			},
			Body: `{"error": "No modal components found"}`,
		}, nil
	}

	// Log the components for debugging
	log.Printf("Modal components: %+v", interaction.Data.Components)

	for _, component := range interaction.Data.Components {
		if len(component.Components) > 0 {
			for _, subComponent := range component.Components {
				log.Printf("Processing component: custom_id=%s, value=%s", subComponent.CustomID, subComponent.Value)
				switch subComponent.CustomID {
				case "ticker":
					ticker = subComponent.Value
				case "sell_date":
					sellDateStr = subComponent.Value
				}
			}
		}
	}

	log.Printf("Extracted values: ticker=%s, sell_date=%s", ticker, sellDateStr)

	// Validate input
	if ticker == "" || sellDateStr == "" {
		response := DiscordResponse{
			Type: ResponseTypeChannelMessageWithSource,
			Data: &DiscordResponseData{
				Content: "❌ Error: All fields are required",
				Flags:   ResponseFlagEphemeral,
			},
		}
		return createResponse(response)
	}

	// Parse sell date
	sellDate, err := time.Parse("2006-01-02", sellDateStr)
	if err != nil {
		response := DiscordResponse{
			Type: ResponseTypeChannelMessageWithSource,
			Data: &DiscordResponseData{
				Content: "❌ Error: Invalid sell date format. Use YYYY-MM-DD",
				Flags:   ResponseFlagEphemeral,
			},
		}
		return createResponse(response)
	}

	// Set buy date to current date
	buyDate := time.Now().Truncate(24 * time.Hour) // Truncate to start of day

	// Validate date logic - sell date should be today or in the future
	if sellDate.Before(buyDate) {
		response := DiscordResponse{
			Type: ResponseTypeChannelMessageWithSource,
			Data: &DiscordResponseData{
				Content: "❌ Error: Sell date must be today or in the future",
				Flags:   ResponseFlagEphemeral,
			},
		}
		return createResponse(response)
	}

	// Create signal
	signal := types.Signal{
		UUID:      uuid.New(),
		Ticker:    ticker,
		BuyDate:   buyDate,
		SellDate:  sellDate,
		Status:    types.SignalStatusPending,
		NumStocks: 0,
		BuyPrice:  0,
		SellPrice: 0,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Save to DynamoDB using the shared service
	err = dbService.SaveSignal(ctx, signal)
	if err != nil {
		log.Printf("Failed to save signal: %v", err)
		response := DiscordResponse{
			Type: ResponseTypeChannelMessageWithSource,
			Data: &DiscordResponseData{
				Content: "❌ Error: Failed to save signal to database",
				Flags:   ResponseFlagEphemeral,
			},
		}
		return createResponse(response)
	}

	// Success response
	response := DiscordResponse{
		Type: ResponseTypeChannelMessageWithSource,
		Data: &DiscordResponseData{
			Content: fmt.Sprintf("✅ **Signal Added Successfully!**\n"+
				"**Ticker:** %s\n"+
				"**Buy Date:** %s (Today)\n"+
				"**Sell Date:** %s\n"+
				"**Status:** Pending\n"+
				"**UUID:** %s",
				ticker, buyDate.Format("2006-01-02"), sellDate.Format("2006-01-02"), signal.UUID.String()),
			Flags: ResponseFlagEphemeral,
		},
	}

	return createResponse(response)
}

func createResponse(response interface{}) (events.LambdaFunctionURLResponse, error) {
	responseBody, err := json.Marshal(response)
	if err != nil {
		return events.LambdaFunctionURLResponse{
			StatusCode: http.StatusInternalServerError,
			Headers: map[string]string{
				"Content-Type": "application/json",
			},
			Body: `{"error": "Failed to marshal response"}`,
		}, nil
	}

	return events.LambdaFunctionURLResponse{
		StatusCode: http.StatusOK,
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
		Body: string(responseBody),
	}, nil
}

func main() {
	lambda.Start(handler)
}
