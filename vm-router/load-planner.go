package main

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/net/websocket"
)

var (
	serverStates = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "yccup_server_state",
			Help: "aaaaaaaa",
		},
		[]string{"server"},
	)
	serverAvail = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "yccup_server_avail",
			Help: "aaaaaaaa",
		},
		[]string{"server"},
	)
	serverRunning = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "yccup_server_running",
			Help: "aaaaaaaa",
		},
		[]string{"server"},
	)
	globalCount = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "yccup_server_overall",
			Help: "aaaaaaaa",
		},
		[]string{"stat"},
	)
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

func intToState(in int) string {
	switch in {
	case STATE_OFFLINE:
		return "STATE_OFFLINE"
	case STATE_ONLINE:
		return "STATE_ONLINE"
	case STATE_ALWAYS_ONLINE:
		return "STATE_ALWAYS_ONLINE"
	case STATE_BOOTING:
		return "STATE_BOOTING"
	case STATE_WINDDOWN:
		return "STATE_WINDDOWN"
	case STATE_FAULTY:
		return "STATE_FAULTY"
	}
	return "UNKNOWN"
}

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
		if sip == nil {
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

	for _, v := range lp.Servers {
		err := v.update()
		if err == nil && v.State == STATE_OFFLINE {
			v.State = STATE_ONLINE
			v.StateChangeTime = time.Now()
			v.OnlineSince = time.Now()
		}
	}

	go lp.maintain()

	return &lp
}

func handleProbe(w http.ResponseWriter, r *http.Request) {
	auth := r.URL.Query().Get("auth")
	h := md5.Sum([]byte(auth))
	authh := hex.EncodeToString(h[:])
	if authh != "f005e3068a5f87b34e6999dc3d98f1d6" {
		http.Error(w, fmt.Sprintf("Failed to auth - %s", authh), http.StatusForbidden)
		return
	}

	for _, v := range globalLoadPlanner.Servers {
		if v.State == STATE_OFFLINE {
			err := v.update()
			if err == nil && v.State == STATE_OFFLINE {
				v.State = STATE_ONLINE
				v.StateChangeTime = time.Now()
				v.OnlineSince = time.Now()
			}
		}
	}
}

func (l *loadPlanner) getServer() *websocket.Conn {
	leastLoaded := &serverState{
		Available: -1,
	}

	for _, ss := range l.Servers {
		if ss.State == STATE_ONLINE || ss.State == STATE_ALWAYS_ONLINE {
			if ss.Available > leastLoaded.Available {
				leastLoaded = ss
			}
		}
	}

	ws, err := websocket.Dial(fmt.Sprintf("ws://[%s]:10000/serve", leastLoaded.IP.String()), "", "http://benjojo.co.uk")
	if err != nil {
		log.Printf("Failed to dial server (%s), retrying in 500ms", err.Error())
		leastLoaded.failureCount++
		time.Sleep(time.Millisecond * 500)
		return l.getServer()
	}

	go func() {
		for {
			codec := websocket.Codec{Marshal: func(v interface{}) (data []byte, payloadType byte, err error) {
				return nil, websocket.PingFrame, nil
			}}
			if err := codec.Send(ws, nil); err != nil {
				ws.Close()
			}
			time.Sleep(time.Second)
		}
	}()

	return ws
	// leastLoaded.IP
}

func (l *loadPlanner) maintain() {
	var turboCapacityUsed, bootedCapacityUsed float64

	for {
		log.Printf("Turbo Capacity ready to use %1f percent | Unused Booted capacity: %1f percent", turboCapacityUsed, bootedCapacityUsed)
		globalCount.WithLabelValues("turbo").Set(turboCapacityUsed)
		globalCount.WithLabelValues("ubc").Set(bootedCapacityUsed)

		time.Sleep(time.Millisecond * 500)
		for _, server := range l.Servers {
			// log.Printf("%s (%s)- A: %d R: %d (L: %1f)", server.IP, intToState(server.State), server.Available, server.Running, server.Load.Current)
			serverStates.WithLabelValues(server.IP.String()).Set(float64(server.State))
			serverAvail.WithLabelValues(server.IP.String()).Set(float64(server.Available))
			serverRunning.WithLabelValues(server.IP.String()).Set(float64(server.Running))
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

				if server.Load.Current < 0.2 && time.Since(server.OnlineSince) > time.Hour/2 {
					server.State = STATE_WINDDOWN
					server.StateChangeTime = time.Now()
				}

			}
			if server.State == STATE_OFFLINE {
				// Check if we need more hosts to be online to
				// deal with the demand

				systemsAvailable := 0
				systemsRunning := 0
				turboCapacity := 0
				systemsBooting := 0
				for _, ss := range l.Servers {
					if ss.State == STATE_ONLINE {
						systemsAvailable += ss.Available
						systemsRunning += ss.Running
						turboCapacity += 90
					}
					if ss.State == STATE_ALWAYS_ONLINE {
						systemsAvailable += ss.Available
						systemsRunning += ss.Running
						turboCapacity += 30
					}
					if ss.State == STATE_BOOTING {
						systemsBooting++
					}
				}

				turboCapacityUsed = (float64(systemsRunning) / float64(turboCapacity) * 100)
				bootedCapacityUsed = (float64(systemsAvailable) / float64(systemsRunning) * 100)
				/// If we have 80% of our possible spawnable capacity running, and 50% of that capacity is used, boot more systems.
				if (turboCapacityUsed > 80 && bootedCapacityUsed < 50 &&
					systemsBooting == 0) || (systemsAvailable == 0 && systemsRunning < 15 && systemsBooting == 0) {
					// boot the system
					err := windsorBoot(server.IP)
					if err != nil {
						log.Printf("failed to boot node %s, moving to faulty state", server.IP.String())
						server.State = STATE_FAULTY
						server.StateChangeTime = time.Now()
						server.failureCount = 0
						server.Available = 0
						server.Running = 0
						continue
					}

					server.State = STATE_BOOTING
					server.StateChangeTime = time.Now()
					server.failureCount = 0
					server.Available = 0
					server.Running = 0
				}
			}
			if server.State == STATE_BOOTING {
				// Check if the system is booted
				err := server.update()
				if err == nil {
					server.State = STATE_ONLINE
					server.StateChangeTime = time.Now()
					server.OnlineSince = time.Now()
					continue
				}

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
				err := server.update()
				if err != nil {
					server.failureCount++
				}

				if (server.Available == server.Running) ||
					server.failureCount == 4 ||
					time.Since(server.StateChangeTime) > time.Minute*10 {
					// power down the node
					go windsorPowerDown(server.IP)
					server.State = STATE_OFFLINE
					server.Available = 0
					server.Running = 0
					server.failureCount = 0
					server.StateChangeTime = time.Now()
					continue
				}

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

	// Parse JSON response

	jbytes, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return err
	}

	sE := statusEndpointResponse{}
	err = json.Unmarshal(jbytes, &sE)
	if err != nil {
		return err
	}

	s.Running = sE.Running
	s.Available = sE.Available
	s.Load.UpdateNow(float64(s.Running - s.Available))

	return nil
}

type statusEndpointResponse struct {
	Running   int
	Available int
}
