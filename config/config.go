package config

import (
	"log"

	"github.com/joho/godotenv"
)

// LoadConfig loads environment variables from the .env file
func LoadConfig() {
	// Load .env file
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found")
	}
}
