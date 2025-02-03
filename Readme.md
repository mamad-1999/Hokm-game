# Hokm Backend

![istockphoto-1037693148-612x612](https://github.com/user-attachments/assets/037bf4a5-d5ac-4090-a7e7-9fd421d7e2cd)


## Overview ♠️

Hokm Backend is a Go-based server application designed to manage the backend logic for a multiplayer card game called Hokm. The application provides a RESTful API for user authentication and a WebSocket-based real-time game management system. It supports user registration, login, game room creation, card dealing, trick management, and real-time player interactions.

![2025-02-03_10-46_waifu2x_photo_noise2_scale](https://github.com/user-attachments/assets/447bf228-da1a-44b6-88e8-c0a90ccf5844)
![SIMPLE](https://github.com/user-attachments/assets/427c1a14-b3f0-4ef6-9638-a7d4fd9884fc)


## Features ♥️

- **User Authentication**: Register and login with secure password hashing.
- **Game Management**: Create and manage game rooms with up to 4 players.
- **Real-Time Communication**: WebSocket-based communication for real-time game updates.
- **Card Dealing**: Automated card dealing and shuffling.
- **Trick Management**: Track and determine the winner of each trick.
- **Player Replacement**: Handle player disconnections and replacements seamlessly.
- **Game State Persistence**: Save and restore game state for disconnected players.

## Project Structure ♣️

```
hokm-backend/
├── config/               # Configuration management
│   └── config.go         # Load environment variables
├── game/                 # Game logic and models
│   └── game.go           # Game state and management
├── handlers/             # HTTP and WebSocket handlers
│   ├── user.go           # User authentication handlers
│   └── websocket.go      # WebSocket game handlers
├── models/               # Database models
│   ├── database.go       # Database connection and initialization
│   └── user.go           # User model and password utilities
├── utils/                # Utility functions
│   ├── errors.go         # Custom error messages
│   └── helpers.go        # Helper functions for game logic
├── .env.example          # Example environment variables
├── go.mod                # Go module dependencies
├── go.sum                # Go dependency checksums
├── main.go               # Main application entry point
└── README.md             # Project documentation
```

## Getting Started ♦️

### Prerequisites

- Go 1.21.3 or higher
- PostgreSQL database
- Environment variables configured in `.env` file

### Installation ♠️

1. Clone the repository:

   ```sh
   git clone https://github.com/yourusername/hokm-backend.git
   cd hokm-backend
   ```

2. Install dependencies:

   ```sh
   go mod download
   ```

3. Set up your `.env` file:

   ```sh
   cp .env.example .env
   ```

4. Update the `.env` file with your database credentials:

   ```env
   DB_HOST=your_db_host
   DB_PORT=your_db_port
   DB_USER=your_db_user
   DB_PASSWORD=your_db_password
   DB_NAME=your_db_name
   ```

5. Run the application:
   ```sh
   go run main.go
   ```

### API Endpoints ♥️

- **POST /register**: Register a new user.
- **POST /login**: Authenticate a user.
- **GET /ws**: Establish a WebSocket connection for real-time game updates.

### WebSocket Messages ♣️

- **join_room**: Join a game room.
- **play_card**: Play a card in the current trick.
- **choose_trump**: Choose the trump suit.
- **leave_game**: Leave the current game.

### Example of messages ♥️
```json
{"action":"choose_trump","data":"clubs"}
{"action":"choose_trump","data":"diamonds"}
{"action":"choose_trump","data":"hearts"}
{"action":"choose_trump","data":"hearts"}

{"action": "leave_game"}

{"action":"play_card","data":{"Suit":"hearts","Rank":"2","Value":2}}
{"action":"play_card","data":{"Suit":"hearts","Rank":"3","Value":3}}
{"action":"play_card","data":{"Suit":"hearts","Rank":"4","Value":4}}
{"action":"play_card","data":{"Suit":"hearts","Rank":"5","Value":5}}
{"action":"play_card","data":{"Suit":"hearts","Rank":"6","Value":6}}
{"action":"play_card","data":{"Suit":"hearts","Rank":"7","Value":7}}
{"action":"play_card","data":{"Suit":"hearts","Rank":"8","Value":8}}
{"action":"play_card","data":{"Suit":"hearts","Rank":"9","Value":9}}
{"action":"play_card","data":{"Suit":"hearts","Rank":"10","Value":10}}
{"action":"play_card","data":{"Suit":"hearts","Rank":"J","Value":11}}
{"action":"play_card","data":{"Suit":"hearts","Rank":"Q","Value":12}}
{"action":"play_card","data":{"Suit":"hearts","Rank":"K","Value":13}}
{"action":"play_card","data":{"Suit":"hearts","Rank":"A","Value":14}}

{"action":"play_card","data":{"Suit":"diamonds","Rank":"2","Value":2}}
{"action":"play_card","data":{"Suit":"diamonds","Rank":"3","Value":3}}
{"action":"play_card","data":{"Suit":"diamonds","Rank":"4","Value":4}}
{"action":"play_card","data":{"Suit":"diamonds","Rank":"5","Value":5}}
{"action":"play_card","data":{"Suit":"diamonds","Rank":"6","Value":6}}
{"action":"play_card","data":{"Suit":"diamonds","Rank":"7","Value":7}}
{"action":"play_card","data":{"Suit":"diamonds","Rank":"8","Value":8}}
{"action":"play_card","data":{"Suit":"diamonds","Rank":"9","Value":9}}
{"action":"play_card","data":{"Suit":"diamonds","Rank":"10","Value":10}}
{"action":"play_card","data":{"Suit":"diamonds","Rank":"J","Value":11}}
{"action":"play_card","data":{"Suit":"diamonds","Rank":"Q","Value":12}}
{"action":"play_card","data":{"Suit":"diamonds","Rank":"K","Value":13}}
{"action":"play_card","data":{"Suit":"diamonds","Rank":"A","Value":14}}

{"action":"play_card","data":{"Suit":"clubs","Rank":"2","Value":2}}
{"action":"play_card","data":{"Suit":"clubs","Rank":"3","Value":3}}
{"action":"play_card","data":{"Suit":"clubs","Rank":"4","Value":4}}
{"action":"play_card","data":{"Suit":"clubs","Rank":"5","Value":5}}
{"action":"play_card","data":{"Suit":"clubs","Rank":"6","Value":6}}
{"action":"play_card","data":{"Suit":"clubs","Rank":"7","Value":7}}
{"action":"play_card","data":{"Suit":"clubs","Rank":"8","Value":8}}
{"action":"play_card","data":{"Suit":"clubs","Rank":"9","Value":9}}
{"action":"play_card","data":{"Suit":"clubs","Rank":"10","Value":10}}
{"action":"play_card","data":{"Suit":"clubs","Rank":"J","Value":11}}
{"action":"play_card","data":{"Suit":"clubs","Rank":"Q","Value":12}}
{"action":"play_card","data":{"Suit":"clubs","Rank":"K","Value":13}}
{"action":"play_card","data":{"Suit":"clubs","Rank":"A","Value":14}}

{"action":"play_card","data":{"Suit":"spades","Rank":"2","Value":2}}
{"action":"play_card","data":{"Suit":"spades","Rank":"3","Value":3}}
{"action":"play_card","data":{"Suit":"spades","Rank":"4","Value":4}}
{"action":"play_card","data":{"Suit":"spades","Rank":"5","Value":5}}
{"action":"play_card","data":{"Suit":"spades","Rank":"6","Value":6}}
{"action":"play_card","data":{"Suit":"spades","Rank":"7","Value":7}}
{"action":"play_card","data":{"Suit":"spades","Rank":"8","Value":8}}
{"action":"play_card","data":{"Suit":"spades","Rank":"9","Value":9}}
{"action":"play_card","data":{"Suit":"spades","Rank":"10","Value":10}}
{"action":"play_card","data":{"Suit":"spades","Rank":"J","Value":11}}
{"action":"play_card","data":{"Suit":"spades","Rank":"Q","Value":12}}
{"action":"play_card","data":{"Suit":"spades","Rank":"K","Value":13}}
{"action":"play_card","data":{"Suit":"spades","Rank":"A","Value":14}}
```

## Dependencies ♦️

- **Gin**: HTTP web framework.
- **GORM**: ORM for database management.
- **Viper**: Configuration management.
- **Gorilla WebSocket**: WebSocket implementation.
- **JWT**: JSON Web Tokens for authentication.

## Contributing ♠️

Contributions are welcome! Please fork the repository and create a pull request with your changes.

## License ♥️

This project is licensed under the GPL-3.0 License - see the [LICENSE](LICENSE) file for details.
