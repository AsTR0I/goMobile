package http

import (
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

func (h *HTTPServer) initMiddleware() {
	h.engine.Use(h.authMiddleware())
}

func (h *HTTPServer) authMiddleware() gin.HandlerFunc {
	// Берём токен из окружения
	expectedToken := os.Getenv("HTTP_TOKEN")

	return func(c *gin.Context) {
		// Получаем токен из заголовка
		providedToken := c.Query("token")

		if expectedToken == "" {
			// Если токен не настроен, можно разрешить или запретить все запросы
			logrus.Warn("HTTP_TOKEN not set in environment")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "server misconfiguration"})
			c.Abort()
			return
		}

		if providedToken != expectedToken {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			c.Abort()
			return
		}

		// Всё ок, продолжаем обработку
		c.Next()
	}
}
