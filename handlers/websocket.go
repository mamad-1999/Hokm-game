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

// *****************************************************************
// ************************* Handle Message ************************
// *****************************************************************

// processMessage processes incoming WebSocket messages
func processMessage(player *game.Player, msg game.WSMessage) {
	if !player.Connected {
		log.Println("Message from disconnected player")
		return
	}

	// Find the room the player is in
	room := findPlayerRoom(player)
	if room == nil {
		log.Println("Player is not in any room")
		return
	}

	// Block all game actions if paused
	if room.Game.IsGameOver && msg.Action != "reconnect" {
		player.Conn.WriteJSON(game.WSResponse{
			Type: "game_paused",
			Payload: map[string]interface{}{
				"message": "Waiting for player replacement. Game paused.",
			},
		})
		return
	}

	// Handle the message based on the action
	switch msg.Action {
	case "play_card":
		// Handle playing a card
		cardData, ok := msg.Data.(map[string]interface{})
		if !ok {
			log.Println("Invalid card data")
			return
		}

		// Validate card details
		suit, ok := cardData["Suit"].(string)
		if !ok || !isValidSuit(suit) {
			log.Println("Invalid suit")
			return
		}

		rank, ok := cardData["Rank"].(string)
		if !ok || !isValidRank(rank) {
			log.Println("Invalid rank")
			return
		}

		value, ok := cardData["Value"].(float64)
		if !ok {
			log.Println("Invalid value type")
			return
		}
		intValue := int(value)

		if !isValidValue(rank, intValue) {
			log.Println("Invalid value for rank")
			return
		}

		card := game.Card{
			Suit:  suit,
			Rank:  rank,
			Value: intValue,
		}

		log.Println("Playing card:", card)

		// Add to current trick
		if err := room.Game.PlayCard(player.ID, card); err != nil {
			log.Println("Error playing card:", err)
			return
		}

		// Remove from hand
		for i, c := range player.Hand {
			if c.Suit == card.Suit && c.Rank == card.Rank {
				player.Hand = append(player.Hand[:i], player.Hand[i+1:]...)
				break
			}
		}
		log.Printf("Player %s's updated hand: %v\n", player.Name, player.Hand)

		// Only broadcast if trick is NOT complete
		if len(room.Game.CurrentTrick) < len(room.Players) {
			broadcastGameUpdate(room)
			broadcastTurnUpdate(room)
		}

		// Check if trick completed
		if len(room.Game.CurrentTrick) == len(room.Players) {
			winnerID := room.Game.DetermineTrickWinner(room.Players)
			log.Println("Trick winner:", winnerID)

			var winningTeam string
			for _, p := range room.Players {
				if p.ID == winnerID {
					winningTeam = p.Team
					break
				}
			}

			if winningTeam == "" {
				log.Println("Could not determine winning team")
				return
			}

			room.Game.UpdateScores(winningTeam, 1)
			log.Printf("Updated scores: %+v\n", room.Game.Scores)

			// Inside the "play_card" case, replace the Round winner determination block with:
			// Check if the Round is over (7 tricks won by a team)
			if room.Game.Scores["team1"] >= 2 || room.Game.Scores["team2"] >= 2 {
				// Determine teams
				trumpTeam := room.Game.TrumpPlayer.Team
				oppositeTeam := getOppositeTeam(trumpTeam)

				var roundWinner string
				var roundPoints int
				var losingScore int

				// Determine which team won the Round
				if room.Game.Scores["team1"] >= 2 {
					roundWinner = "team1"
					losingScore = room.Game.Scores["team2"]
				} else {
					roundWinner = "team2"
					losingScore = room.Game.Scores["team1"]
				}

				// Determine points based on Hokm rules
				switch {
				case losingScore == 0 && roundWinner == trumpTeam:
					// Kot: Trump team won 7-0
					roundPoints = 2
					log.Printf("KOT! Trump team (%s) won 7-0. Awarding 2 points", trumpTeam)
				case losingScore == 0 && roundWinner == oppositeTeam:
					// Trump Kot: Opposite team won 7-0 against Trump team
					roundPoints = 3
					log.Printf("TRUMP KOT! Opposite team (%s) won 7-0. Awarding 3 points", oppositeTeam)
				default:
					// Regular win (any score other than 7-0)
					roundPoints = 1
					log.Printf("Regular win. Awarding 1 point to %s", roundWinner)
				}

				// Update Round scores
				room.Game.RoundScores[roundWinner] += roundPoints

				// Broadcast Round winner with points and Trump team info
				broadcastRoundWinner(room, roundWinner, roundPoints, trumpTeam)

				// Check if the game is over (7 Rounds won by a team)
				if room.Game.RoundScores["team1"] >= 7 || room.Game.RoundScores["team2"] >= 7 {
					// Determine the game winner
					var gameWinner string
					if room.Game.RoundScores["team1"] >= 7 {
						gameWinner = "team1"
					} else {
						gameWinner = "team2"
					}

					// Broadcast game over
					broadcastGameOver(room, gameWinner)
					room.Game.IsGameOver = true
					return
				}

				// Restart the game for the next Round
				restartGameForNextRound(room, roundWinner)
				room.Game.ResetTrick()
			} else {
				// Update current player to trick winner
				for i, p := range room.Players {
					if p.ID == winnerID {
						room.Game.CurrentPlayerIndex = i
						break
					}
				}

				room.Game.ResetTrick()

				// Final broadcast with cleaned state
				broadcastGameUpdate(room)
				broadcastTurnUpdate(room)
			}
		}

	case "choose_trump":
		// Handle choosing a trump suit
		trumpSuit, ok := msg.Data.(string)
		if !ok {
			log.Println("Invalid trump suit data")
			return
		}

		// Validate that the player is the Trump Player
		if player.ID != room.Game.TrumpPlayer.ID {
			log.Println("Only the Trump Player can choose the trump suit")
			return
		}

		// Set the Trump Suit
		room.Game.TrumpSuit = trumpSuit
		log.Printf("Trump suit chosen: %s\n", trumpSuit)

		// Broadcast the chosen Trump Suit to all players
		for _, p := range room.Players {
			p.Conn.WriteJSON(game.WSResponse{
				Type: "trump_suit_selected",
				Payload: map[string]interface{}{
					"trump_suit": trumpSuit,
				},
			})
		}

		// Step 1: Clear all players' hands except the Trump Player's initial 5 cards
		for _, p := range room.Players {
			if p.ID != room.Game.TrumpPlayer.ID {
				p.Hand = []game.Card{}
			}
		}

		// Step 2: Deal 5 cards to each of the other 3 players
		log.Printf("Deck length before dealing 5 cards to other players: %d\n", len(room.Game.Deck))
		for _, p := range room.Players {
			if p.ID != room.Game.TrumpPlayer.ID {
				cards := dealCards(room.Game.Deck, 5)
				p.Hand = append(p.Hand, cards...)
				room.Game.Deck = room.Game.Deck[5:]

				// Broadcast the first batch of 5 cards to the player
				p.Conn.WriteJSON(game.WSResponse{
					Type: "deal_cards_batch_1",
					Payload: map[string]interface{}{
						"cards": cards,
					},
				})
			}
		}
		log.Printf("Deck length after dealing 5 cards to other players: %d\n", len(room.Game.Deck))

		// Add a 1-second delay before the next batch
		time.Sleep(1 * time.Second)

		// Step 3: Deal 4 cards to all 4 players (including the Trump Player)
		log.Printf("Deck length before dealing 4 cards to all players: %d\n", len(room.Game.Deck))
		for _, p := range room.Players {
			cards := dealCards(room.Game.Deck, 4)
			p.Hand = append(p.Hand, cards...)
			room.Game.Deck = room.Game.Deck[4:]

			// Broadcast the second batch of 4 cards to the player
			p.Conn.WriteJSON(game.WSResponse{
				Type: "deal_cards_batch_2",
				Payload: map[string]interface{}{
					"cards": cards,
				},
			})
		}
		log.Printf("Deck length after dealing 4 cards to all players: %d\n", len(room.Game.Deck))

		// Add a 1-second delay before the next batch
		time.Sleep(1 * time.Second)

		// Step 4: Deal another 4 cards to all 4 players (including the Trump Player)
		log.Printf("Deck length before dealing another 4 cards to all players: %d\n", len(room.Game.Deck))
		for _, p := range room.Players {
			cards := dealCards(room.Game.Deck, 4)
			p.Hand = append(p.Hand, cards...)
			room.Game.Deck = room.Game.Deck[4:]

			// Broadcast the third batch of 4 cards to the player
			p.Conn.WriteJSON(game.WSResponse{
				Type: "deal_cards_batch_3",
				Payload: map[string]interface{}{
					"cards": cards,
				},
			})
		}
		log.Printf("Deck length after dealing another 4 cards to all players: %d\n", len(room.Game.Deck))

		// Log the hands of all players
		for _, p := range room.Players {
			log.Printf("Player %s (%s) hand: %v\n", p.Name, p.Team, p.Hand)
		}

		// Broadcast the updated game state
		broadcastGameUpdate(room)

		// Start the game with the Trump Player
		room.Game.CurrentPlayerIndex = indexOfPlayer(room.Players, room.Game.TrumpPlayer)
		broadcastTurnUpdate(room)
		// Add to processMessage switch case
	case "leave_game":
		handlePlayerLeave(player, room)
	default:
		// Handle unknown actions
		log.Println("Unknown action:", msg.Action)
	}
}

// *********************************************************
// ****************** Restart Logic ************************
// *********************************************************

func restartGameForNextRound(room *game.Room, roundWinner string) {
	fmt.Println("Reset The Round...")
	// Increment the Round number
	room.Game.CurrentRound++

	// Reset scores for the new Round (only reset Scores, not RoundScores)
	room.Game.Scores = make(map[string]int)

	// Reset the deck and shuffle
	room.Game.Deck = utils.NewDeck()
	room.Game.Deck = utils.ShuffleDeck(room.Game.Deck)

	// Clear all players' hands
	for _, player := range room.Players {
		player.Hand = []game.Card{}
	}

	// Determine the new Trump Player if necessary
	trumpTeam := room.Game.TrumpPlayer.Team
	oppositeTeam := getOppositeTeam(trumpTeam)

	// Rotate Trump Player ONLY if the current Round was won by the opposite team
	if roundWinner == oppositeTeam {
		currentTrumpIndex := indexOfPlayer(room.Players, room.Game.TrumpPlayer)
		nextTrumpIndex := (currentTrumpIndex + 1) % len(room.Players)
		room.Game.TrumpPlayer = room.Players[nextTrumpIndex]

		log.Printf("Current Trump Player: %s, Team: %s", room.Game.TrumpPlayer.ID, room.Game.TrumpPlayer.Team)
		log.Printf("Opposite Team: %s", oppositeTeam)
		log.Printf("Current Trump Index: %d", currentTrumpIndex)
		log.Printf("Next Trump Index: %d", nextTrumpIndex)

		// Broadcast the new Trump Player
		for _, p := range room.Players {
			p.Conn.WriteJSON(game.WSResponse{
				Type: "trump_player_selected",
				Payload: map[string]interface{}{
					"trump_player_id": room.Game.TrumpPlayer.ID,
				},
			})
		}
	}

	// Deal cards for the next Round (skip Ace selection)
	var err error
	room.Players, room.Game.Deck, room.Game.TrumpPlayer, err = utils.DealCards(room.Game.Deck, room.Players, false, room.Game.TrumpPlayer)
	if err != nil {
		log.Println("Error dealing cards:", err)
		return
	}

	// Notify the Trump Player to choose the Trump Suit
	room.Game.TrumpPlayer.Conn.WriteJSON(game.WSResponse{
		Type: "choose_trump",
		Payload: map[string]interface{}{
			"cards": room.Game.TrumpPlayer.Hand[:5], // First 5 cards for choosing the Trump Suit
		},
	})

	// Broadcast the new game state
	// broadcastGameUpdate(room)

	// Start the game with the Trump Player
	room.Game.CurrentPlayerIndex = indexOfPlayer(room.Players, room.Game.TrumpPlayer)
	broadcastTurnUpdate(room)
}

// Helper function to get the opposite team
func getOppositeTeam(team string) string {
	if team == "team1" {
		return "team2"
	}
	return "team1"
}

// ********************************************************
// ********************** Utils ***************************
// ********************************************************

// Helper function to deal a specific number of cards from the deck
func dealCards(deck []game.Card, num int) []game.Card {
	if len(deck) < num {
		return nil
	}
	return deck[:num]
}

func indexOfPlayer(players []*game.Player, player *game.Player) int {
	for i, p := range players {
		if p.ID == player.ID {
			return i
		}
	}
	return -1
}

func isValidSuit(suit string) bool {
	validSuits := []string{"hearts", "diamonds", "clubs", "spades"}
	for _, s := range validSuits {
		if s == suit {
			return true
		}
	}
	return false
}

func isValidRank(rank string) bool {
	validRanks := []string{"2", "3", "4", "5", "6", "7", "8", "9", "10", "J", "Q", "K", "A"}
	for _, r := range validRanks {
		if r == rank {
			return true
		}
	}
	return false
}

func isValidValue(rank string, value int) bool {
	rankValues := map[string]int{
		"2": 2, "3": 3, "4": 4, "5": 5, "6": 6, "7": 7, "8": 8, "9": 9, "10": 10,
		"J": 11, "Q": 12, "K": 13, "A": 14,
	}
	expectedValue, ok := rankValues[rank]
	if !ok {
		return false
	}
	return value == expectedValue
}

// ***********************************************************
// ***************** BroadCast Messages **********************
// ***********************************************************

// broadcastGameOver notifies all players that the game is over
func broadcastGameOver(room *game.Room, winner string) {
	for _, player := range room.Players {
		player.Conn.WriteJSON(game.WSResponse{
			Type: "game_over",
			Payload: map[string]interface{}{
				"winner": winner,
				"scores": room.Game.Scores,
			},
		})
	}
}

// broadcastGameUpdate sends the updated game state to all players in the room
func broadcastGameUpdate(room *game.Room) {
	game.Manager.Mu.RLock()
	defer game.Manager.Mu.RUnlock()
	for _, recipient := range room.Players {
		// Create filtered player list
		filteredPlayers := make([]*game.Player, len(room.Game.Players))

		for i, p := range room.Game.Players {
			playerCopy := *p
			if p.ID != recipient.ID {
				playerCopy.Hand = nil // Will be omitted in JSON
			}
			filteredPlayers[i] = &playerCopy
		}
		// Add just the trump player ID
		payload := map[string]interface{}{
			"game": map[string]interface{}{
				"players":            filteredPlayers,
				"trump_player_id":    room.Game.TrumpPlayer.ID,
				"trump_suit":         room.Game.TrumpSuit,
				"current_trick":      room.Game.CurrentTrick,
				"scores":             room.Game.Scores,
				"current_player_idx": room.Game.CurrentPlayerIndex,
			},
		}

		recipient.Conn.WriteJSON(game.WSResponse{
			Type:    "game_update",
			Payload: payload,
		})
	}
}

func broadcastGameStateAfterReplacement(room *game.Room, _ *game.Player) {
	for _, player := range room.Players {
		player.Conn.WriteJSON(game.WSResponse{
			Type: "game_state_update",
			Payload: map[string]interface{}{
				// "player":             newPlayer.Hand,
				"current_player_idx": room.Game.CurrentPlayerIndex,
				"trump_suit":         room.Game.TrumpSuit,
				"current_trick":      room.Game.CurrentTrick,
				"scores":             room.Game.Scores,
			},
		})
	}
}

func broadcastReplacementNotification(player *game.Player, room *game.Room) {
	for _, p := range room.Players {
		p.Conn.WriteJSON(game.WSResponse{
			Type: MessagePlayerReplaced,
			Payload: map[string]interface{}{
				"old_player_id": player.ID,
				"new_player_id": player.ID,
				"index":         player.Index,
			},
		})
	}
}

func broadcastConnectionStatus(player *game.Player, isConnected bool) {
	for _, room := range game.Manager.Rooms {
		for _, p := range room.Players {
			if p.ID == player.ID {
				msgType := MessagePlayerDisconnected
				if isConnected {
					msgType = MessagePlayerReconnected
				}

				for _, recipient := range room.Players {
					if recipient.ID != player.ID {
						recipient.Conn.WriteJSON(game.WSResponse{
							Type: msgType,
							Payload: map[string]interface{}{
								"player_id": player.ID,
								"connected": isConnected,
							},
						})
					}
				}
				return
			}
		}
	}
}

func broadcastLeaveNotification(player *game.Player, room *game.Room) {
	for _, p := range room.Players {
		if p.Connected {
			p.Conn.WriteJSON(game.WSResponse{
				Type: MessagePlayerLeft,
				Payload: map[string]interface{}{
					"player_id":         player.ID,
					"needs_replacement": true,
					"message":           "Game is paused waiting for a replacement.",
				},
			})
		}
	}
}

func broadcastTurnUpdate(room *game.Room) {
	currentPlayer := room.Game.Players[room.Game.CurrentPlayerIndex]
	for _, player := range room.Players {
		player.Conn.WriteJSON(game.WSResponse{
			Type: "turn_update",
			Payload: map[string]interface{}{
				"current_player": currentPlayer.ID,
			},
		})
	}
}

func broadcastRoundWinner(room *game.Room, winner string, points int, trumpTeam string) {
	for _, player := range room.Players {
		player.Conn.WriteJSON(game.WSResponse{
			Type: "round_winner",
			Payload: map[string]interface{}{
				"winner":         winner,
				"points_awarded": points,
				"trump_team":     trumpTeam,
				"round_scores":   room.Game.RoundScores,
				"current_round":  room.Game.CurrentRound,
			},
		})
	}
}
