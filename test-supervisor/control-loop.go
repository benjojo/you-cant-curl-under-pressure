package main

import (
	"errors"
	"flag"
	"io/ioutil"
	"log"
	"os/exec"
	"path"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

type vmPool struct {
	Available      int
	Running        int
	StartTarget    int
	Target         int
	TurboLimit     int
	Pool           map[int]*vm
	LastKill       time.Time // last time since it culled a old VM
	LastBoot       time.Time // last time since it booted a VM
	lock           sync.Mutex
	Load           *Ewma
	SystemsBooting int
}

var idleTarget = flag.Int("idle.target", 5, "If the node got no jobs, how many VM's to keep around")
var turboLimit = flag.Int("turbo.limit", 90, "Max nodes to spawn before stopping")

func startAndMaintainVMPool() *vmPool {
	vmP := vmPool{
		StartTarget: *idleTarget,
		TurboLimit:  *turboLimit,
		Pool:        make(map[int]*vm),
		LastKill:    time.Now(),
		Load:        NewEwma(time.Minute * 5),
	}

	go vmP.run()
	return &vmP
}

func (v *vmPool) Grab() *vm {
	v.lock.Lock()
	for _, vm := range v.Pool {
		if !vm.InUse {
			vm.InUse = true
			v.lock.Unlock()
			return vm
		}
	}
	v.lock.Unlock()

	log.Printf("Request at me, but I have no machines to serve!!!")
	// None now? Let's see if there are more later
	ticker := time.NewTicker(time.Millisecond * 500)
	for {
		log.Printf("Checking for new VM's to serve")
		<-ticker.C
		v.lock.Lock()
		for _, vm := range v.Pool {
			if !vm.InUse {
				vm.InUse = true
				go ticker.Stop()
				v.lock.Unlock()
				return vm
			}
		}
		v.lock.Unlock()
	}

}

func (v *vmPool) run() {
	v.Target = v.StartTarget
	go v.startCPUsniper()

	for {
		time.Sleep(time.Millisecond * 500)
		// Check if we need to load more VMs
		if v.Load.Current > 0 {
			if v.StartTarget*2 < v.TurboLimit {
				v.Target = v.StartTarget * 2
			} else {
				v.Target = v.StartTarget
			}
		} else {
			v.Target = v.StartTarget
		}

		if v.Running < v.Target && v.Running != v.Target {
			// We are running less machines than we want as a min
			if (time.Since(v.LastBoot) > time.Second &&
				((v.Target-v.Running > v.SystemsBooting) || v.SystemsBooting == 0)) || v.Available < 0 {

				if v.SystemsBooting < 5 {
					v.LastBoot = time.Now()
					rr := make(chan *vm)
					v.lock.Lock()
					v.SystemsBooting++
					v.lock.Unlock()

					go produceVM(rr)

					go func() {
						vm := <-rr
						close(rr)

						v.lock.Lock()
						v.SystemsBooting--
						v.Running++
						v.Available++
						v.Pool[vm.PID] = vm
						v.lock.Unlock()
					}()
				}

			}
		}

		// Check if there are VM's running past their life expectency
		v.lock.Lock()
		for pid, vm := range v.Pool {
			// log.Printf("VM [%d] - Alive since: %s", vm.PID, time.Since(vm.Started).String())
			if time.Since(vm.Started) > time.Hour/2 {
				if vm.InUse {
					if time.Since(vm.Started) < ((time.Hour / 2) + (time.Minute * 5)) {
						// Fine. let it last a little longer before it's killed
						continue
					}
				}
				if time.Since(v.LastKill) > time.Second*30 {
					v.LastKill = time.Now() // call the cops, there is about to be a murder
					vm.QEMU.Process.Kill()
					vm.QEMU.Process.Wait()
					delete(v.Pool, pid)
					break // We are not going to kill anything else now, so we may as well break out
				}
			}
		}
		v.lock.Unlock()

		// Calculate how many VMs are used
		v.lock.Lock()
		inUse := 0
		running := 0
		for pid, vm := range v.Pool {
			testerr := vm.QEMU.Process.Signal(syscall.Signal(0))
			if testerr == nil {
				running++
			} else {
				delete(v.Pool, pid)
			}

			if vm.InUse {
				inUse++
			}
		}
		v.Load.UpdateNow(float64(inUse))
		v.Available = running - inUse
		v.Running = running
		v.lock.Unlock()

		// Check if we are facing pressure on Available VMs
		if (float64(v.Available)/float64(v.Running)) < getControlValue(v.Available) &&
			v.SystemsBooting < 4 {
			log.Printf("Expanding due to pressure [A: %d R: %d: P: %f L: %f B: %d]",
				v.Available, v.Running, (float64(v.Available) / float64(v.Running)), v.Load.Current, v.SystemsBooting)
			if (v.Running + v.SystemsBooting) > v.TurboLimit {
				log.Printf("PRESSURE LIMIT HIT! MORE MACHINES NEEDED")
			} else {
				if time.Since(v.LastBoot) > time.Second {
					v.LastBoot = time.Now()
					rr := make(chan *vm)
					v.lock.Lock()
					v.SystemsBooting++
					v.lock.Unlock()

					go produceVM(rr)

					go func() {
						vm := <-rr

						close(rr)
						v.lock.Lock()
						v.Running++
						v.SystemsBooting--
						v.Available++
						v.Pool[vm.PID] = vm
						v.lock.Unlock()
					}()
				}
			}

		}

	}

}

//  = 0.402 +( 0.0227 * x) - (0.00043 * x^2) + (0.000002868 * x^3)
// =0.911+(-0.19*ln(v.Available))
// for smooth scale

func getControlValue(Available int) float64 {
	return 0.8
	// return 0.5352833626360 - (-0.01482 * float64(Available)) + (00.000200023 * math.Pow(float64(Available), 2)) - (-0.00000103 * math.Pow(float64(Available), 3))
	// return (0.911 + (-0.19 * ln(float64(Available))))
}

// func ln(x float64) float64 {
// 	return math.Log(x) / math.Log(math.E)
// }

var (
	cpuSniperTime = flag.Int("cpu.sniper.seconds", 20, "How many seconds of CPU time a QEMU PID is allowed to use")
)

func (v *vmPool) startCPUsniper() {
	for {
		v.lock.Lock()
		for _, vv := range v.Pool {
			s, err := getPIDstat(vv.PID)
			if err != nil {
				log.Printf("Failed to get rusage of pid %d / %s", vv.PID, err.Error())
				continue
			}

			if s.utime > float64(*cpuSniperTime) {
				v.Running--
				if !vv.InUse {
					v.Available--
				}
				vv.QEMU.Process.Kill()
				vv.QEMU.Process.Wait()
				vv.TestComplete <- false
				log.Printf("VM consumed too much CPU time, so I nuked it")
			}

		}
		v.lock.Unlock()
		time.Sleep(time.Second)
	}

}

// Code for CPU usage is snipped and altered from:
// https://github.com/struCoder/pidusage
// MIT License
// Copyright (c) 2017 David 大伟

var cpuCLKTCK float64

type stat struct {
	utime  float64
	stime  float64
	cutime float64
	cstime float64
	start  float64
	rss    float64
	uptime float64
}

func parseFloat(val string) float64 {
	floatVal, _ := strconv.ParseFloat(val, 64)
	return floatVal
}

func formatStdOut(stdout []byte, userfulIndex int) []string {
	infoArr := strings.Split(string(stdout), "\n")[userfulIndex]
	ret := strings.Fields(infoArr)
	return ret
}

func getPIDstat(pid int) (*stat, error) {
	// default clkTck and pageSize
	var clkTck float64 = 100

	uptimeFileBytes, _ := ioutil.ReadFile(path.Join("/proc", "uptime"))
	uptime := parseFloat(strings.Split(string(uptimeFileBytes), " ")[0])

	if cpuCLKTCK == 0 {
		clkTckStdout, err := exec.Command("getconf", "CLK_TCK").Output()
		if err == nil {
			clkTck = parseFloat(formatStdOut(clkTckStdout, 0)[0])
		} else {
			clkTck = 100
		}
	} else {
		clkTck = cpuCLKTCK
	}

	procStatFileBytes, err := ioutil.ReadFile(path.Join("/proc", strconv.Itoa(pid), "stat"))
	if err != nil {
		return nil, errors.New("Can't find process with this PID: " + strconv.Itoa(pid))
	}

	splitAfter := strings.SplitAfter(string(procStatFileBytes), ")")

	if len(splitAfter) == 0 || len(splitAfter) == 1 {
		return nil, errors.New("Can't find process with this PID: " + strconv.Itoa(pid))
	}
	infos := strings.Split(splitAfter[1], " ")
	sstat := &stat{
		utime:  parseFloat(infos[12]) / clkTck,
		stime:  parseFloat(infos[13]) / clkTck,
		cutime: parseFloat(infos[14]) / clkTck,
		cstime: parseFloat(infos[15]) / clkTck,
		start:  parseFloat(infos[20]) / clkTck,
		rss:    parseFloat(infos[22]),
		uptime: uptime,
	}
	return sstat, nil

}
