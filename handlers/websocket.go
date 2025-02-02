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

// **************************************************************
// *********************** Connection ***************************
// **************************************************************

func handleReconnectingPlayer(player *game.Player, conn *websocket.Conn) *game.Player {
	game.Manager.Mu.Lock()
	defer game.Manager.Mu.Unlock()

	// Update connection and status
	player.Conn = conn
	player.Connected = true

	// Find and update player in room
	for _, room := range game.Manager.Rooms {
		for i, p := range room.Players {
			if p.ID == player.ID {
				room.Players[i] = player
				// Update game players reference
				for j, gp := range room.Game.Players {
					if gp.ID == player.ID {
						room.Game.Players[j] = player
						break
					}
				}
				sendReconnectNotifications(player, room)
				return player
			}
		}
	}
	return nil
}

// Modify getAvailableRoom to create rooms without deadlock
func getAvailableRoom() *game.Room {

	for _, room := range game.Manager.Rooms {
		if len(room.SavedPlayers) > 0 && len(room.Players) < 4 {
			return room
		}
	}
	// Find first non-full, non-ended game room
	for _, room := range game.Manager.Rooms {
		if len(room.Players) < 4 && !room.Game.IsGameOver {
			return room
		}
	}
	// Create new room if none available
	roomID := game.GenerateRoomID()
	room := &game.Room{
		ID:      roomID,
		Players: []*game.Player{},
		Game:    game.NewGame(),
	}
	game.Manager.Rooms[roomID] = room
	return room
}

func determineTeam(playerCount int) string {
	// Preserve original team assignment logic
	if playerCount%2 == 0 {
		return "team2"
	}
	return "team1"
}

func sendJoinMessage(player *game.Player, room *game.Room) {
	response := game.WSResponse{
		Type: "join_room",
		Payload: map[string]interface{}{
			"room_id": room.ID,
			"players": room.Players,
			"your_id": player.ID,
		},
	}
	if err := player.Conn.WriteJSON(response); err != nil {
		log.Printf("🚨 Error sending join_room to %s: %v", player.ID, err)
	} else {
		log.Printf("✅ Sent join_room to %s in room %s", player.ID, room.ID)
	}
}

func sendReconnectNotifications(player *game.Player, room *game.Room) {
	// Send full game state to reconnected player
	sendGameState(player)

	// Notify others about reconnection
	for _, p := range room.Players {
		if p.ID != player.ID && p.Connected {
			p.Conn.WriteJSON(game.WSResponse{
				Type: MessagePlayerReconnected,
				Payload: map[string]interface{}{
					"player_id": player.ID,
					"position":  player.Index,
				},
			})
		}
	}
}

func removePlayerPermanently(player *game.Player) {
	for _, room := range game.Manager.Rooms {
		for i, p := range room.Players {
			if p.ID == player.ID {
				room.Players = append(room.Players[:i], room.Players[i+1:]...)
				broadcastGameUpdate(room)
				break
			}
		}
	}
}

func handlePlayerLeave(player *game.Player, room *game.Room) {
	game.Manager.Mu.Lock()
	defer game.Manager.Mu.Unlock()

	// Save player state
	if room.SavedPlayers == nil {
		room.SavedPlayers = make(map[string]*game.SavedPlayerData)
	}

	// In handlers/websocket.go - handlePlayerLeave()
	room.SavedPlayers[player.ID] = &game.SavedPlayerData{
		PlayerID:  player.ID,
		Hand:      player.Hand,
		Team:      player.Team,
		Index:     player.Index,
		IsLeaving: true,
		RoomID:    room.ID, // Track the room
	}

	// Remove from active players
	for i, p := range room.Players {
		if p.ID == player.ID {
			room.Players = append(room.Players[:i], room.Players[i+1:]...)
			break
		}
	}

	// Pause the game
	room.Game.IsGameOver = true

	// Notify other players
	broadcastLeaveNotification(player, room)
}

// **************************************************************
// ************************ Room Handler ************************
// **************************************************************

func sendGameState(player *game.Player) {
	room := findPlayerRoom(player)
	if room == nil {
		return
	}

	// Create personalized game state
	personalizedState := map[string]interface{}{
		"trump_suit":     room.Game.TrumpSuit,
		"scores":         room.Game.Scores,
		"round_scores":   room.Game.RoundScores,
		"current_trick":  room.Game.CurrentTrick,
		"your_hand":      player.Hand,
		"teams":          getTeamInfo(room),
		"current_player": room.Game.Players[room.Game.CurrentPlayerIndex].ID,
	}

	player.Conn.WriteJSON(game.WSResponse{
		Type:    MessageGameState,
		Payload: personalizedState,
	})
}

func getTeamInfo(room *game.Room) map[string][]string {
	teams := make(map[string][]string)
	for _, p := range room.Players {
		teams[p.Team] = append(teams[p.Team], p.ID)
	}
	return teams
}

// findPlayerRoom finds the room that the player is in
func findPlayerRoom(player *game.Player) *game.Room {
	game.Manager.Mu.RLock()
	defer game.Manager.Mu.RUnlock()

	for _, room := range game.Manager.Rooms {
		// Check active players
		for _, p := range room.Players {
			if p.ID == player.ID {
				return room
			}
		}
		// Check saved players
		if room.SavedPlayers != nil {
			if _, ok := room.SavedPlayers[player.ID]; ok {
				return room
			}
		}
	}
	return nil
}
