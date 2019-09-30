package main

import (
	"bufio"
	"bytes"
	"crypto/md5"
	"fmt"
	"log"
	"net"
	"strings"
	"time"
)

func listenForFTPDataConnections(address string) {
	l, err := net.Listen("tcp", address)
	if err != nil {
		log.Fatalf("Failed to listen for FTP, %s", err.Error())
	}

	failures := 0

	for {
		c, err := l.Accept()
		if err != nil {
			failures++
			if failures == 10 {
				log.Fatalf("Failing to listen on FTP connections anymore")
			}
			time.Sleep(time.Millisecond * 700)
			continue
		}

		go handleFTPDataconnection(c)
	}
}

func handleFTPDataconnection(c net.Conn) {
	vm := locateVM(c.RemoteAddr().String())
	if vm == nil {
		log.Printf("Unable to locate source of the connection")
		c.Close()
		return
	}

	defer c.Close()

	if vm.TestAssigned != "FTPGET" && vm.TestAssigned != "FTPUPLOAD" {
		return
	}

	if vm.TestAssigned == "FTPUPLOAD" {
		buffer := make([]byte, 600*1024) // max buffer size of 600kb
		buf := bytes.NewBuffer(buffer)
		bytesWritten := 0

		for {
			tbuf := make([]byte, 1500)
			n, err := c.Read(tbuf)
			if err != nil {
				break
			}

			bytesWritten += n
			if bytesWritten > 600*1024 {
				punishFailedAttempt(vm)
				log.Printf("Attempted DoS")
				return
			}
			buf.Write(tbuf[:n])
		}

		bhash := md5.Sum(buf.Bytes())
		hash := fmt.Sprintf("%x", bhash)
		if strings.ToLower(hash) == "8eb42d34c5744458fde534facdfe7869" {
			vm.TestComplete <- true
			vm.TestComplete <- true
		} else {
			log.Printf("Not expected data: got %x vs 8eb42d34c5744458fde534facdfe7869", bhash)
			return
		}

	}

	// Sink all data
	go func() {
		for {
			b := make([]byte, 10000)
			_, err := c.Read(b)
			if err != nil {
				c.Close()
				return
			}
		}
	}()

	timeout := time.NewTicker(time.Second * 20)

	select {
	case _ = <-vm.TestComplete:
		return
	case _ = <-timeout.C:
		return
	}
}

func listenForFTPConnections(address string) {
	l, err := net.Listen("tcp", address)
	if err != nil {
		log.Fatalf("Failed to listen for FTP, %s", err.Error())
	}

	failures := 0

	for {
		c, err := l.Accept()
		if err != nil {
			failures++
			if failures == 10 {
				log.Fatalf("Failing to listen on FTP connections anymore")
			}
			time.Sleep(time.Millisecond * 700)
			continue
		}

		go ftpHandleConnection(c)
	}
}

func punishFailedAttempt(vmo *vm) {
	vmo.FailedAttempts++
}

func ftpHandleConnection(c net.Conn) {
	vm := locateVM(c.RemoteAddr().String())
	if vm == nil {
		log.Printf("Unable to locate source of the connection")
		c.Close()
		return
	}

	defer c.Close()

	if vm.TestAssigned != "FTPGET" && vm.TestAssigned != "FTPUPLOAD" {
		return
	}
	defer punishFailedAttempt(vm)

	r, err := writeAndReadLine(c, "220 BnBlog 1.0 Server (test.arpa) [::ffff:127.0.0.1]\n")
	if err != nil || !strings.HasPrefix(r, "USER ") {
		return
	}

	r, err = writeAndReadLine(c, "331 Anonymous login ok, send your complete email address as your password\n")
	if err != nil || !strings.HasPrefix(r, "PASS ") {
		return
	}

	r, err = writeAndReadLine(c, "230 Anonymous access granted, restrictions apply\n")
	if err != nil || !strings.HasPrefix(r, "PWD") {
		return
	}

	if vm.TestAssigned == "FTPGET" {

		/*
			> 220 ProFTPD 1.3.3g Server (ftp.arin.net) [::ffff:199.212.0.151]
			< USER anonymous
			> 331 Anonymous login ok, send your complete email address as your password
			< PASS ftp@example.com
			> 230 Anonymous access granted, restrictions apply
			< PWD
			> 257 "/" is the current directory
			< CWD info
			250 CWD command successful
			< CWD current
			250 CWD command successful
			< EPSV
			229 Entering Extended Passive Mode (|||59681|)
			< TYPE I
			200 Type set to I
			< SIZE asn.txt
			213 13859350
			< RETR asn.txt
			150 Opening BINARY mode data connection for asn.txt (13859350 bytes)
			451 Transfer aborted. Broken pipe
		*/

		r, err = writeAndReadLine(c, "257 \"/\" is the current directory\n")
		if err != nil {
			return
		}

		for {
			if strings.HasPrefix(r, "CWD ") {
				r, err = writeAndReadLine(c, "250 CWD command successful\n")
				if err != nil {
					return
				}
				continue
			}

			if strings.HasPrefix(r, "EPSV") {
				// vm.TestAssigned = "PASSIVE-FTPGET"
				r, err = writeAndReadLine(c, "229 Entering Extended Passive Mode (|||1337|)\n")
				if err != nil {
					return
				}
				continue
			}

			if strings.HasPrefix(r, "TYPE ") {
				r, err = writeAndReadLine(c, "200 Type set to I\n")
				if err != nil {
					return
				}
				continue
			}

			if strings.HasPrefix(r, "SIZE ") {
				r, err = writeAndReadLine(c, "213 800\n")
				if err != nil {
					return
				}
				continue
			}

			if strings.HasPrefix(r, "RETR hello.tx") {
				vm.TestComplete <- true
				vm.TestComplete <- true
				return
			}
			return
		}

	} else {
		// UPLOAD

		/*
			220 Service Ready.
			USER bentest
			331 Username ok, need password
			PASS xxxx
			230 User logged in
			PWD
			257 "/bentest/" is current directory.
			EPSV
			502 Command not implemented
			PASV
			227 Entering Passive Mode (x,x,x,x,x,x)
			TYPE I
			200 Type set to Image
			STOR test
			150 Opening Passive mode data transfer for STOR
			226 Closing data connection, file transfer successful
			QUIT
			221 Service closing control connection
			BYE
		*/

		r, err = writeAndReadLine(c, "257 \"/\" is the current directory\n")
		if err != nil {
			return
		}

		for {
			if strings.HasPrefix(r, "CWD ") {
				r, err = writeAndReadLine(c, "250 CWD command successful\n")
				if err != nil {
					return
				}
				continue
			}

			if strings.HasPrefix(r, "EPSV") {
				// vm.TestAssigned = "PASSIVE-FTPGET"
				r, err = writeAndReadLine(c, "229 Entering Extended Passive Mode (|||1337|)\n")
				if err != nil {
					return
				}
				continue
			}

			if strings.HasPrefix(r, "TYPE ") {
				r, err = writeAndReadLine(c, "200 Type set to I\n")
				if err != nil {
					return
				}
				continue
			}

			if strings.HasPrefix(r, "SIZE ") {
				r, err = writeAndReadLine(c, "213 800\n")
				if err != nil {
					return
				}
				continue
			}

			if strings.HasPrefix(r, "STOR curl") {
				c.Write([]byte("150 SGTM\n"))
				time.Sleep(time.Second * 4)
				return
			}
			return
		}
	}

}

func writeAndReadLine(c net.Conn, write string) (s string, err error) {
	_, err = c.Write([]byte(write))
	if err != nil {
		return "", err
	}

	bio := bufio.NewReader(c)
	bline, tooBig, err := bio.ReadLine()
	if tooBig {
		log.Printf("Line is too big to buffer, things are likely about to go south now")
	}

	if err != nil {
		return "", err
	}

	return string(bline), nil
}
