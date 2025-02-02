package handlers

import (
	"fmt"
	"hokm-backend/game"
	"hokm-backend/utils"
	"log"
	"net/http"
	"sort"
	"strconv"
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

func initializeGame(room *game.Room) {
	// Create and shuffle deck
	deck := utils.NewDeck()
	deck = utils.ShuffleDeck(deck)
	room.Game.Deck = deck

	// Deal cards
	var err error
	room.Players, room.Game.Deck, room.Game.TrumpPlayer, err = utils.DealCards(
		deck, room.Players, true, nil)

	if err != nil {
		log.Println("Error dealing cards:", err)
		return
	}

	room.Game.TrumpPlayer.Conn.WriteJSON(game.WSResponse{
		Type: "choose_trump",
		Payload: map[string]interface{}{
			"cards": room.Game.TrumpPlayer.Hand[:5], // First 5 cards for choosing the Trump Suit
		},
	})

	// Notify players about trump player
	// broadcastTrumpPlayer(room)
}

// ****************************************************************
// *********************** Replace Logic **************************
// ****************************************************************

// In handlers/websocket.go - findReplacementSpot()
func findReplacementSpot() (*game.Room, *game.SavedPlayerData) {
	game.Manager.Mu.RLock()
	defer game.Manager.Mu.RUnlock()

	// First pass: Find any saved player with their room ID
	for _, room := range game.Manager.Rooms {
		for _, data := range room.SavedPlayers {
			if data.IsLeaving {
				// Return the room where the saved player belongs
				return game.Manager.Rooms[data.RoomID], data
			}
		}
	}
	return nil, nil
}

func handleReplacement(room *game.Room, savedData *game.SavedPlayerData, conn *websocket.Conn) *game.Player {

	if room.ID != savedData.RoomID {
		log.Printf("Mismatched room ID during replacement")
		return nil
	}

	game.Manager.Mu.Lock()
	defer game.Manager.Mu.Unlock()

	// Create new player with saved data
	playerCounter++
	newPlayer := &game.Player{
		ID:        savedData.PlayerID, // Maintain same ID
		Name:      fmt.Sprintf("Player%d", playerCounter),
		Team:      savedData.Team,
		Hand:      savedData.Hand,
		Conn:      conn,
		Connected: true,
		Index:     savedData.Index,
	}

	// Add to room
	room.Players = append(room.Players, newPlayer)

	// Sort players to maintain order
	sort.Slice(room.Players, func(i, j int) bool {
		return room.Players[i].Index < room.Players[j].Index
	})

	// Update game references
	for i, p := range room.Game.Players {
		if p.ID == savedData.PlayerID {
			room.Game.Players[i] = newPlayer
			break
		}
	}

	// Remove from saved players
	delete(room.SavedPlayers, savedData.PlayerID)

	// Resume game if enough players
	if len(room.Players) == 4 {
		room.Game.IsGameOver = false

		// Notify all players about the new turn order
		broadcastTurnUpdate(room)
	}

	// Notify all players about the replacement
	broadcastReplacementNotification(newPlayer, room)

	// Broadcast the updated game state
	broadcastGameStateAfterReplacement(room, newPlayer)

	return newPlayer
}

// *****************************************************
// ******************** Register ***********************
// *****************************************************

func registerPlayer(conn *websocket.Conn) *game.Player {
	conn.WriteJSON(game.WSResponse{
		Type:    "connection_ack",
		Payload: map[string]interface{}{"status": "connecting"},
	})

	room, savedData := findReplacementSpot()
	if room != nil && savedData != nil {
		return handleReplacement(room, savedData, conn)
	}

	// First check for existing disconnected player
	existingPlayer := findExistingPlayer(conn)
	if existingPlayer != nil {
		return handleReconnectingPlayer(existingPlayer, conn)
	}

	// Create new player with proper locking
	game.Manager.Mu.Lock()
	defer game.Manager.Mu.Unlock()

	// Generate player ID and name
	playerCounter++
	playerID := strconv.Itoa(playerCounter)

	// Get or create room with available slot
	room = getAvailableRoom()

	// Determine team based on original player order
	team := determineTeam(len(room.Players))

	// Create new player with preserved index
	newPlayer := &game.Player{
		ID:        playerID,
		Name:      fmt.Sprintf("Player%d", playerCounter),
		Team:      team,
		Conn:      conn,
		Hand:      []game.Card{},
		Connected: true,
		Index:     len(room.Players), // Preserve position in original order
	}

	// Add to room and game
	room.Players = append(room.Players, newPlayer)
	room.Game.Players = append(room.Game.Players, newPlayer)

	// Send initial join message
	sendJoinMessage(newPlayer, room)

	// Start game if room is full
	if len(room.Players) == 4 {
		initializeGame(room)
	}

	return newPlayer
}

// Helper functions
func findExistingPlayer(conn *websocket.Conn) *game.Player {
	game.Manager.Mu.RLock()
	defer game.Manager.Mu.RUnlock()

	// Simple IP-based session (replace with proper session management)
	incomingIP := conn.RemoteAddr().String()

	for _, room := range game.Manager.Rooms {
		for _, p := range room.Players {
			if !p.Connected && p.Conn != nil && p.Conn.RemoteAddr().String() == incomingIP {
				return p
			}
		}
	}
	return nil
}

func unregisterPlayer(player *game.Player) {
	player.Connected = false
	broadcastConnectionStatus(player, false)

	// Only remove if disconnected for too long
	go func() {
		time.Sleep(ReconnectTimeout)
		if !player.Connected {
			removePlayerPermanently(player)
		}
	}()
}
