package middleware

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/gin-gonic/gin"
	"token_blockchain/utils"
)

func RSARequestMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !isEncryptedRequest(c) {
			c.Next()
			return
		}

		decryptedBody, err := decryptRequestBody(c)
		if err != nil {
			c.JSON(400, gin.H{"error": "解密请求失败: " + err.Error()})
			c.Abort()
			return
		}

		c.Request.Body = io.NopCloser(strings.NewReader(decryptedBody))
		c.Next()
	}
}

func RSAResponseMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		writer := &responseBodyWriter{body: &bytes.Buffer{}, ResponseWriter: c.Writer}
		c.Writer = writer

		c.Next()

		if shouldEncryptResponse(c) {
			encrypted, err := encryptResponseData(writer.body.String())
			if err != nil {
				c.JSON(500, gin.H{"error": "加密响应失败: " + err.Error()})
				return
			}

			c.Header("Content-Type", "application/json")
			c.String(200, fmt.Sprintf(`{"encrypted":true,"data":"%s"}`, encrypted))
		} else {
			c.Data(200, c.ContentType(), writer.body.Bytes())
		}
	}
}

func isEncryptedRequest(c *gin.Context) bool {
	encrypted := c.GetHeader("X-Encrypted-Request")
	return encrypted == "true"
}

func shouldEncryptResponse(c *gin.Context) bool {
	return isEncryptedRequest(c)
}

func decryptRequestBody(c *gin.Context) (string, error) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return "", fmt.Errorf("读取请求体失败: %v", err)
	}

	var encryptedRequest struct {
		EncryptedData string `json:"encryptedData"`
		Signature     string `json:"signature,omitempty"`
	}

	if err := json.Unmarshal(body, &encryptedRequest); err != nil {
		return "", fmt.Errorf("解析加密请求失败: %v", err)
	}

	decrypted, err := utils.DecryptWithRSA(encryptedRequest.EncryptedData)
	if err != nil {
		return "", fmt.Errorf("RSA解密失败: %v", err)
	}

	return decrypted, nil
}

func encryptResponseData(data string) (string, error) {
	return utils.EncryptWithRSA(data)
}

type responseBodyWriter struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

func (w *responseBodyWriter) Write(b []byte) (int, error) {
	return w.body.Write(b)
}