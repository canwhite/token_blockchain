package service

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"sort"
	"strconv"
	"time"
)

const (
	MAX_REQUEST_AGE = 5 * 60
)

func GetRechargeSecretKey() string {
	key := os.Getenv("RECHARGE_SECRET_KEY")
	if key == "" {
		log.Printf("Warning: RECHARGE_SECRET_KEY environment variable not set, using default")
		key = "your-secret-key-change-in-production"
	}
	return key
}

func ComputeHMACSignature(params map[string]string, secretKey string) string {
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var parts []string
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%s", k, params[k]))
	}
	paramStr := joinStrings(parts, "&")

	h := hmac.New(sha256.New, []byte(secretKey))
	h.Write([]byte(paramStr))
	return hex.EncodeToString(h.Sum(nil))
}

func joinStrings(strs []string, sep string) string {
	if len(strs) == 0 {
		return ""
	}
	result := strs[0]
	for i := 1; i < len(strs); i++ {
		result += sep + strs[i]
	}
	return result
}

func ValidateHMACSignature(params map[string]string, receivedSignature string, secretKey string) bool {
	computedSignature := ComputeHMACSignature(params, secretKey)
	isValid := hmac.Equal([]byte(computedSignature), []byte(receivedSignature))

	if isValid {
		log.Printf("[ValidateHMACSignature] Signature verified")
	} else {
		log.Printf("[ValidateHMACSignature] Signature mismatch")
	}

	return isValid
}

func ValidateTimestamp(timestamp int64) error {
	now := time.Now().Unix()
	age := now - timestamp

	if age < 0 {
		return fmt.Errorf("timestamp from future")
	}

	if age > MAX_REQUEST_AGE {
		return fmt.Errorf("request expired, older than %d seconds", MAX_REQUEST_AGE)
	}

	return nil
}

func ParseTimestamp(ts string) (int64, error) {
	return strconv.ParseInt(ts, 10, 64)
}
