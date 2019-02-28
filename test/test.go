package main

import (
	"fmt"
	"log"
	"net/http"
	"time"
)

var list = make(map[string]bool)

func sleep(t string) {
	fmt.Println(time.Now())
	time.Sleep(10000 * time.Millisecond)
	delete(list, t)
	fmt.Println(time.Now())
	fmt.Println(list)
}

func run(w http.ResponseWriter, r *http.Request) {
	t := r.URL.Query()["t"][0]
	fmt.Println(time.Now())
	list[t] = true
	fmt.Println(list)
	go sleep(t)
	go sleep(t)
}

func main() {
	http.HandleFunc("/main/", run)
	log.Println("http server started on :8001")
	err := http.ListenAndServe("0.0.0.0:8001", nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
