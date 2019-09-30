package main

import (
	"fmt"
	"log"
	"net"

	"github.com/weaveworks/procspy"
)

func locateVM(RemoteAddr string) *vm {
	fmt.Printf("debug! %s", RemoteAddr)
	cs, err := procspy.Connections(true)
	if err != nil {
		return nil
	}

	h, port, _ := net.SplitHostPort(RemoteAddr)

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
				log.Printf("It is PID %d", c.PID)
				return globalPool.Pool[int(c.PID)]
			}
			log.Printf("Got the PID %d, but could not find it in our own pid table", c.PID)
			globalPool.lock.Unlock()
		}
	}
	return nil
}
