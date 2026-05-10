package middleware

import (
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type DebounceMiddleware struct {
	requests sync.Map
}

func NewDebounceMiddleware() *DebounceMiddleware {
	return &DebounceMiddleware{}
}

func (dm *DebounceMiddleware) Debounce() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method != "PUT" && c.Request.Method != "POST" && c.Request.Method != "DELETE" {
			c.Next()
			return
		}

		clientIP := c.ClientIP()
		path := c.FullPath()
		userAgent := c.Request.UserAgent()

		requestKey := clientIP + ":" + path + ":" + userAgent

		if timestamp, exists := dm.requests.Load(requestKey); exists {
			lastTime := timestamp.(time.Time)
			if time.Since(lastTime) < 500*time.Millisecond {
				c.JSON(429, gin.H{
					"error": "请求过于频繁，请稍后重试",
					"code":  "RATE_LIMITED",
				})
				c.Abort()
				return
			}
		}

		dm.requests.Store(requestKey, time.Now())

		defer func() {
			go func(key string) {
				time.Sleep(1 * time.Second)
				dm.requests.Delete(key)
			}(requestKey)
		}()

		c.Next()
	}
}
