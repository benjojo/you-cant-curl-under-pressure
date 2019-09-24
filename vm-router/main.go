package main

import (
	"net/http"

	"golang.org/x/net/websocket"
)

var (
	globalLoadPlanner *loadPlanner
)

func main() {
	lp := startLoadPlanner([]string{
		"",
	})

	globalLoadPlanner = lp

	http.Handle("/serve", websocket.Handler(startGame))
	http.ListenAndServe(":10000", nil)
}

func startGame(ws *websocket.Conn) {
	// plan:
	// Generate itenery of challenges to run
	// Handshake with the javascript
	// start timer.
	// start recorder.
	// use load planner to grab game shells
	// loop until finish
	// dump link to recording
	// dump high score board
}
