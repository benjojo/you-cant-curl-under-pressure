package main

import (
	"bufio"
	"crypto/rand"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"
)

type vm struct {
	QEMU                 *exec.Cmd
	PID                  int
	StdoutOutput         []string
	StderrOutput         []string
	Stdin                io.WriteCloser
	Stdout               io.ReadCloser
	Stderr               io.ReadCloser
	StopReadingIntoArray bool
	InUse                bool
	TestAssigned         string
	TestComplete         chan bool
	Started              time.Time
	FailedAttempts       int // if it goes over 50 then kill the VM
}

var (
	inShm = flag.Bool("useShm", false, "Store temp VM disks in /dev/shm")
)

func produceVM(fin chan *vm) {

	fsName := fmt.Sprintf("./%s.ext2", randString(5))

	if *inShm {
		fsName = fmt.Sprintf("/dev/shm/%s.ext2", randString(5))
	}

	fs, err := os.Create(fsName)
	if err != nil {
		log.Fatalf("Unable to make VM Image %s", err.Error())
	}

	orig, err := os.Open(*hda)
	if err != nil {
		log.Fatalf("Unable to open head VM image %s", err.Error())
	}

	io.Copy(fs, orig)
	orig.Close()

	Result := vm{}
	// Ok so the pre-needed things are here.
	// let's start the VM

	qemu := exec.Command("/usr/bin/qemu-system-x86_64",
		"-m", "64",
		"-netdev", "user,id=n1", // Network Interface
		"-device", "virtio-net-pci,netdev=n1", // Network Interface
		"-device", "virtio-scsi-pci", // VirtIO disk
		"-drive", fmt.Sprintf("file=%s,if=virtio", fsName), // VirtIO disk
		"-kernel", "bzImage",
		"-append", "root=/dev/vda rw console=ttyS0",
		"-serial", "mon:stdio",
		"-nographic",
		"-object", "rng-random,filename=/dev/urandom,id=rng0", // TLS speedup
		"-device", "virtio-rng-pci,rng=rng0", // TLS Speedup
	)

	if *runas != "" {
		qemu = exec.Command("/usr/bin/qemu-system-x86_64",
			"-m", "64",
			"-netdev", "user,id=n1", // Network Interface
			"-device", "virtio-net-pci,netdev=n1", // Network Interface
			"-device", "virtio-scsi-pci", // VirtIO disk
			"-drive", fmt.Sprintf("file=%s,if=virtio", fsName), // VirtIO disk
			"-kernel", "bzImage",
			"-append", "root=/dev/vda rw console=ttyS0",
			"-serial", "mon:stdio",
			"-nographic",
			"-runas", *runas,
			"-object", "rng-random,filename=/dev/urandom,id=rng0", // TLS speedup
			"-device", "virtio-rng-pci,rng=rng0", // TLS Speedup
		)
	}

	Result.StdoutOutput = make([]string, 0)
	Result.StderrOutput = make([]string, 0)
	stdoutPipe, _ := qemu.StdoutPipe()
	stderrPipe, _ := qemu.StderrPipe()
	stdinPipe, _ := qemu.StdinPipe()
	Result.Stdout = stdoutPipe
	Result.Stderr = stderrPipe
	Result.Stdin = stdinPipe
	Result.TestComplete = make(chan bool, 1)

	err = qemu.Start()
	if err != nil {
		log.Fatalf("Unable to start qemu, %s", err.Error())
	}

	time.Sleep(time.Millisecond * 200)
	Result.QEMU = qemu
	go Result.readVMintoStringArray(stdoutPipe, "O")
	go Result.readVMintoStringArray(stderrPipe, "E")

	Result.getFullyBooted()

	// unlink them so they actually removed when qemu dies
	os.Remove(fsName)

	Result.PID = Result.QEMU.Process.Pid
	Result.Started = time.Now()

	fin <- &Result
}

func (v *vm) getFullyBooted() {
	started := time.Now()
	for {
		time.Sleep(time.Millisecond * 100)
		if len(v.StdoutOutput) == 0 {
			continue
		}
		if strings.Contains(v.StdoutOutput[len(v.StdoutOutput)-1], "bnblog login") {
			v.Stdin.Write([]byte("\nroot\n\n\n"))
			return
		}

		if strings.Contains(v.StdoutOutput[len(v.StdoutOutput)-1], "bnbloglogin") {
			v.Stdin.Write([]byte("\nroot\n\n\n"))
			return
		}

		if time.Since(started) > time.Second*3 {
			v.Stdin.Write([]byte("\n "))
		}
	}
}

func (v *vm) readVMintoStringArray(in io.ReadCloser, prefix string) {
	bio := bufio.NewReader(in)

	for {
		line, _, err := bio.ReadLine()
		if err != nil || v.StopReadingIntoArray {
			log.Printf("Disconnecting from VM's outputs")
			break
		}

		// log.Printf("%s: %s", prefix, string(line))

		if prefix == "E" {
			v.StderrOutput = append(v.StderrOutput, string(line))
		}

		if prefix == "O" {
			v.StdoutOutput = append(v.StdoutOutput, string(line))
		}
	}
}

func randString(n int) string {
	const alphanum = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	var bytes = make([]byte, n)
	rand.Read(bytes)
	for i, b := range bytes {
		bytes[i] = alphanum[b%byte(len(alphanum))]
	}
	return string(bytes)
}
