package main

import (
	"fmt"
	"log"
	"net"
	"net/http"

	"github.com/weaveworks/procspy"
)

func locateVM(r *http.Request) *vm {
	fmt.Printf("debug! %s", r.RemoteAddr)
	cs, err := procspy.Connections(true)
	if err != nil {
		return nil
	}

	h, port, _ := net.SplitHostPort(r.RemoteAddr)

	for {
		c := cs.Next()
		if c == nil {
			break
		}

		log.Printf("Comparsing MH: %s MP: %s  RH: %s RP: %d ", h, port, c.RemoteAddress, c.RemotePort)
		if fmt.Sprint(c.LocalPort) == port {
			log.Printf("Suspect %s %d", c.LocalAddress.String(), c.LocalPort)
			globalPool.lock.Lock()
			if globalPool.Pool[int(c.PID)] != nil {
				globalPool.lock.Unlock()
				return globalPool.Pool[int(c.PID)]
			}
			globalPool.lock.Unlock()
		}
	}
	return nil
}
