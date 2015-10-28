package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"path/filepath"

	"github.com/daviddengcn/go-colortext"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"

	"net/http"
	"os"
	_ "strings"
)

var (
	port             int
	directory        string
	cors             bool
	stickToHappyPath bool
)

func check(e error) {
	if e != nil {
		ct.ChangeColor(ct.Red, true, ct.None, false)
		panic(e)
		ct.ResetColor()
	}
}

func say(msg string) {
	ct.ChangeColor(ct.Green, true, ct.None, true)
	fmt.Println(msg)
	ct.ResetColor()
}

func init() {
	flag.IntVar(&port, "p", 3000, "port to run on")
	flag.StringVar(&directory, "d", "./api", "directory to load blueprints from.")
	flag.BoolVar(&cors, "cors", true, "automatically respond to preflight requests.")
	flag.BoolVar(&stickToHappyPath, "happy", false, "Always stick to the happy path.")
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

var (
	askingQuestions = make(chan *Question)
	responseChan    = make(chan string)
)

func websocketHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Println("Got a websocket!")
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	for true {
		fmt.Println("Standing by for questions")
		question := <-askingQuestions

		json, err := json.Marshal(question)
		if err != nil {
			log.Println(err)
			return
		}
		fmt.Println("Got a question", string(json))

		ws.WriteMessage(websocket.TextMessage, json)

		_, message, err := ws.ReadMessage()
		if err != nil {
			fmt.Println(err)
			return
		}

		fmt.Println("GOT MESSAGE: ", string(message))
		responseChan <- string(message)
	}
}

func main() {
	flag.Parse()
	files, err := filepath.Glob(fmt.Sprintf("%s/*.md", directory))
	check(err)

	r := mux.NewRouter()

	for _, file := range files {
		say(fmt.Sprintf("√ Reading API %s", file))
		api, err := NewAPI(file)
		check(err)

		err = api.GenerateRoutes(r, cors, stickToHappyPath)
		check(err)
	}

	if !stickToHappyPath {
		r.HandleFunc("/ws", websocketHandler)
	}

	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", port), handlers.LoggingHandler(os.Stdout, r)))
}
