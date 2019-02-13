package main

import (
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/go-redis/redis"
	"github.com/gorilla/websocket"
)

var tokensBase = make(map[string]bool)

var clients = make(map[string]map[string][]*websocket.Conn)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type RedisClientData struct {
	Token string
}

func checkToken(guid string) bool {
	ok := tokensBase[guid]
	return ok
}

func deleteClinet(token string, channel string, ws *websocket.Conn) {
	for index, current_ws := range clients[token][channel] {
		if current_ws == ws {
			clients[token][channel] = append(clients[token][channel][:index], clients[token][channel][index+1:]...)
		}
	}
}

func handleConnections(w http.ResponseWriter, r *http.Request) {
	channel := r.URL.Query()["channel"][0]
	token := r.URL.Query()["token"][0]
	println(channel)
	println(token)

	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	if !checkToken(token) {
		ws.Close()
		return
	}

	defer ws.Close()

	if _, ok := clients[token]; !ok {
		clients[token] = make(map[string][]*websocket.Conn)
	}

	clients[token][channel] = append(clients[token][channel], ws)
	fmt.Println(clients)

	for {
		_, msg, err := ws.ReadMessage()
		if err != nil {
			log.Printf("error: %v", err)
			deleteClinet(token, channel, ws)
			break
		}
		for _, client := range clients[token][channel] {
			client.WriteMessage(websocket.TextMessage, msg)
		}
	}
}

func handleMultiCast(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		return
	}
	token := r.Form.Get("token")
	message := r.Form.Get("message")
	channels := r.Form.Get("channels")
	channelList := strings.Split(channels, ",")

	if !checkToken(token) {
		return
	}
	for _, channel := range channelList {
		for _, conn := range clients[token][strings.TrimSpace(channel)] {
			conn.WriteMessage(websocket.TextMessage, []byte(message))
		}
	}
}

func handleBroadCast(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		return
	}
	token := r.Form.Get("token")
	message := r.Form.Get("message")
	println(token)
	println(message)
	if !checkToken(token) {
		return
	}
	for val, connections := range clients[token] {
		fmt.Println(val)
		for _, conn := range connections {
			conn.WriteMessage(websocket.TextMessage, []byte(message))
		}
	}
}

func handleUpdate(w http.ResponseWriter, r *http.Request) {
	go clientsBaseUpdate()
}

func clientsBaseUpdate() {
	tmpDict := make(map[string]bool)
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
			tmpDict[key] = true
		}
		if cursor == 0 {
			break
		}
	}
	tokensBase = tmpDict
	fmt.Println(tokensBase)
}

func main() {
	clientsBaseUpdate()
	http.HandleFunc("/update/", handleUpdate)
	http.HandleFunc("/ws/", handleConnections)
	http.HandleFunc("/multicast/", handleMultiCast)
	http.HandleFunc("/broadcast/", handleBroadCast)
	log.Println("http server started on :8001")
	err := http.ListenAndServe(":8001", nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
