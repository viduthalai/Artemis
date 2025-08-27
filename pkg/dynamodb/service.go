package dynamodb

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dynamodbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/vignesh-goutham/artemis/pkg/types"
)

// Service handles all DynamoDB operations with single table design
type Service struct {
	client    *dynamodb.Client
	tableName string
}

// NewService creates a new DynamoDB service instance
func NewService(region, tableName string) (*Service, error) {
	cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	client := dynamodb.NewFromConfig(cfg)

	return &Service{
		client:    client,
		tableName: tableName,
	}, nil
}

// LoadAllData loads all data from DynamoDB into memory
func (d *Service) LoadAllData(ctx context.Context) ([]types.Signal, *types.AllocationWindow, error) {
	// Scan the entire table
	input := &dynamodb.ScanInput{
		TableName: aws.String(d.tableName),
	}

	result, err := d.client.Scan(ctx, input)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to scan table: %w", err)
	}

	var signals []types.Signal
	var allocationWindow *types.AllocationWindow

	for _, item := range result.Items {
		var unifiedItem types.UnifiedItem
		err := attributevalue.UnmarshalMap(item, &unifiedItem)
		if err != nil {
			continue
		}

		switch unifiedItem.Type {
		case types.ItemTypeSignal:
			var signal types.Signal
			err := json.Unmarshal([]byte(unifiedItem.Data), &signal)
			if err == nil {
				signals = append(signals, signal)
			}
		case types.ItemTypeAllocation:
			var window types.AllocationWindow
			err := json.Unmarshal([]byte(unifiedItem.Data), &window)
			if err == nil {
				allocationWindow = &window
			}
		}
	}

	return signals, allocationWindow, nil
}

// SaveAllData saves all data back to DynamoDB in one batch operation
func (d *Service) SaveAllData(ctx context.Context, signals []types.Signal, allocationWindow *types.AllocationWindow) error {
	var writeRequests []dynamodbtypes.WriteRequest

	// Add signals
	for _, signal := range signals {
		data, err := json.Marshal(signal)
		if err != nil {
			continue
		}

		unifiedItem := types.UnifiedItem{
			PK:        "SIGNAL#" + string(signal.Status),
			SK:        signal.UUID.String(),
			Type:      types.ItemTypeSignal,
			Data:      string(data),
			CreatedAt: signal.CreatedAt,
			UpdatedAt: signal.UpdatedAt,
		}

		item, err := attributevalue.MarshalMap(unifiedItem)
		if err != nil {
			continue
		}

		writeRequests = append(writeRequests, dynamodbtypes.WriteRequest{
			PutRequest: &dynamodbtypes.PutRequest{
				Item: item,
			},
		})
	}

	// Add allocation window
	if allocationWindow != nil {
		data, err := json.Marshal(allocationWindow)
		if err == nil {
			unifiedItem := types.UnifiedItem{
				PK:        "ALLOCATION#CURRENT",
				SK:        "WINDOW",
				Type:      types.ItemTypeAllocation,
				Data:      string(data),
				CreatedAt: allocationWindow.UpdatedAt,
				UpdatedAt: allocationWindow.UpdatedAt,
			}

			item, err := attributevalue.MarshalMap(unifiedItem)
			if err == nil {
				writeRequests = append(writeRequests, dynamodbtypes.WriteRequest{
					PutRequest: &dynamodbtypes.PutRequest{
						Item: item,
					},
				})
			}
		}
	}

	// Batch write in chunks of 25 (DynamoDB limit)
	for i := 0; i < len(writeRequests); i += 25 {
		end := i + 25
		if end > len(writeRequests) {
			end = len(writeRequests)
		}

		batchInput := &dynamodb.BatchWriteItemInput{
			RequestItems: map[string][]dynamodbtypes.WriteRequest{
				d.tableName: writeRequests[i:end],
			},
		}

		_, err := d.client.BatchWriteItem(ctx, batchInput)
		if err != nil {
			return fmt.Errorf("failed to batch write items: %w", err)
		}
	}

	return nil
}

// SaveSignal saves a single signal to DynamoDB
func (d *Service) SaveSignal(ctx context.Context, signal types.Signal) error {
	// Marshal the signal to JSON
	data, err := json.Marshal(signal)
	if err != nil {
		return fmt.Errorf("failed to marshal signal: %w", err)
	}

	// Create the unified item
	unifiedItem := types.UnifiedItem{
		PK:        "SIGNAL#" + string(signal.Status),
		SK:        signal.UUID.String(),
		Type:      types.ItemTypeSignal,
		Data:      string(data),
		CreatedAt: signal.CreatedAt,
		UpdatedAt: signal.UpdatedAt,
	}

	// Marshal to DynamoDB format
	item, err := attributevalue.MarshalMap(unifiedItem)
	if err != nil {
		return fmt.Errorf("failed to marshal item: %w", err)
	}

	// Put the item directly to DynamoDB
	input := &dynamodb.PutItemInput{
		TableName: aws.String(d.tableName),
		Item:      item,
	}

	_, err = d.client.PutItem(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to put item: %w", err)
	}

	return nil
}
