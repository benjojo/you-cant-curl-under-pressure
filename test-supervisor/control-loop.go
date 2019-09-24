package main

import (
	"log"
	"sync"
	"syscall"
	"time"
)

type vmPool struct {
	Available  int
	Running    int
	Target     int
	TurboLimit int
	Pool       map[int]*vm
	LastKill   time.Time // last time since it culled a old VM
	LastBoot   time.Time // last time since it booted a VM
	lock       sync.Mutex
}

func startAndMaintainVMPool() *vmPool {
	vmP := vmPool{
		Target:     5,
		TurboLimit: 90,
		Pool:       make(map[int]*vm),
		LastKill:   time.Now(),
	}

	go vmP.run()
	return &vmP
}

func (v *vmPool) Grab() *vm {
	v.lock.Lock()
	defer v.lock.Unlock()

	for _, vm := range v.Pool {
		if !vm.InUse {
			vm.InUse = true
			return vm
		}
	}

	if v.TurboLimit > v.Running {
		// oh crap, start one now and hope for the best
		vm := produceVM()
		v.LastBoot = time.Now()
		v.Running++
		v.Available++
		v.Pool[vm.PID] = vm
		vm.InUse = true

		return vm
	}

	return nil
}

func (v *vmPool) run() {
	for {
		time.Sleep(time.Millisecond * 500)
		// fmt.Print("A")
		// Check if we need to load more VMs
		if v.Running < v.Target && v.Running != v.Target {
			// We are running less machines than we want as a min
			if time.Since(v.LastBoot) > time.Second {
				vm := produceVM()
				v.lock.Lock()
				v.LastBoot = time.Now()
				v.Running++
				v.Available++

				v.Pool[vm.PID] = vm

				v.lock.Unlock()
			}
			// fmt.Print("B")
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
		// fmt.Print("C")

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
		v.Available = running - inUse
		v.Running = running
		v.lock.Unlock()
		// fmt.Print("D")

		// Check if we are facing pressure on Available VMs
		if (float64(v.Available) / float64(v.Running)) < getControlValue(v.Available) {
			log.Printf("Expanding due to pressure [A: %d R: %d: P: %f]", v.Available, v.Running, (float64(v.Available) / float64(v.Running)))
			if v.Running > v.TurboLimit {
				log.Printf("PRESSURE LIMIT HIT! MORE MACHINES NEEDED")
			} else {
				if time.Since(v.LastBoot) > time.Second {
					vm := produceVM()
					v.lock.Lock()
					v.LastBoot = time.Now()
					v.Running++
					v.Available++

					v.Pool[vm.PID] = vm
					v.lock.Unlock()
				}
			}

		}
		// fmt.Print("E")

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
