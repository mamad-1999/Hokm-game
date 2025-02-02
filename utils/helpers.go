package utils

import (
	"hokm-backend/game"
	"math/rand"
	"time"
)

func GenerateRoomID() string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, 6)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

// Initialize the deck with 52 cards
func NewDeck() []game.Card {
	suits := []string{"hearts", "diamonds", "clubs", "spades"}
	ranks := []string{"2", "3", "4", "5", "6", "7", "8", "9", "10", "J", "Q", "K", "A"}
	values := map[string]int{
		"2": 2, "3": 3, "4": 4, "5": 5, "6": 6, "7": 7, "8": 8, "9": 9, "10": 10,
		"J": 11, "Q": 12, "K": 13, "A": 14,
	}

	var deck []game.Card
	for _, suit := range suits {
		for _, rank := range ranks {
			deck = append(deck, game.Card{
				Suit:  suit,
				Rank:  rank,
				Value: values[rank],
			})
		}
	}
	return deck
}

// Shuffle the deck
func ShuffleDeck(deck []game.Card) []game.Card {
	rand.Seed(time.Now().UnixNano())
	rand.Shuffle(len(deck), func(i, j int) {
		deck[i], deck[j] = deck[j], deck[i]
	})
	return deck
}
