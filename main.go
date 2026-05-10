package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/gin-gonic/gin"
	"token_blockchain/api"
	"token_blockchain/database"
)

func main() {
	gin.SetMode(gin.ReleaseMode)
	router := gin.Default()

	server := api.NewServer()
	server.RegisterRoutes(router)

	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = "8080"
	}

	go func() {
		log.Printf("Server starting on port %s", addr)
		if err := router.Run(":" + addr); err != nil {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	if err := database.GetMongoInstance().Close(); err != nil {
		log.Printf("Error closing MongoDB: %v", err)
	}
}