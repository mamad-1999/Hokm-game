package main

import (
	"hokm-backend/config"
	"hokm-backend/game"
	"hokm-backend/handlers"
	"hokm-backend/models"
	"log"

	"github.com/gin-gonic/gin"
)

func main() {
	// Load configuration
	config.LoadConfig()

	// Initialize database
	db, err := models.InitDB()
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	// Auto-migrate models
	db.AutoMigrate(&models.User{}, &game.GameHistory{})

	if err := models.TestConnection(); err != nil {
		log.Fatalf("💾 Database connection failed: %v", err)
	}

	// Set up Gin router
	router := gin.Default()

	// Routes
	router.POST("/register", handlers.Register)
	router.POST("/login", handlers.Login)
	router.GET("/ws", handlers.HandleWebSocket)

	// Start server
	log.Println("Starting server on :8080...")
	router.Run(":8080")
}
