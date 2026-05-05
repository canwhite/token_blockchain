package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"token_blockchain/api"
	"token_blockchain/database"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("Warning: .env file not found, using environment variables")
	}

	mongoURI := os.Getenv("MONGODB_URI")
	mongoDB := os.Getenv("MONGODB_DATABASE")

	if mongoURI != "" && mongoDB != "" {
		if err := database.InitMongoDB(mongoURI, mongoDB); err != nil {
			log.Printf("Warning: Failed to connect to MongoDB: %v", err)
			log.Println("Continuing without MongoDB (blockchain-only mode)")
		} else {
			log.Println("MongoDB connected successfully")
			defer database.CloseMongoDB()
		}
	} else {
		log.Println("MongoDB not configured, running in blockchain-only mode")
	}

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
}
