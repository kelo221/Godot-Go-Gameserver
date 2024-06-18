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
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	upgrader = newUpgrader()
	mu       sync.RWMutex
)
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
			mu.Lock()
			delete(players, playerID)
			delete(conns, playerID)
			mu.Unlock()
		}
	}

	fmt.Println(c.Session())
	fmt.Println("OnClose:", c.RemoteAddr().String(), err)
}

const (
	NULL = iota
	REGISTER
	UPDATE_LOCATION
	POLL_LOCATIONS
	DAMAGE_PLAYER
	INIT_CAST
	RESPAWN_PLAYER
)

func onMessage(c *websocket.Conn, messageType websocket.MessageType, _data []byte) {
	msgType := _data[0]
	data := _data[1:]

	switch msgType {
	case REGISTER:
		player_data := strings.Fields(string(data))
		playerName := player_data[0]
		playerColor := player_data[1]

		fmt.Println("Registering player", playerName)
		err := c.WriteMessage(2, registerPlayer(playerName, playerColor, c))
		if err != nil {
			return
		}
	case UPDATE_LOCATION:
		updatePlayerLocation(data)
	case POLL_LOCATIONS:
		err := c.WriteMessage(2, pollPlayerLocations())
		if err != nil {
			return
		}
	case DAMAGE_PLAYER:
		// If player is killed and needs to be respawned, return the new player state
		isDead := damagePlayer(data)
		if isDead != nil {
			broadcastMessage(6, isDead)
		}
	case INIT_CAST:
		playerID := (string(data))
		playerStartCast(playerID)
	case RESPAWN_PLAYER:

	default:
		fmt.Println("Unknown message type", msgType)
	}
}

func respawnPlayer(p *proto.Player) []byte {

	rnd := rand.New(rand.NewSource(time.Now().UnixNano()))

	newX := rnd.Float32()*18 - 9
	newZ := rnd.Float32()*18 - 9

	p.Pos = []*proto.Player_Position{
		{X: proto2.Float32(newX), Y: proto2.Float32(1.0), Z: proto2.Float32(newZ)},
	}
	p.Health = proto2.Float32(100)

	byteSlice, protoerr := proto2.Marshal(p)

	if protoerr != nil {
		println(protoerr.Error())
		return nil
	}

	return append([]byte{6}, byteSlice...)
}

func playerStartCast(id string) {

	u64, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		return
	}

	p := proto.Damage{
		CasterId: proto2.Uint32(uint32(u64)),
		TargetId: proto2.Uint32(0),
		Damage:   proto2.Float32(0),
	}

	byteSlice, protoerr := proto2.Marshal(&p)

	if protoerr != nil {
		println(protoerr.Error())
		return
	}

	broadcastMessage(INIT_CAST, byteSlice)
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
	fmt.Println("Incoming damage: ", p.GetDamage())

	var targetPlayer *proto.Player

	mu.Lock()
	if player, ok := players[p.GetTargetId()]; ok {
		targetPlayer = player
		player.Health = proto2.Float32(player.GetHealth() - p.GetDamage())
		if player.GetHealth() <= 0 {
			queRespawn = true
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
	return append([]byte{3}, byteSlice...)
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

func registerPlayer(playerName string, playerColor string, c *websocket.Conn) []byte {
	rnd := rand.New(rand.NewSource(time.Now().UnixNano()))

	newX := rnd.Float32()*18 - 9
	newZ := rnd.Float32()*18 - 9

	playerID := rand.Uint32()
	c.SetSession(playerID)
	playerState := proto.PLAYER_STATE_STANDING

	p := &proto.Player{
		PlayerColor:  proto2.String(playerColor),
		PlayerState:  &playerState,
		Name:         proto2.String(playerName),
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
	mu.Unlock()

	if protoerr != nil {
		println(protoerr.Error())
		return nil
	}

	return append([]byte{1}, byteSlice...)
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
