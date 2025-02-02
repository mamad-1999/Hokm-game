package utils

import (
	"fmt"
	"hokm-backend/game"
	"log"
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

func DealCards(deck []game.Card, players []*game.Player, isInitialGame bool, trumpPlayer *game.Player) ([]*game.Player, []game.Card, *game.Player, error) {
	// Step 0: Shuffle the deck
	deck = ShuffleDeck(deck)
	log.Println("Deck shuffled.")
	log.Printf("Deck length after shuffling: %d\n", len(deck)) // Debug log

	// Step 1: Choose the Trump Player by dealing one card to each player until an Ace is drawn (only for initial game)
	if isInitialGame {
		log.Println("Choosing the Trump Player...")
		for i := 0; ; i++ {
			if len(deck) == 0 {
				return nil, nil, nil, fmt.Errorf("not enough cards in the deck")
			}

			player := players[i%len(players)]
			card := deck[0]
			deck = deck[1:]

			// Log the card being dealt to the player
			log.Printf("Dealt card %s of %s to %s\n", card.Rank, card.Suit, player.Name)

			// Broadcast the card being dealt to all players
			for _, p := range players {
				p.Conn.WriteJSON(game.WSResponse{
					Type: "dealing_card",
					Payload: map[string]interface{}{
						"player_id": player.ID,
						"card":      card,
					},
				})
			}

			// Add a delay of 1/4 second between each card deal
			time.Sleep(250 * time.Millisecond)

			// Add the card to the player's hand temporarily
			player.Hand = append(player.Hand, card)

			// Check if the card is an Ace
			if card.Rank == "A" {
				trumpPlayer = player
				log.Printf("Trump Player chosen: %s (drew an Ace)\n", trumpPlayer.Name)

				// Broadcast the Trump Player selection to all players
				for _, p := range players {
					p.Conn.WriteJSON(game.WSResponse{
						Type: "trump_player_selected",
						Payload: map[string]interface{}{
							"trump_player_id": trumpPlayer.ID,
							"card":            card,
						},
					})
				}

				// Clear the Trump Player's hand after selection
				trumpPlayer.Hand = []game.Card{}
				break
			}
		}
	} else {
		// If not the initial game, use the existing Trump Player passed as an argument
		log.Printf("Using existing Trump Player: %s\n", trumpPlayer.Name)
	}

	log.Printf("Deck length after choosing Trump Player: %d\n", len(deck)) // Debug log

	// Step 2: Reset the deck to 52 cards and shuffle again
	deck = NewDeck()
	deck = ShuffleDeck(deck)
	log.Println("Deck reset and shuffled again for dealing cards.")
	log.Printf("Deck length after reshuffling: %d\n", len(deck)) // Debug log

	// Step 3: Deal 5 cards to the Trump Player
	log.Println("Dealing 5 cards to the Trump Player...")
	for i := 0; i < 5; i++ {
		if len(deck) == 0 {
			log.Println("Not enough cards in the deck")
			return nil, nil, nil, fmt.Errorf("not enough cards in the deck")
		}
		trumpPlayer.Hand = append(trumpPlayer.Hand, deck[0])
		deck = deck[1:]

		// Add a delay of 1/4 second between each card deal
		time.Sleep(250 * time.Millisecond)
	}

	log.Printf("Trump Player's hand after 5 cards: %v\n", trumpPlayer.Hand)
	log.Printf("Deck length after dealing 5 cards to Trump Player: %d\n", len(deck)) // Debug log

	// Return the players, deck, and Trump Player
	return players, deck, trumpPlayer, nil
}
