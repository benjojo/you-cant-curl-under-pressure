package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"golang.org/x/net/websocket"
)

var hda = flag.String("hda", "rootfs.ext2", "")
var bzimage = flag.String("bzimage", "bzImage", "")
var runas = flag.String("runas", "", "The user you want to run the qemu as (see iptables proxy tricks)")
var globalPool *vmPool

func main() {
	fmt.Print(".")
	flag.Parse()
	http.HandleFunc("/stats", serverStats)
	http.Handle("/serve", websocket.Handler(serveChallenge))
	go http.ListenAndServe(":10000", nil)

	ChallengeHandler := http.NewServeMux()
	ChallengeHandler.HandleFunc("/", rootTestHandler)
	go http.ListenAndServe(":9999", ChallengeHandler)
	go listenForFTPConnections(":9995")
	go listenForFTPDataConnections(":9993")
	go listenForSMTPConnections(":9996")

	// start := time.Now()
	// produceVM()
	// log.Printf("================ Booted in %s ======================", time.Since(start).String())

	vP := startAndMaintainVMPool()
	globalPool = vP
	for {
		time.Sleep(time.Second * 1)
		log.Printf("[A: %d R: %d: P: %f PT: %f L: %f, B: %d]",
			vP.Available, vP.Running, (float64(vP.Available) / float64(vP.Running)), getControlValue(vP.Available), vP.Load.Current, vP.SystemsBooting)
	}
}

type statusEndpointResponse struct {
	Running   int
	Available int
}

func serverStats(w http.ResponseWriter, r *http.Request) {
	if globalPool == nil {
		http.Error(w, "not ready", http.StatusInternalServerError)
		return
	}
	sresp := statusEndpointResponse{
		Available: globalPool.Available,
		Running:   globalPool.Running,
	}

	b, _ := json.Marshal(sresp)
	w.Write(b)
}

func serveChallengeWithName(ws *websocket.Conn, ChallengeCode string) {

	log.Printf("Grabbing VM")
	vm := globalPool.Grab()
	vm.TestAssigned = ChallengeCode
	vm.Stdin.Write([]byte("\n\nroot\n\n\n"))
	vm.StopReadingIntoArray = true
	vm.Stdin.Write([]byte("\n\n"))

	log.Printf("Disconnecting it from the array reader")
	buf := make([]byte, 1024)
	vm.Stdout.Read(buf)
	failures := 0
	backoff := 50
	for {
		vm.Stdin.Write([]byte("\n\n"))
		buf := make([]byte, 1024)
		n, _ := vm.Stdout.Read(buf)
		sbuf := string(buf[:n])
		tbuf := strings.Replace(strings.Replace(strings.Replace(sbuf, " ", "", -1), "\n", "", -1), "\r", "", -1)
		if tbuf == "##" {
			break
		} else {
			if strings.Contains(tbuf, "bnbloglogin") {
				vm.Stdin.Write([]byte("\nroot\n\n\n"))
			}

			log.Printf("trying again '%s' / '%x'", tbuf, []byte(tbuf))
			failures++
			backoff += 50
		}
		if failures > 5 {
			globalPool.lock.Lock()
			vm.QEMU.Process.Kill()
			vm.QEMU.Process.Wait()
			globalPool.Running--
			globalPool.lock.Unlock()
			serveChallengeWithName(ws, ChallengeCode)
			return
		}
		time.Sleep(time.Millisecond * time.Duration(backoff))
	}

	go func() {
		r := <-vm.TestComplete
		if r {
			ws.Write([]byte("TEST_PASSED"))
		}
		ws.Close()
	}()

	go io.Copy(ws, vm.Stdout)
	io.Copy(vm.Stdin, ws)

	vm.QEMU.Process.Kill()
	vm.QEMU.Process.Wait()
}

func serveChallenge(ws *websocket.Conn) {
	Chal := make([]byte, 100)
	n, err := ws.Read(Chal)
	if err != nil {
		return
	}
	SChal := string(Chal[:n])
	SChal = strings.Trim(SChal, "\r\n\t ")
	log.Printf("Challenge: %s", SChal)
	if strings.ToUpper(SChal) != SChal {
		log.Printf("Not fed with challenge name")
		return
	}

	serveChallengeWithName(ws, SChal)
}
