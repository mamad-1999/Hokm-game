package game

import (
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
