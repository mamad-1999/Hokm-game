package handlers

import (
	"hokm-backend/game"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var playerCounter int

const ReconnectTimeout = 30 * time.Second

// Add new message types
const (
	MessagePlayerDisconnected = "player_disconnected"
	MessagePlayerReconnected  = "player_reconnected"
	MessageGameState          = "game_state"
	MessagePlayerLeft         = "player_left"
	MessagePlayerReplaced     = "player_replaced"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all connections (for development)
	},
}

// HandleWebSocket handles WebSocket connections
func HandleWebSocket(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Println("🔌 WebSocket upgrade failed:", err)
		return
	}
	log.Println("🌟 New WebSocket connection from:", conn.RemoteAddr())
	defer conn.Close()

	// Register the player
	player := registerPlayer(conn)
	if player == nil {
		return
	}

	// Handle incoming messages
	for {
		var msg game.WSMessage
		if err := conn.ReadJSON(&msg); err != nil {
			log.Println("Read error:", err)
			unregisterPlayer(player)
			break
		}

		// Process the message
		processMessage(player, msg)
	}
}
