package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/go-redis/redis"
	"github.com/gorilla/websocket"
)

var guidsBase []string

var clients = make(map[string][]*websocket.Conn)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type redisClientData struct {
	Guid string
}

func handleConnections(w http.ResponseWriter, r *http.Request) {
	channel := r.URL.Query()["channel"][0]
	println(channel)

	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer ws.Close()

	clients[channel] = append(clients[channel], ws)
	fmt.Println(clients)

	for {
		mt, msg, _ := ws.ReadMessage()
		println(string(msg))
		println("mt", mt)
		for _, client := range clients[channel] {
			client.WriteMessage(mt, msg)
		}
	}
}

func handleNewNotification(w http.ResponseWriter, r *http.Request) {

}

func clientsBaseInit() {
	redisConn := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "",
		DB:       1,
	})
	var cursor uint64
	for {
		var keys []string
		var err error
		keys, cursor, err = redisConn.Scan(cursor, "", 10).Result()
		if err != nil {
			// handle error
		}
		for _, key := range keys {
			guidsBase = append(guidsBase, key)
		}
		if cursor == 0 {
			break
		}
	}
	fmt.Println(guidsBase)
}

func main() {
	clientsBaseInit()
	// fs := http.FileServer(http.Dir(""))
	// http.Handle("/", fs)
	// http.HandleFunc("/ws/", handleConnections)
	// http.HandleFunc("/broadcast/", handleNewNotification)
	// log.Println("http server started on :8000")
	// err := http.ListenAndServe("192.168.0.105:8000", nil)
	// if err != nil {
	// 	log.Fatal("ListenAndServe: ", err)
	// }
}
