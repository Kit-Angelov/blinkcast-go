package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/go-redis/redis"
	"github.com/gorilla/websocket"
	uuid "github.com/satori/go.uuid"
)

var tokensBase = make(map[string]bool)

var clients = make(map[string]map[string][]*websocket.Conn)

var accessKeysCh = make(chan string)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type RedisClientData struct {
	Token string
}

type AccessKeyStruct struct {
	AccessKey string `json:"accessKey"`
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
	channelList := r.URL.Query()["channel"]
	accessKeyList := r.URL.Query()["key"]

	if len(channelList) == 0 || len(accessKeyList) == 0 {
		return
	}
	channel := channelList[0]
	accessKey := accessKeyList[0]

	token, res := checkAccessKey(accessKey)

	if !res {
		fmt.Println("closeCONNECT")
		return
	}

	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("error: %v", err)
		return
	}

	defer ws.Close()

	if _, ok := clients[token]; !ok {
		clients[token] = make(map[string][]*websocket.Conn)
	}

	clients[token][channel] = append(clients[token][channel], ws)

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

func setAccessKey(accessKey, token string) {
	redisConn := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "",
		DB:       2,
	})
	_ = redisConn.Set(accessKey, token, 360*time.Second).Err()
	redisConn.Close()
}

func checkAccessKey(accessKey string) (string, bool) {
	redisConn := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "",
		DB:       2,
	})
	defer redisConn.Close()
	val, err := redisConn.Get(accessKey).Result()
	if err == redis.Nil {
		return "", false
	} else {
		_ = redisConn.Del(accessKey).Err()
		return val, true
	}
}

func handleGettingAccessKey(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	if r.Method != http.MethodGet {
		return
	}
	tokenList := r.URL.Query()["token"]
	if len(tokenList) == 0 {
		return
	}
	token := tokenList[0]
	if !checkToken(token) {
		return
	}
	guid := uuid.Must(uuid.NewV4())
	sGuid := fmt.Sprintf("%s", guid)
	setAccessKey(sGuid, token)
	responseBody := AccessKeyStruct{sGuid}
	jsonResponse, _ := json.Marshal(responseBody)
	w.Header().Set("Content-Type", "application/json")
	w.Write(jsonResponse)
}

func handleMultiCast(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	if r.Method != http.MethodPost {
		return
	}
	err := r.ParseForm()
	if err != nil {
		log.Printf("error: %v", err)
		return
	}
	token := r.Form.Get("token")
	message := r.Form.Get("message")
	channels := r.Form.Get("channels")

	if len(token) == 0 || len(message) == 0 || len(channels) == 0 {
		return
	}

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
	w.Header().Set("Access-Control-Allow-Origin", "*")
	if r.Method != http.MethodPost {
		return
	}
	err := r.ParseForm()
	if err != nil {
		log.Printf("error: %v", err)
		return
	}
	token := r.Form.Get("token")
	message := r.Form.Get("message")

	if len(token) == 0 || len(message) == 0 {
		return
	}

	if !checkToken(token) {
		return
	}
	for _, connections := range clients[token] {
		for _, conn := range connections {
			conn.WriteMessage(websocket.TextMessage, []byte(message))
		}
	}
}

func handleUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		go clientsBaseUpdate()
	} else {
		return
	}
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
			log.Printf("error: %v", err)
			return
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
	http.HandleFunc("/get-access-key/", handleGettingAccessKey)
	log.Println("http server started on :8001")
	err := http.ListenAndServe("0.0.0.0:8001", nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
