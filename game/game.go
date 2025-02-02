package game

import (
	"fmt"
	"math/rand"
	"sort"
	"sync"

	"github.com/gorilla/websocket"
	"gorm.io/gorm"
)

type GameHistory struct {
	gorm.Model
	Players []string `gorm:"type:text[]"`
	Winner  string
	Score   int
}

type Game struct {
	Deck               []Card
	TrumpSuit          string
	Players            []*Player
	CurrentTrick       []Card
	TrickPlayOrder     []*Player
	Scores             map[string]int // Scores for the current Round (tricks won)
	RoundScores        map[string]int // Scores for the overall game (Rounds won)
	CurrentPlayerIndex int
	DealerIndex        int
	TrumpPlayer        *Player
	CurrentRound       int  // Current Round number (1 to 7)
	IsGameOver         bool // Flag to indicate if the game is over
}

type Room struct {
	ID                 string                      // Unique identifier for the room
	Players            []*Player                   // List of players in the room
	Game               *Game                       // The game being played in the room
	SavedPlayers       map[string]*SavedPlayerData // Add this
	CurrentPlayerIndex int                         // Store the current player index
}

type GameManager struct {
	Rooms map[string]*Room
	Mu    sync.RWMutex // Capitalize to export the field
}

type Card struct {
	Suit  string // e.g., "hearts", "diamonds", "clubs", "spades"
	Rank  string // e.g., "2", "3", ..., "10", "J", "Q", "K", "A"
	Value int    // Numeric value for ranking
}

type Player struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Team      string          `json:"team"`
	Hand      []Card          `json:"hand,omitempty"`
	Conn      *websocket.Conn `json:"-"`
	Connected bool            `json:"connected"` // Add this
	Index     int             `json:"index"`     // Add this to maintain position
}

// In game/game.go
type SavedPlayerData struct {
	PlayerID  string
	Hand      []Card
	Team      string
	Index     int
	IsLeaving bool
	RoomID    string // Add this field
}

// WSMessage represents a WebSocket message
type WSMessage struct {
	Action string      `json:"action"` // e.g., "play_card", "choose_trump"
	Data   interface{} `json:"data"`   // Additional data (e.g., card played, trump suit)
}

type WSResponse struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload"`
}

var Manager = GameManager{
	Rooms: make(map[string]*Room),
	Mu:    sync.RWMutex{},
}

// Initialize RoundScores when creating a new Game
func NewGame() *Game {
	return &Game{
		Deck:               []Card{},             // Initialize Deck
		TrumpSuit:          "",                   // Initialize TrumpSuit
		Players:            []*Player{},          // Initialize Players
		CurrentTrick:       []Card{},             // Initialize CurrentTrick
		TrickPlayOrder:     []*Player{},          // Initialize TrickPlayOrder
		Scores:             make(map[string]int), // Initialize Scores
		RoundScores:        make(map[string]int), // Initialize RoundScores
		CurrentPlayerIndex: 0,                    // Initialize CurrentPlayerIndex
		DealerIndex:        0,                    // Initialize DealerIndex
		TrumpPlayer:        nil,                  // Initialize TrumpPlayer
		CurrentRound:       1,                    // Initialize CurrentRound (start with Round 1)
		IsGameOver:         false,                // Initialize IsGameOver
	}
}

// Update all mutex references in GameManager methods:
func (gm *GameManager) GetRoom(roomID string) *Room {
	gm.Mu.RLock()
	defer gm.Mu.RUnlock()
	return gm.Rooms[roomID]
}

func (gm *GameManager) CreateRoom() *Room {
	gm.Mu.Lock()
	defer gm.Mu.Unlock()

	roomID := GenerateRoomID()
	room := &Room{
		ID:      roomID,
		Players: []*Player{},
		Game:    NewGame(),
	}
	gm.Rooms[roomID] = room
	return room
}

func GenerateRoomID() string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, 6)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

func (r *Room) SortPlayers() {
	sort.Slice(r.Players, func(i, j int) bool {
		return r.Players[i].Index < r.Players[j].Index
	})
}

func (g *Game) NextTurn() {
	g.CurrentPlayerIndex = (g.CurrentPlayerIndex + 1) % len(g.Players)
}

// Play a card in the current trick
func (g *Game) PlayCard(playerID string, card Card) error {
	// Check if there are players in the game
	if len(g.Players) == 0 {
		return fmt.Errorf("no players in the game")
	}

	// Check if it's the player's turn
	currentPlayer := g.Players[g.CurrentPlayerIndex]

	if currentPlayer.ID != playerID {
		return fmt.Errorf("it's not your turn")
	}

	// Validate the card
	if !g.ValidateCardPlay(playerID, card) {
		return fmt.Errorf("invalid card play")
	}

	player := g.Players[g.CurrentPlayerIndex]
	g.TrickPlayOrder = append(g.TrickPlayOrder, player)

	// Add the card to the current trick
	g.CurrentTrick = append(g.CurrentTrick, card)

	// Move to the next player
	g.NextTurn()

	return nil
}

// Determine the winner of the current trick
func (g *Game) DetermineTrickWinner(players []*Player) string {
	if len(g.CurrentTrick) == 0 || len(g.TrickPlayOrder) != len(g.CurrentTrick) {
		return ""
	}

	leadingSuit := g.CurrentTrick[0].Suit
	winningCard := g.CurrentTrick[0]
	winnerIndex := 0

	for i, card := range g.CurrentTrick {
		if card.Suit == g.TrumpSuit {
			if winningCard.Suit != g.TrumpSuit {
				winningCard = card
				winnerIndex = i
			} else if card.Value > winningCard.Value {
				winningCard = card
				winnerIndex = i
			}
		} else if card.Suit == leadingSuit && winningCard.Suit != g.TrumpSuit {
			if card.Value > winningCard.Value {
				winningCard = card
				winnerIndex = i
			}
		}
	}

	// Use TrickPlayOrder instead of players slice
	if winnerIndex < len(g.TrickPlayOrder) {
		return g.TrickPlayOrder[winnerIndex].ID
	}
	return ""
}

// Add this to reset play order when starting new trick
func (g *Game) ResetTrick() {
	g.CurrentTrick = []Card{}
	g.TrickPlayOrder = []*Player{}
}

// Update scores based on the number of tricks won
func (g *Game) UpdateScores(team string, tricksWon int) {
	if g.Scores == nil {
		g.Scores = make(map[string]int)
	}
	g.Scores[team] += tricksWon
}

// Check if a team has won the game
func (g *Game) CheckForWinner(targetScore int) string {
	for team, score := range g.Scores {
		if score >= targetScore {
			return team
		}
	}
	return ""
}

func (g *Game) ChooseTrumpSuit(dealerID string, suit string) error {
	// Check if the dealer is choosing the suit
	dealer := g.Players[g.DealerIndex]
	if dealer.ID != dealerID {
		return fmt.Errorf("only the dealer can choose the trump suit")
	}

	// Set the trump suit
	g.TrumpSuit = suit
	return nil
}

func (g *Game) ValidateCardPlay(playerID string, card Card) bool {
	// Find the player
	var player *Player
	for _, p := range g.Players {
		if p.ID == playerID {
			player = p
			break
		}
	}
	if player == nil {
		return false
	}

	// Check if the player has the card
	hasCard := false
	for _, c := range player.Hand {
		if c.Suit == card.Suit && c.Rank == card.Rank {
			hasCard = true
			break
		}
	}
	if !hasCard {
		return false
	}

	// Check if the player is following the leading suit (if applicable)
	if len(g.CurrentTrick) > 0 {
		leadingSuit := g.CurrentTrick[0].Suit
		if card.Suit != leadingSuit {
			// Check if the player has a card of the leading suit
			for _, c := range player.Hand {
				if c.Suit == leadingSuit {
					return false // Player must follow the leading suit
				}
			}
		}
	}

	return true
}
