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
	upgrader   = newUpgrader()
	players    sync.Map
	scoreboard sync.Map
	conns      sync.Map
	mu         sync.Mutex
)

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
		if playerID, ok := session.(uint32); ok {
			broadcastPlayerData(PLAYER_DISCONNECT, disconnectedPlayerData(playerID), playerID)
			players.Delete(playerID)
			scoreboard.Delete(playerID)
			conns.Delete(playerID)
			broadcastMessage(REQUEST_SCOREBOARD, returnScoreboard())
		}
	}
	fmt.Println("OnClose:", c.RemoteAddr().String(), err)
}

func disconnectedPlayerData(id uint32) []byte {
	mu.Lock()
	defer mu.Unlock()

	if player, ok := players.Load(id); ok {
		byteSlice, protoErr := proto2.Marshal(player.(*proto.Player))
		if protoErr != nil {
			fmt.Printf("Error marshaling disconnected player with ID %d: %v\n", id, protoErr)
			return nil
		}
		return byteSlice
	}
	fmt.Printf("Player with ID %d not found\n", id)
	return nil
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
		fmt.Println("Received a text message, which is not expected.")
	case websocket.BinaryMessage:
		msgType := _data[0]
		data := _data[1:]

		switch msgType {
		case REQUEST_PLAYERS:
			err := c.WriteMessage(websocket.BinaryMessage, pollPlayers())
			if err != nil {
				fmt.Println("REQUEST_PLAYERS error")
				fmt.Println(err.Error())
			}
		case REGISTER:
			err := c.WriteMessage(websocket.BinaryMessage, registerPlayer(data, c))
			if err != nil {
				fmt.Println("REGISTER error")
				fmt.Println(err.Error())
			}
			broadcastPlayerData(REQUEST_PLAYERS, pollPlayers(), c.Session().(uint32))
			broadcastMessage(REQUEST_SCOREBOARD, returnScoreboard())
		case UPDATE_LOCATION:
			updatePlayerLocation(data)
		case POLL_LOCATIONS:
			err := c.WriteMessage(websocket.BinaryMessage, pollPlayerLocations())
			if err != nil {
				fmt.Println("POLL_LOCATIONS error")
				fmt.Println(err.Error())
			}
		case DAMAGE_PLAYER:
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
				fmt.Println("REQUEST_SCOREBOARD error")
				fmt.Println(err.Error())
			}
		default:
			fmt.Println("Unknown message type", msgType)
		}
	default:
		fmt.Printf("Received unexpected message type: %v\n", messageType)
	}
}

func broadcastPlayerData(messageType byte, message []byte, id uint32) {
	conns.Range(func(_, value interface{}) bool {
		conn := value.(*websocket.Conn)
		if conn.Session().(uint32) != id {
			err := conn.WriteMessage(websocket.BinaryMessage, append([]byte{messageType}, message...))
			if err != nil {
				fmt.Println("Failed to send message to client:", err)
			}
		}
		return true
	})
}

func returnScoreboard() []byte {
	scoreSlice := proto.Scoreboard{}
	scoreboard.Range(func(_, value interface{}) bool {
		score := value.(*proto.Score)
		scoreSlice.Score = append(scoreSlice.Score, score)
		return true
	})
	byteSlice, protoErr := proto2.Marshal(&scoreSlice)
	if protoErr != nil {
		fmt.Printf("Error marshaling Scoreboard: %v\n", protoErr)
		return nil
	}
	return append([]byte{REQUEST_SCOREBOARD}, byteSlice...)
}

func respawnPlayer(p *proto.Player) []byte {
	rnd := rand.New(rand.NewSource(time.Now().UnixNano()))
	newX := rnd.Float32()*18 - 9
	newZ := rnd.Float32()*18 - 9

	p.Health = proto2.Float32(100)
	p.Pos = []*proto.Player_Position{
		{X: proto2.Float32(newX), Y: proto2.Float32(1.0), Z: proto2.Float32(newZ)},
	}
	byteSlice, protoErr := proto2.Marshal(p)
	if protoErr != nil {
		fmt.Printf("Error marshaling respawned player with ID %d: %v\n", p.GetId(), protoErr)
		return nil
	}
	return append([]byte{RESPAWN_PLAYER}, byteSlice...)
}

func broadcastMessage(messageType byte, message []byte) {
	conns.Range(func(_, value interface{}) bool {
		conn := value.(*websocket.Conn)
		err := conn.WriteMessage(websocket.BinaryMessage, append([]byte{messageType}, message...))
		if err != nil {
			fmt.Println("Failed to send message to client:", err)
		}
		return true
	})
}

func damagePlayer(data []byte) []byte {
	p := proto.Damage{}
	err := proto2.Unmarshal(data, &p)
	if err != nil {
		fmt.Printf("Error unmarshaling damage data: %v\n", err)
		return nil
	}

	var targetPlayer *proto.Player
	queRespawn := false

	if value, ok := players.Load(p.GetTargetId()); ok {
		player := value.(*proto.Player)
		player.Health = proto2.Float32(player.GetHealth() - p.GetDamage())
		if player.GetHealth() <= 0 {
			queRespawn = true
			if scoreValue, ok := scoreboard.Load(p.GetCasterId()); ok {
				score := scoreValue.(*proto.Score)
				score.Score = proto2.Uint32(score.GetScore() + 1)
				scoreboard.Store(p.GetCasterId(), score)
			}
		}
		targetPlayer = player
		players.Store(p.GetTargetId(), player)
	}

	byteSlice, protoErr := proto2.Marshal(targetPlayer)
	if protoErr != nil {
		fmt.Printf("Error marshaling damaged player with ID %d: %v\n", targetPlayer.GetId(), protoErr)
		return nil
	}

	broadcastMessage(DAMAGE_PLAYER, byteSlice)
	if queRespawn {
		return respawnPlayer(targetPlayer)
	}
	return nil
}

// pollPlayers polls the players and marshals the data to be sent.
func pollPlayers() []byte {
	mu.Lock()
	defer mu.Unlock()

	playerSlice := make([]*proto.Player, 0)
	players.Range(func(_, value interface{}) bool {
		player := value.(*proto.Player)
		playerSlice = append(playerSlice, player)
		return true
	})

	if len(playerSlice) == 0 {
		return nil
	}

	byteSlice, protoErr := proto2.Marshal(&proto.Players{Player: playerSlice})
	if protoErr != nil {
		fmt.Printf("Error marshaling Players: %v\n", protoErr)
		return nil
	}
	return append([]byte{REQUEST_PLAYERS}, byteSlice...)
}

func pollPlayerLocations() []byte {
	return pollPlayers()
}

func updatePlayerLocation(data []byte) {
	mu.Lock()
	defer mu.Unlock()
	p := proto.Player{}
	err := proto2.Unmarshal(data, &p)
	if err != nil {
		fmt.Println("error in unmarshalling player data")
		fmt.Println(err.Error())
		return
	}

	if value, ok := players.Load(p.GetId()); ok {
		player := value.(*proto.Player)
		player.Pos = p.Pos
		player.RotationY = p.RotationY
		player.RotationX = p.RotationX
		players.Store(p.GetId(), player)
	}
}

// registerPlayer registers a new player and returns the marshaled player data.
func registerPlayer(data []byte, c *websocket.Conn) []byte {
	mu.Lock()
	defer mu.Unlock()

	tempPlayer := proto.Player{}
	err := proto2.Unmarshal(data, &tempPlayer)
	if err != nil {
		fmt.Printf("Error unmarshaling player data during registration: %v\n", err)
		return nil
	}

	rnd := rand.New(rand.NewSource(time.Now().UnixNano()))
	newX := rnd.Float32()*18 - 9
	newZ := rnd.Float32()*18 - 9
	playerID := rand.Uint32()
	c.SetSession(playerID)
	playerState := proto.PLAYER_STATE_STANDING

	p := &proto.Player{
		Casting:      proto2.Bool(false),
		CurrentSpell: proto2.Uint32(0),
		PlayerColor:  proto2.String(tempPlayer.GetPlayerColor()),
		PlayerState:  &playerState,
		Name:         proto2.String(tempPlayer.GetName()),
		Id:           proto2.Uint32(playerID),
		RotationY:    proto2.Float32(0),
		RotationX:    proto2.Float32(0),
		Health:       proto2.Float32(100),
		Pos:          []*proto.Player_Position{{X: proto2.Float32(newX), Y: proto2.Float32(1.0), Z: proto2.Float32(newZ)}},
	}

	players.Store(playerID, p)
	conns.Store(playerID, c)

	byteSlice, protoErr := proto2.Marshal(p)
	if protoErr != nil {
		fmt.Printf("Error marshaling player at registration with ID %d: %v\n", playerID, protoErr)
		return nil
	}

	newPlayerScore := &proto.Score{
		Name:  tempPlayer.Name,
		Id:    proto2.Uint32(playerID),
		Score: proto2.Uint32(0),
	}
	scoreboard.Store(playerID, newPlayerScore)

	return append([]byte{REGISTER}, byteSlice...)
}

func onRegister(c *websocket.Conn) {
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

	ticker := time.NewTicker(time.Second / 60)
	defer ticker.Stop()

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
		fmt.Println("Engine shutdown failed:", err)
	}
}
