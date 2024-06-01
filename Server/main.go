package main

import (
	"Server/proto"
	"fmt"
	"github.com/lxzan/gws"
	proto2 "google.golang.org/protobuf/proto"
	"io"
	"math/rand"
	"net/http"
	"sync"
	"time"
)

const (
	PingInterval = 60 * time.Second
	PingWait     = 3 * time.Second
)

var (
	players  proto.Players
	mu       sync.Mutex
	upgrader *gws.Upgrader
)

func main() {
	router := http.NewServeMux()
	router.HandleFunc("/websocket", websocket)
	router.HandleFunc("/register", register)

	upgrader = gws.NewUpgrader(&Handler{}, &gws.ServerOption{
		ParallelEnabled:   true,                                  // Parallel message processing
		Recovery:          gws.Recovery,                          // Exception recovery
		PermessageDeflate: gws.PermessageDeflate{Enabled: false}, // Enable compression
	})

	err := http.ListenAndServe(":8080", router)
	if err != nil {
		return
	}
}

func websocket(writer http.ResponseWriter, request *http.Request) {
	socket, err := upgrader.Upgrade(writer, request)
	if err != nil {
		return
	}
	go func() {
		socket.ReadLoop() // Blocking prevents the context from being GC.
	}()
}

func register(writer http.ResponseWriter, request *http.Request) {
	body, err := io.ReadAll(request.Body)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}
	defer request.Body.Close()

	playerID := rand.Int31()

	newX := rand.Intn(8) - 8
	newZ := rand.Intn(8) - 8

	p := &proto.Player{
		Id:     proto2.Int32(playerID),
		Name:   proto2.String(string(body)),
		Health: proto2.Float32(100),
		Pos: []*proto.Player_Position{
			{X: proto2.Float32(float32(newX)), Y: proto2.Float32(1.0), Z: proto2.Float32(float32(newZ))},
		},
	}

	mu.Lock()
	players.Player = append(players.Player, p)
	mu.Unlock()

	playerIDStr := fmt.Sprintf("%d", playerID)
	_, _ = writer.Write([]byte(playerIDStr))
}

type Handler struct{}

func (c *Handler) OnOpen(socket *gws.Conn) {
	_ = socket.SetDeadline(time.Now().Add(PingInterval + PingWait))
}

// OnClose removes the player from the list of players upon disconnection.
func (c *Handler) OnClose(socket *gws.Conn, err error) {
	playerID, found := socket.Session().Load("player_id")
	if found {
		mu.Lock()
		defer mu.Unlock()
		for i := 0; i < len(players.Player); i++ {
			if players.Player[i].GetId() == playerID.(int32) {
				players.Player = append(players.Player[:i], players.Player[i+1:]...)
				break
			}
		}
	}
}

func (c *Handler) OnPing(socket *gws.Conn, payload []byte) {
	_ = socket.SetDeadline(time.Now().Add(PingInterval + PingWait))
	_ = socket.WritePong(nil)
}

func (c *Handler) OnPong(socket *gws.Conn, payload []byte) {}

// OnMessage receives one player's information and returns information of all players.
func (c *Handler) OnMessage(socket *gws.Conn, message *gws.Message) {
	defer message.Close()
	var byteSlice []byte

	if len(message.Bytes()) != 0 {
		p := proto.Player{}
		err := proto2.Unmarshal(message.Bytes(), &p)
		if err != nil {
			return
		}

		_, exists := socket.Session().Load("player_id")
		if !exists {
			socket.Session().Store("player_id", p.GetId())
		}

		mu.Lock()
		// Update the player's position and rotation.
		for i := 0; i < len(players.Player); i++ {
			if players.Player[i].GetId() == p.GetId() {
				players.Player[i].Pos = p.Pos
				players.Player[i].Rot = p.Rot
				break
			}
		}
		var protoerr error
		byteSlice, protoerr = proto2.Marshal(&players)

		for i := 0; i < len(players.Player); i++ {
			fmt.Println(players.Player[i].GetId(), players.Player[i].GetName(), players.Player[i].GetHealth(), players.Player[i].GetPos(), players.Player[i].GetRot())
		}

		mu.Unlock()
		if protoerr != nil {
			println("error, ", protoerr.Error())
		}
	} else {
		fmt.Println("Warning no new data received, sending old data.")
		mu.Lock()
		var protoerr error
		byteSlice, protoerr = proto2.Marshal(&players)
		mu.Unlock()
		if protoerr != nil {
			println("error, ", protoerr.Error())
		}
	}

	err := socket.WriteMessage(0x2, byteSlice)
	if err != nil {
		return
	}
}
