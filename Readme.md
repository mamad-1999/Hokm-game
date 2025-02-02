# Hokm Backend

## Overview

Hokm Backend is a Go-based server application designed to manage the backend logic for a multiplayer card game called Hokm. The application provides a RESTful API for user authentication and a WebSocket-based real-time game management system. It supports user registration, login, game room creation, card dealing, trick management, and real-time player interactions.

## Features

- **User Authentication**: Register and login with secure password hashing.
- **Game Management**: Create and manage game rooms with up to 4 players.
- **Real-Time Communication**: WebSocket-based communication for real-time game updates.
- **Card Dealing**: Automated card dealing and shuffling.
- **Trick Management**: Track and determine the winner of each trick.
- **Player Replacement**: Handle player disconnections and replacements seamlessly.
- **Game State Persistence**: Save and restore game state for disconnected players.

## Project Structure

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

## Getting Started

### Prerequisites

- Go 1.21.3 or higher
- PostgreSQL database
- Environment variables configured in `.env` file

### Installation

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

### API Endpoints

- **POST /register**: Register a new user.
- **POST /login**: Authenticate a user.
- **GET /ws**: Establish a WebSocket connection for real-time game updates.

### WebSocket Messages

- **join_room**: Join a game room.
- **play_card**: Play a card in the current trick.
- **choose_trump**: Choose the trump suit.
- **leave_game**: Leave the current game.

## Dependencies

- **Gin**: HTTP web framework.
- **GORM**: ORM for database management.
- **Viper**: Configuration management.
- **Gorilla WebSocket**: WebSocket implementation.
- **JWT**: JSON Web Tokens for authentication.

## Contributing

Contributions are welcome! Please fork the repository and create a pull request with your changes.

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Acknowledgments

- Special thanks to the Go community for providing excellent libraries and tools.
- Inspiration from traditional Hokm card game rules.

---

This README provides a comprehensive overview of the Hokm Backend project, including setup instructions, project structure, and key features. Adjust the content as needed to better fit your project's specifics.
