# Godot-Go-Gameserver
 
## Features
* WebSocket-based communication for real-time interactions.
* Player registration, location updates, and score tracking.
* Broadcasting of player actions and game state to all connected clients.
* Periodic polling and broadcasting of player locations.
* Handling of player damage and respawn.
* Efficient Server Implementation
* The server is implemented in Go, using the lesismal/nbio/nbhttp package for WebSocket communication and Protobuffers for data serialization.

![alt text](https://github.com/kelo221/Godot-Go-Gameserver/blob/main/showcase.png?raw=true)

## Key Components
* Player Management: Players are stored in a concurrent map, with each player having an ID, name, health, position, and other attributes.
* Message Handling: The server listens for various message types (e.g., player registration, location updates) and handles each accordingly.
* Broadcasting: The server broadcasts player actions and game state updates to all connected clients.
* Scoreboard: A scoreboard keeps track of player scores, which are updated based on game events (e.g., dealing damage).

# Networking

Message Types
The server handles the following message types as defined by enums on both ends of communication:

- REQUEST_PLAYERS
- REGISTER
- UPDATE_LOCATION
- POLL_LOCATIONS
- DAMAGE_PLAYER
- INIT_CAST
- RESPAWN_PLAYER
- REQUEST_SCOREBOARD
- PLAYER_DISCONNECT

![alt text](https://github.com/kelo221/Godot-Go-Gameserver/blob/main/network_graph.png?raw=true)
Simplified network graph.
