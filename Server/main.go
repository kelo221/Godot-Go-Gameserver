package main

import (
	"Server/proto"
	"context"
	"fmt"
	"github.com/lesismal/nbio/nbhttp"
	"github.com/lesismal/nbio/nbhttp/websocket"
	proto2 "google.golang.org/protobuf/proto"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"time"
)

var (
	upgrader = newUpgrader()
	mu       sync.RWMutex
)
var scoreboard = make(map[uint32]*proto.Score)
var players = make(map[uint32]*proto.Player)
var conns = make(map[uint32]*websocket.Conn)

func newUpgrader() *websocket.Upgrader {
	u := websocket.NewUpgrader()
	u.OnOpen(onRegister)
	u.OnMessage(onMessage)
	u.OnClose(onClose)
	return u
}

func onClose(c *websocket.Conn, err error) {
	session := c.Session()
	if session != nil {
		playerID, ok := session.(uint32)
		if ok {
			broadcastMessage(PLAYER_DISCONNECT, disconnectedPlayerData(playerID))
			mu.Lock()
			delete(scoreboard, playerID)
			delete(players, playerID)
			delete(conns, playerID)
			mu.Unlock()
		}
	}

	fmt.Println(c.Session())
	fmt.Println("OnClose:", c.RemoteAddr().String(), err)
}

func disconnectedPlayerData(id uint32) []byte {

	print(players)

	if _, ok := players[id]; ok {
		byteSlice, protoerr := proto2.Marshal(players[id])
		if protoerr != nil {
			println(protoerr.Error())
			return nil
		}
		return byteSlice
	} else {
		println("Player not found")
		return nil
	}

}

const (
	REQUEST_PLAYERS = iota
	REGISTER
	UPDATE_LOCATION
	POLL_LOCATIONS
	DAMAGE_PLAYER
	INIT_CAST
	RESPAWN_PLAYER
	REQUEST_SCOREBOARD
	PLAYER_DISCONNECT
)

func onMessage(c *websocket.Conn, messageType websocket.MessageType, _data []byte) {
	switch messageType {
	case websocket.TextMessage:
		// Handle text message if necessary
		fmt.Println("Received a text message, which is not expected.")
		return
	case websocket.BinaryMessage:
		// Handle binary message
		msgType := _data[0]
		data := _data[1:]

		switch msgType {

		case REQUEST_PLAYERS:
			print("Requesting players")
			err := c.WriteMessage(websocket.BinaryMessage, pollPlayers())
			if err != nil {
				println(err.Error())
				return
			}

		case REGISTER:
			err := c.WriteMessage(websocket.BinaryMessage, registerPlayer(data, c))
			if err != nil {
				return
			}

			// Send the updated player list to all clients, excluding the new player
			broadcastPlayerData(REQUEST_PLAYERS, pollPlayers(), c.Session().(uint32))

			// Send the updated scoreboard to all clients
			broadcastMessage(REQUEST_SCOREBOARD, returnScoreboard())
		case UPDATE_LOCATION:
			updatePlayerLocation(data)

		case POLL_LOCATIONS:
			err := c.WriteMessage(websocket.BinaryMessage, pollPlayerLocations())
			if err != nil {
				return
			}

		case DAMAGE_PLAYER:
			// If player is killed and needs to be respawned, return the new player state
			isDead := damagePlayer(data)
			if isDead != nil {
				broadcastMessage(RESPAWN_PLAYER, isDead)
				broadcastMessage(REQUEST_SCOREBOARD, returnScoreboard())
			}

		case INIT_CAST:
			broadcastPlayerData(INIT_CAST, data, c.Session().(uint32))

		case REQUEST_SCOREBOARD:
			err := c.WriteMessage(websocket.BinaryMessage, returnScoreboard())
			if err != nil {
				return
			}

		default:
			fmt.Println("Unknown message type", msgType)
		}

	default:
		fmt.Printf("Received unexpected message type: %v\n", messageType)
	}
}

func broadcastPlayerData(messageType byte, message []byte, id uint32) {

	mu.RLock()
	defer mu.RUnlock()
	for _, conn := range conns {

		if conn.Session().(uint32) == id {
			continue
		}

		err := conn.WriteMessage(2, append([]byte{messageType}, message...))
		if err != nil {
			fmt.Println("Failed to send message to client:", err)
		}
	}
}

func returnScoreboard() []byte {
	mu.RLock()
	scoreSlice := proto.Scoreboard{}

	for _, score := range scoreboard {
		scoreSlice.Score = append(scoreSlice.Score, score)
	}

	byteSlice, protoerr := proto2.Marshal(&scoreSlice)
	mu.RUnlock()

	if protoerr != nil {
		println(protoerr.Error())
		return nil
	}

	return append([]byte{REQUEST_SCOREBOARD}, byteSlice...)

}

func respawnPlayer(p *proto.Player) []byte {

	rnd := rand.New(rand.NewSource(time.Now().UnixNano()))

	newX := rnd.Float32()*18 - 9
	newZ := rnd.Float32()*18 - 9

	mu.Lock()
	if player, ok := players[p.GetId()]; ok {
		player.Health = proto2.Float32(100)
		p.Pos = []*proto.Player_Position{
			{X: proto2.Float32(newX), Y: proto2.Float32(1.0), Z: proto2.Float32(newZ)},
		}

	}
	mu.Unlock()

	byteSlice, protoerr := proto2.Marshal(p)

	if protoerr != nil {
		println(protoerr.Error())
		return nil
	}

	return append([]byte{RESPAWN_PLAYER}, byteSlice...)
}

// Broadcast a message to all clients
func broadcastMessage(messageType byte, message []byte) {

	mu.RLock()
	defer mu.RUnlock()
	for _, conn := range conns {
		err := conn.WriteMessage(2, append([]byte{messageType}, message...))
		if err != nil {
			fmt.Println("Failed to send message to client:", err)
		}
	}
}

func damagePlayer(data []byte) []byte {
	p := proto.Damage{}
	err := proto2.Unmarshal(data, &p)
	if err != nil {
		println(err.Error())
	}
	queRespawn := false

	var targetPlayer *proto.Player

	mu.Lock()
	if player, ok := players[p.GetTargetId()]; ok {
		targetPlayer = player
		player.Health = proto2.Float32(player.GetHealth() - p.GetDamage())
		if player.GetHealth() <= 0 {
			queRespawn = true
			scoreboard[p.GetCasterId()].Score = proto2.Uint32(scoreboard[p.GetCasterId()].GetScore() + 1)
		}

	}
	mu.Unlock()

	byteSlice, protoerr := proto2.Marshal(targetPlayer)
	if protoerr != nil {
		println(protoerr.Error())
	}

	// Broadcast the updated player state to all clients
	broadcastMessage(DAMAGE_PLAYER, byteSlice)

	if queRespawn {
		return respawnPlayer(targetPlayer)
	}
	return nil
}

func pollPlayers() []byte {
	mu.RLock()
	playerSlice := make([]*proto.Player, 0, len(players))
	for _, player := range players {
		playerSlice = append(playerSlice, player)
	}
	byteSlice, protoerr := proto2.Marshal(&proto.Players{Player: playerSlice})
	mu.RUnlock()

	if protoerr != nil {
		println(protoerr.Error())
		return nil
	}
	return append([]byte{REQUEST_PLAYERS}, byteSlice...)
}

func pollPlayerLocations() []byte {
	mu.RLock()
	playerSlice := make([]*proto.Player, 0, len(players))
	for _, player := range players {
		playerSlice = append(playerSlice, player)
	}
	byteSlice, protoerr := proto2.Marshal(&proto.Players{Player: playerSlice})
	mu.RUnlock()

	if protoerr != nil {
		println(protoerr.Error())
		return nil
	}
	return append([]byte{POLL_LOCATIONS}, byteSlice...)
}

func updatePlayerLocation(data []byte) {
	p := proto.Player{}
	err := proto2.Unmarshal(data, &p)
	if err != nil {
		return
	}

	mu.Lock()
	// Update the player's position and rotation.
	if player, ok := players[p.GetId()]; ok {
		player.Pos = p.Pos
		player.RotationY = p.RotationY
		player.RotationX = p.RotationX
	}
	mu.Unlock()
}

func registerPlayer(data []byte, c *websocket.Conn) []byte {

	tempPlayer := proto.Player{}
	err := proto2.Unmarshal(data, &tempPlayer)
	if err != nil {
		println(err.Error())
		return nil
	}

	rnd := rand.New(rand.NewSource(time.Now().UnixNano()))

	newX := rnd.Float32()*18 - 9
	newZ := rnd.Float32()*18 - 9

	playerID := rand.Uint32()
	c.SetSession(playerID)
	playerState := proto.PLAYER_STATE_STANDING

	p := &proto.Player{
		PlayerColor:  proto2.String(tempPlayer.GetPlayerColor()),
		PlayerState:  &playerState,
		Name:         proto2.String(tempPlayer.GetName()),
		Id:           proto2.Uint32(playerID),
		RotationY:    proto2.Float32(0),
		RotationX:    proto2.Float32(0),
		Health:       proto2.Float32(100),
		CurrentSpell: proto2.Uint32(0),
		Casting:      proto2.Bool(false),
		Pos: []*proto.Player_Position{
			{X: proto2.Float32(newX), Y: proto2.Float32(1.0), Z: proto2.Float32(newZ)},
		},
	}

	mu.Lock()
	players[playerID] = p
	conns[playerID] = c

	byteSlice, protoerr := proto2.Marshal(p)

	newPlayerScore := &proto.Score{
		Name:  tempPlayer.Name,
		Id:    proto2.Uint32(playerID),
		Score: proto2.Uint32(0),
	}
	scoreboard[playerID] = newPlayerScore

	mu.Unlock()

	if protoerr != nil {
		println(protoerr.Error())
		return nil
	}

	return append([]byte{REGISTER}, byteSlice...)
}

func onRegister(c *websocket.Conn) {
	// echo
	fmt.Println("OnOpen:", c.RemoteAddr().String())
}

func onWebsocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		panic(err)
	}
	fmt.Println("Upgraded:", conn.RemoteAddr().String())
}

func main() {
	mux := &http.ServeMux{}
	mux.HandleFunc("/", onWebsocket)
	engine := nbhttp.NewEngine(nbhttp.Config{
		Network:                 "tcp",
		Addrs:                   []string{"localhost:8080"},
		MaxLoad:                 1000000,
		ReleaseWebsocketPayload: true,
		Handler:                 mux,
	})

	err := engine.Start()
	if err != nil {
		fmt.Printf("nbio.Start failed: %v\n", err)
		return
	}

	// Create a ticker that ticks 60 times per second
	ticker := time.NewTicker(time.Second / 60)
	defer ticker.Stop()

	// Start a goroutine to broadcast player locations at each tick
	go func() {
		for range ticker.C {
			broadcastMessage(UPDATE_LOCATION, pollPlayerLocations())
		}
	}()

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)
	<-interrupt

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
	defer cancel()
	err = engine.Shutdown(ctx)
	if err != nil {
		return
	}
}
