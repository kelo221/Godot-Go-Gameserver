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
	players  proto.Players
	upgrader = newUpgrader()
	mu       sync.Mutex
)

func newUpgrader() *websocket.Upgrader {
	u := websocket.NewUpgrader()
	u.OnOpen(onRegister)
	u.OnMessage(onMessage)
	u.OnClose(onClose)
	return u
}

func onClose(c *websocket.Conn, err error) {

	playerID := c.Session()
	if playerID != nil {
		mu.Lock()
		defer mu.Unlock()
		for i := 0; i < len(players.Player); i++ {
			if players.Player[i].GetId() == playerID {
				players.Player = append(players.Player[:i], players.Player[i+1:]...)
				break
			}
		}
	}

	fmt.Println(c.Session())
	fmt.Println("OnClose:", c.RemoteAddr().String(), err)
}

func onMessage(c *websocket.Conn, messageType websocket.MessageType, _data []byte) {

	msgType := _data[0]
	data := _data[1:]

	switch msgType {
	case 1:
		playerName := string(data)
		fmt.Println("Registering player", playerName)
		err := c.WriteMessage(2, registerPlayer(playerName, c))
		if err != nil {
			return
		}
	case 2:
		updatePlayerLocation(data)
	case 3:
		err := c.WriteMessage(2, pollPlayerLocations())
		if err != nil {
			return
		}
	default:
		fmt.Println("Unknown message type")
	}

}

func pollPlayerLocations() []byte {

	mu.Lock()
	byteSlice, protoerr := proto2.Marshal(&players)
	mu.Unlock()

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
	for i := 0; i < len(players.Player); i++ {
		if players.Player[i].GetId() == p.GetId() {
			players.Player[i].Pos = p.Pos
			players.Player[i].RotationY = p.RotationY
			players.Player[i].RotationX = p.RotationX
			break
		}
	}

	mu.Unlock()
}

func registerPlayer(playerName string, c *websocket.Conn) []byte {

	newX := rand.Intn(8) - 8
	newZ := rand.Intn(8) - 8

	playerID := rand.Uint32()
	c.SetSession(playerID)
	playerState := proto.PLAYER_STATE_STANDING

	p := &proto.Player{
		PlayerState:  &playerState,
		Name:         proto2.String(playerName),
		Id:           proto2.Uint32(playerID),
		RotationY:    proto2.Float32(0),
		RotationX:    proto2.Float32(0),
		Health:       proto2.Float32(100),
		CurrentSpell: proto2.Uint32(0),
		Casting:      proto2.Bool(false),
		Pos: []*proto.Player_Position{
			{X: proto2.Float32(float32(newX)), Y: proto2.Float32(1.0), Z: proto2.Float32(float32(newZ))},
		},
	}

	mu.Lock()
	players.Player = append(players.Player, p)
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
