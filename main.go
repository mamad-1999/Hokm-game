package main

import (
	"hokm-backend/config"
	"log"

	"github.com/gin-gonic/gin"
)

func main() {
	// Load configuration
	config.LoadConfig()

	// Set up Gin router
	router := gin.Default()

	// Start server
	log.Println("Starting server on :8080...")
	router.Run(":8080")
}
