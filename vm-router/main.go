package main

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"golang.org/x/crypto/acme/autocert"
	"golang.org/x/net/websocket"
)

var (
	globalLoadPlanner *loadPlanner
	devMode           = flag.Bool("dev", false, "Don't do HTTPs and listen on port 10001")
)

func init() {
	// Register the summary and the histogram with Prometheus's default registry.
	prometheus.MustRegister(serverStates)
	prometheus.MustRegister(serverAvail)
	prometheus.MustRegister(serverRunning)
	prometheus.MustRegister(globalCount)
}

func main() {
	lp := startLoadPlanner([]string{
		// "::ffff:192.168.99.111",
		"::ffff:192.168.99.112",
		"::ffff:192.168.99.113",
		"::ffff:192.168.99.114",
		"::ffff:192.168.99.115",
		"::ffff:192.168.99.116",
		"::ffff:192.168.99.117",
		"::ffff:192.168.99.118",
	})

	globalLoadPlanner = lp

	if *devMode {
		http.Handle("/metrics", promhttp.Handler())
		http.Handle("/serve", websocket.Handler(startGame))
		http.HandleFunc("/", handleAlive)
		http.HandleFunc("/replay/", handleReplay)

		http.ListenAndServe(":10001", nil)
		return
	}

	var httpsSrv *http.Server
	httpsSrv = makeHTTPServer()

	// Note: use a sensible value for data directory
	// this is where cached certificates are stored
	dataDir := "."
	hostPolicy := func(ctx context.Context, host string) error {
		if host == "yccup.benjojo.co.uk" {
			return nil
		}
		return fmt.Errorf("acme/autocert: only yccup.benjojo.co.uk host is allowed")
	}

	m := &autocert.Manager{
		Prompt:     autocert.AcceptTOS,
		HostPolicy: hostPolicy,
		Cache:      autocert.DirCache(dataDir),
		Email:      "ben@benjojo.co.uk",
	}
	httpsSrv.Addr = fmt.Sprintf(":443")
	httpsSrv.TLSConfig = &tls.Config{GetCertificate: m.GetCertificate}

	go func() {
		mux := http.NewServeMux()
		mux.HandleFunc("/", redirect)
		s := &http.Server{
			Handler: m.HTTPHandler(mux),
			Addr:    fmt.Sprintf(":80"),
		}

		go s.ListenAndServe()
	}()

	err := httpsSrv.ListenAndServeTLS("", "")
	if err != nil {
		log.Fatalf("httpsSrv.ListendAndServeTLS() failed with %s", err)
	}
}

func redirect(w http.ResponseWriter, req *http.Request) {
	// remove/add not default ports from req.Host
	target := "https://" + req.Host + req.URL.Path
	if len(req.URL.RawQuery) > 0 {
		target += "?" + req.URL.RawQuery
	}
	log.Printf("redirect to: %s", target)
	http.Redirect(w, req, target,
		// see @andreiavrammsd comment: often 307 > 301
		http.StatusTemporaryRedirect)
}

func makeHTTPServer() *http.Server {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/check", handleAlive)
	mux.HandleFunc("/reload", handleProbe)
	mux.HandleFunc("/", handleRoot)
	mux.HandleFunc("/replay/", handleReplay)
	mux.Handle("/serve", websocket.Handler(startGame))

	return &http.Server{
		IdleTimeout: 120 * time.Second,
		Handler:     mux,
	}

}

type publicLoadResponse struct {
	Avail         int
	Running       int
	TurboCapacity float64
	UnusedBoost   float64
}

func handleAlive(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "https://blog.benjojo.co.uk")
	w.Header().Set("content-type", "application/json")
	Avail, Running := 0, 0
	MaxCapacity := 0

	for _, v := range globalLoadPlanner.Servers {

		if v.State != STATE_ALWAYS_ONLINE && v.State != STATE_ONLINE {
			continue
		}

		Running += v.Running
		Avail += v.Available

		if v.State == STATE_ONLINE {
			MaxCapacity += 90
		} else if v.State == STATE_ALWAYS_ONLINE {
			MaxCapacity += 30
		}
	}

	Turbo, Unused := (float64(Running) / float64(MaxCapacity) * 100), (float64(Avail) / float64(Running) * 100)

	pLR := publicLoadResponse{
		Avail:         Avail,
		Running:       Running,
		TurboCapacity: Turbo,
		UnusedBoost:   Unused,
	}

	b, _ := json.Marshal(&pLR)
	w.Write([]byte(b))
}

func startGame(ws *websocket.Conn) {
	// plan:
	defer ws.Close()
	// Handshake with the javascript

	err := jsHandshake(ws)
	if err != nil {
		fmt.Printf("failed to handshake with the client, %s", err.Error())
		return
	}
	sessionHash := makeHash()

	// Generate itenery of challenges to run
	log.Printf("[%s] Begins, order of challenges will be", sessionHash)
	itenery := getChallengeList()
	for _, v := range itenery {
		log.Printf("[%s] %s", sessionHash, v.ChallengeCode)
	}

	// start timer.
	startTimer := time.Now()

	// start recorder.
	recordingFile, _ := os.Create(fmt.Sprintf("./rec_%s.ttyrec", sessionHash))
	defer recordingFile.Close()

	// use load planner to grab game shells
	// loop until finish

	for k, v := range itenery {
		sendCurrentChallenge(ws, v)
		oneTimeWriteTottyFile([]byte(fmt.Sprintf("\r\n======\r\nCurrent Challenge: %s\r\n - %s \r\n\n", v.Title, v.Description)), recordingFile)
		log.Printf("[%s] [%d/%d] Getting server for challenge %s", sessionHash, k, len(itenery), v.ChallengeCode)
		system := globalLoadPlanner.getServer()
		system.Write([]byte(v.ChallengeCode + "\n"))
		doneCh := make(chan bool)
		go func() {
			io.Copy(system, ws)
			doneCh <- false
		}()

		log.Printf("[%s] [%d/%d] Uplinking for challenge %s", sessionHash, k, len(itenery), v.ChallengeCode)
		go func() {
			passed := inlineTTYRecRecorder(system, ws, recordingFile)
			doneCh <- passed
		}()
		passed := <-doneCh
		system.Close()
		if !passed {
			ws.Write([]byte("sadly, something has gone wrong with this session. Either an internal failure of the anti-DoS system has triggered, try again maybe?"))
			log.Printf("DIDNT PASS!?")
			return
		}
	}

	// dump link to recording
	ws.Write([]byte(fmt.Sprintf("\r\n\r\nYou have finished! You took %1f seconds to compete the challenges!", time.Since(startTimer).Seconds())))
	ws.Write([]byte(fmt.Sprintf("\r\n\r\nShould you want to show off to someone your session here is a link to the recording:")))
	ws.Write([]byte(fmt.Sprintf("\r\nhttps://yccup.benjojo.co.uk/replay/?s=%s \r\n ", sessionHash)))

	// dump high score board
}

func sendCurrentChallenge(ws *websocket.Conn, cs challengeSpec) {
	// [20:29:14] ben@metropolis:~$ echo -e '\033]2;AAAA\007' | hexdump -C
	// 00000000  1b 5d 32 3b 41 41 41 41  07 0a                    |.]2;AAAA..|

	start := []byte{0x1b, 0x5d, 0x30, 0x3b}
	end := []byte{0x07, 0x0a}
	res := string(start) + fmt.Sprintf("%s|%s", cs.Title, cs.Description) + string(end)
	ws.Write([]byte(res))
	ws.Write([]byte(fmt.Sprintf("\r\n======\r\nCurrent Challenge: %s\r\n - %s \r\n\r\n", cs.Title, cs.Description)))
}

var errHandshakeFailure = fmt.Errorf("Failed to handshake proper")

func jsHandshake(ws *websocket.Conn) error {
	ws.Write([]byte("blog.benjojo.co.uk - you can't curl under pressure"))
	handshake := make([]byte, 100)
	n, err := ws.Read(handshake)
	if err != nil {
		return err
	}

	if strings.Contains(string(handshake[:n]), "quite.") {
		return nil
	}
	return errHandshakeFailure
}

func makeHash() string {
	r := make([]byte, 8)
	rand.Read(r)

	return fmt.Sprintf("%x", r)
}

func getChallengeList() []challengeSpec {
	a := make([]challengeSpec, 0)

	for level := 0; level < 6; level++ {
		for _, v := range challenges {
			if v.Stage == level {
				a = append(a, v)
			}
		}
	}
	return a
}
