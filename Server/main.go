package main

import (
	"Server/proto"
	"fmt"
	"github.com/lxzan/gws"
	proto2 "google.golang.org/protobuf/proto"
	"io"
	"math/rand/v2"
	"net/http"
	"time"
)

const (
	PingInterval = 60 * time.Second
	PingWait     = 60 * time.Second
)

var players proto.Players
var upgrader *gws.Upgrader

func main() {

	router := http.NewServeMux()
	router.HandleFunc("GET /websocket", websocket)
	router.HandleFunc("POST /register", register)

	upgrader = gws.NewUpgrader(&Handler{}, &gws.ServerOption{
		ParallelEnabled:   true,                                  // Parallel message processing
		Recovery:          gws.Recovery,                          // Exception recovery
		PermessageDeflate: gws.PermessageDeflate{Enabled: false}, // Enable compression
	})

	http.ListenAndServe(":8080", router)
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

	// Read the request body
	body, err := io.ReadAll(request.Body)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {

		}
	}(request.Body)

	print(string(body))

	var PlayerID = rand.Int32()
	p := &proto.Player{
		Id:     proto2.Int32(PlayerID),
		Name:   proto2.String(string(body)),
		Health: proto2.Float32(100),
		Pos: []*proto.Player_Position{
			{X: proto2.Float32(0), Y: proto2.Float32(0), Z: proto2.Float32(0)},
		},
	}

	players.Player = append(players.Player, p)

	playerIDStr := fmt.Sprintf("%d", PlayerID)
	_, _ = writer.Write([]byte(playerIDStr))
}

type Handler struct{}

func (c *Handler) OnOpen(socket *gws.Conn) {
	_ = socket.SetDeadline(time.Now().Add(PingInterval + PingWait))
}

func (c *Handler) OnClose(socket *gws.Conn, err error) {
	fmt.Println(err)
}

func (c *Handler) OnPing(socket *gws.Conn, payload []byte) {
	_ = socket.SetDeadline(time.Now().Add(PingInterval + PingWait))
	_ = socket.WritePong(nil)
}

func (c *Handler) OnPong(socket *gws.Conn, payload []byte) {}

// OnMessage recives one player's information and returns information of all players.
func (c *Handler) OnMessage(socket *gws.Conn, message *gws.Message) {
	defer message.Close()

	var byteSlice = []byte{}

	if len(message.Bytes()) != 0 {
		p := proto.Player{}

		err := proto2.Unmarshal(message.Bytes(), &p)
		if err != nil {
			return
		}

		players := proto.Player{}
		byteSlice, _ = proto2.Marshal(&players)
	}

	err := socket.WriteMessage(0x2, byteSlice)
	if err != nil {
		return
	}
}
