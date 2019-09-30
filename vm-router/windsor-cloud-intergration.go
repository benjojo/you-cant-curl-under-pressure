package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"sync"
)

type windsorCloudServer struct {
	IP      string   `json:"IP"`
	PDUPort int      `json:"PDUPort"`
	Powered bool     `json:"Powered"`
	Tags    []string `json:"Tags"`
}

// Some actions can overload the PDU that backs the cluster, since
// actions like windsorPowerDown are done async, they _could_ all
// fire at once, overloading the backing PDU and causing it to drop
// some packets to remove power from systems, meaning that a zombie
// system emerges
var windsorLock sync.Mutex

func windsorPowerDown(IP net.IP) {
	windsorLock.Lock()
	defer windsorLock.Unlock()
	pduPort := windsorIPtoPDU(IP)
	if pduPort != -1 {
		r, err := http.Get(fmt.Sprintf("http://192.168.99.43:1333/off/?id=%d", pduPort))
		if err != nil {
			return
		}

		ioutil.ReadAll(r.Body)
	}
}

var errAlreadyInUse = fmt.Errorf("Server already booted for another use")
var errFailedToFind = fmt.Errorf("Server could not be located")

func windsorBoot(IP net.IP) error {
	pdu := -1
	castTov4 := IP.To4()
	cloud := windsorListAllServers()
	for _, v := range cloud {
		if v.IP == castTov4.String() {
			pdu = v.PDUPort
			if v.Powered {
				for _, tag := range v.Tags {
					if tag == "youcantcurl" {
						return nil
					}
				}
				return errAlreadyInUse
			}
		}
	}

	if pdu == -1 {
		return errFailedToFind
	}

	res, err := http.Get(fmt.Sprintf(
		"http://192.168.99.43:1333/on/?id=%d&nbdserver=192.168.99.38%%3A10809&nbdexport=cow-curl&tags=youcantcurl,automatic",
		pdu,
	))

	if err != nil {
		return err
	}

	res.Body.Close()
	return nil
}

func windsorListAllServers() (e []windsorCloudServer) {
	r, err := http.Get("http://192.168.99.43:1333/list")
	if err != nil {
		return e
	}

	b, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return e
	}
	err = json.Unmarshal(b, &e)
	return e
}

func windsorIPtoPDU(IP net.IP) int {
	castTov4 := IP.To4()
	cloud := windsorListAllServers()
	for _, v := range cloud {
		if v.IP == castTov4.String() {
			return v.PDUPort
		}
	}

	return -1
}
