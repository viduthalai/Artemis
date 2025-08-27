package discord

import (
	"crypto/ed25519"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// VerifyRequest verifies that a request is actually from Discord
func VerifyRequest(body []byte, signature, timestamp string, publicKey string) error {
	// Check if timestamp is within 5 minutes (Discord's requirement)
	ts, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid timestamp: %w", err)
	}

	// Discord requires requests to be within 5 minutes
	if time.Now().Unix()-ts > 300 {
		return fmt.Errorf("request timestamp too old")
	}

	// Verify the signature
	if !verifySignature(body, signature, timestamp, publicKey) {
		return fmt.Errorf("invalid signature")
	}

	return nil
}

// verifySignature verifies the Discord signature using the correct format
func verifySignature(body []byte, signature, timestamp, publicKey string) bool {
	// Decode the public key
	pubKeyBytes, err := hex.DecodeString(publicKey)
	if err != nil || len(pubKeyBytes) != ed25519.PublicKeySize {
		return false
	}

	// Decode the signature
	sigBytes, err := hex.DecodeString(signature)
	if err != nil {
		return false
	}

	// Create the message to verify: timestamp + body
	message := append([]byte(timestamp), body...)

	// Verify the signature using Ed25519
	return ed25519.Verify(pubKeyBytes, message, sigBytes)
}

// ExtractSignatureHeaders extracts signature and timestamp from request headers
func ExtractSignatureHeaders(headers map[string]string) (signature, timestamp string, err error) {
	// Try different possible header names (case insensitive)
	signature = getHeaderCaseInsensitive(headers, "x-signature-ed25519")
	if signature == "" {
		return "", "", fmt.Errorf("missing x-signature-ed25519 header")
	}

	timestamp = getHeaderCaseInsensitive(headers, "x-signature-timestamp")
	if timestamp == "" {
		return "", "", fmt.Errorf("missing x-signature-timestamp header")
	}

	return signature, timestamp, nil
}

// getHeaderCaseInsensitive gets a header value case-insensitively
func getHeaderCaseInsensitive(headers map[string]string, key string) string {
	// Try exact match first
	if value, exists := headers[key]; exists {
		return value
	}

	// Try lowercase
	if value, exists := headers[strings.ToLower(key)]; exists {
		return value
	}

	// Try uppercase
	if value, exists := headers[strings.ToUpper(key)]; exists {
		return value
	}

	// Try title case (capitalize first letter)
	titleKey := strings.ToUpper(key[:1]) + strings.ToLower(key[1:])
	if value, exists := headers[titleKey]; exists {
		return value
	}

	return ""
}
