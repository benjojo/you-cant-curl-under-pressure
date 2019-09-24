package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"sync"
	"time"
)

type loadPlanner struct {
	Servers []*serverState
}

type serverState struct {
	IP              net.IP
	Running         int
	Available       int
	Online          bool
	State           int
	lock            sync.Mutex
	StateChangeTime time.Time
	failureCount    int
	Load            *Ewma
	OnlineSince     time.Time
}

const (
	STATE_OFFLINE       = 1
	STATE_ONLINE        = 2
	STATE_ALWAYS_ONLINE = 3
	STATE_BOOTING       = 4
	STATE_WINDDOWN      = 5
	STATE_FAULTY        = 6
)

func startLoadPlanner(otherServers []string) *loadPlanner {
	lp := loadPlanner{
		Servers: make([]*serverState, 0),
	}

	loopback := net.ParseIP("::1")

	myself := serverState{
		IP:     loopback,
		Online: false,
		State:  STATE_ALWAYS_ONLINE,
		Load:   NewEwma(time.Minute),
	}

	lp.Servers = append(lp.Servers, &myself)

	for _, otherServer := range otherServers {
		sip := net.ParseIP(otherServer)
		if sip != nil {
			log.Printf("Invalid IP %s", otherServer)
			continue
		}

		ss := serverState{
			IP:     sip,
			Online: false,
			State:  STATE_OFFLINE,
			Load:   NewEwma(time.Minute),
		}

		lp.Servers = append(lp.Servers, &ss)
	}

	go lp.maintain()

	return &lp
}

func (l *loadPlanner) maintain() {
	for {
		time.Sleep(time.Millisecond * 500)
		for _, server := range l.Servers {

			//             +-------+
			//      +----> +OFFLINE| <--------+
			//      |      +--+----+          |
			// +----+-+       |           +---+----+
			// |FAULTY|       |           |WINDDOWN|
			// +-+--+-+       v           +---+----+
			//   ^  ^      +-------+          ^
			//   |  +------+BOOTING|          |
			//   |         +---+---+          |
			//   |             |              |
			//   |             |              |
			//   |             v              |
			//   |          +--+---+          |
			//   +----------+ONLINE+----------+
			//              +------+

			if server.State == STATE_ALWAYS_ONLINE {
				server.update()
				// Node is forced to always be online
				continue
			}

			if server.State == STATE_ONLINE {
				// File to update it's numbers and EWMA
				err := server.update()
				if err != nil {
					server.failureCount++
				}

				if server.failureCount == 5 {
					// clearly broken if it's failed 5 probes in a row
					server.State = STATE_FAULTY
					server.StateChangeTime = time.Now()
					server.failureCount = 0
					server.Available = 0
					server.Running = 0
					continue
				}

			}
			if server.State == STATE_OFFLINE {
				// Check if we need more hosts to be online to
				// deal with the demand
			}
			if server.State == STATE_BOOTING {
				// Check if the system is booted
				if time.Since(server.StateChangeTime) > time.Minute*5 {
					server.State = STATE_FAULTY
					server.StateChangeTime = time.Now()
					server.failureCount = 0
					server.Available = 0
					server.Running = 0
					continue
				}
			}
			if server.State == STATE_WINDDOWN {
				// Check if the load has dropped to 0 or if
				// it's been this way for over 10 mins
				//
				// Then shut it down -> OFFLINE
			}
			if server.State == STATE_FAULTY {
				// See how long it's been faulty for, if longer than
				// 30 mins then change to offline
				if time.Since(server.StateChangeTime) > time.Minute*30 {
					server.State = STATE_OFFLINE
					server.StateChangeTime = time.Now()
					server.failureCount = 0
					server.Available = 0
					server.Running = 0
					continue
				}
			}
		}
	}
}

var http1SecTimeout *http.Client

func (s *serverState) update() error {
	if http1SecTimeout == nil {
		http1SecTimeout = &http.Client{
			Timeout: time.Second,
		}
	}

	r, err := http1SecTimeout.Get(fmt.Sprintf("http://[%s]:10000/stats", s.IP.String()))
	if err != nil {
		return err
	}

	r.Body.Close() // CLEAR COMPILE ERROR
	// Parse JSON response

	return nil
}
