package main

import (
	"log"
	"net"
	"regexp"
	"strings"
	"time"
)

func listenForSMTPConnections(address string) {
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
				log.Fatalf("Failing to listen on SMTP connections anymore")
			}
			time.Sleep(time.Millisecond * 700)
			continue
		}

		go smtpHandleConnection(c)
	}
}

func smtpHandleConnection(c net.Conn) {
	vm := locateVM(c.RemoteAddr().String())
	if vm == nil {
		log.Printf("Unable to locate source of the connection")
		c.Close()
		return
	}

	defer c.Close()

	if vm.TestAssigned != "SMTP" {
		punishFailedAttempt(vm)
		return
	}
	defer punishFailedAttempt(vm)

	r, err := writeAndReadLine(c, "220 mx.blog.benjojo.co.uk ESMTP helpme.109 - bsmtp\n")
	if err != nil || !strings.HasPrefix(r, "EHLO ") {
		return
	}

	r, err = writeAndReadLine(c, `250-mx.blog.benjojo.co.uk god, let's get over with it, [0.0.0.0]
250-SIZE 157286400
250-8BITMIME
250-STARTTLS
250-ENHANCEDSTATUSCODES
250-PIPELINING
250-CHUNKING
250 SMTPUTF8
`)
	if err != nil || !strings.HasPrefix(r, "MAIL FROM:") {
		return
	}

	r, err = writeAndReadLine(c, "250 2.1.0 OK helpme.109 - bsmtp\n")
	if err != nil || !strings.HasPrefix(r, "RCPT TO") {
		return
	}

	r, err = writeAndReadLine(c, "250 2.1.0 OK helpme.109 - bsmtp\n")
	if err != nil || !strings.HasPrefix(r, "DATA") {
		return
	}

	c.Write([]byte("354  Go ahead - bsmtp\n"))

	email := make([]byte, 1000)
	n, err := c.Read(email)
	if err != nil {
		return
	}

	if emailRegex1.Match(email[:n]) {
		vm.TestComplete <- true
	}

	return
}

var (
	emailRegex1 = regexp.MustCompile(`To:.+<blog@benjojo.co.uk>`)
)

// 220 mx.google.com ESMTP f188si464031wme.109 - gsmtp
// EHLO email.txt
// 250-mx.google.com at your service, [2a0c:2f07:4663:4663:468a:5bff:fe9a:691e]
// 250-SIZE 157286400
// 250-8BITMIME
// 250-STARTTLS
// 250-ENHANCEDSTATUSCODES
// 250-PIPELINING
// 250-CHUNKING
// 250 SMTPUTF8
// MAIL FROM:<ops@benjojo.co.uk> SIZE=182
// 250 2.1.0 OK f188si464031wme.109 - gsmtp
// RCPT TO:<ben@benjojo.co.uk>
// 250 2.1.5 OK f188si464031wme.109 - gsmtp
// DATA
// 354  Go ahead f188si464031wme.109 - gsmtp
// From: Ops Cox <ops@benjojo.co.uk>
// To: Ben Cox <ben@benjojo.co.uk>
// Subject: testing stuff
// Date: Mon, 7 Sep 2019 08:45:16

// Dear Ben,
// Welcome to this example email. What a lovely day.

// .
// 550-5.7.1 [2a0c:2f07:4663:4663:468a:5bff:fe9a:691e] Our system has detected that
// 550-5.7.1 this message does not meet IPv6 sending guidelines regarding PTR
// 550-5.7.1 records and authentication. Please review
// 550-5.7.1  https://support.google.com/mail/?p=IPv6AuthError for more information
// 550 5.7.1 . f188si464031wme.109 - gsmtp
// QUIT
// --------------------------------------------------------------------------------------
// 220 mx.google.com ESMTP q19si445436wmj.37 - gsmtp
// EHLO email.txt
// 250-mx.google.com at your service, [185.230.223.8]
// 250-SIZE 157286400
// 250-8BITMIME
// 250-STARTTLS
// 250-ENHANCEDSTATUSCODES
// 250-PIPELINING
// 250-CHUNKING
// 250 SMTPUTF8
// MAIL FROM:<ops@benjojo.co.uk> SIZE=182
// 250 2.1.0 OK q19si445436wmj.37 - gsmtp
// RCPT TO:<ben@benjojo.co.uk>
// 250 2.1.5 OK q19si445436wmj.37 - gsmtp
// DATA
// 354  Go ahead q19si445436wmj.37 - gsmtp
// From: Ops Cox <ops@benjojo.co.uk>
// To: Ben Cox <ben@benjojo.co.uk>
// Subject: testing stuff
// Date: Mon, 7 Sep 2019 08:45:16
//
// Dear Ben,
// Welcome to this example email. What a lovely day.
//
//
// .
// 250 2.0.0 OK  1569876485 q19si445436wmj.37 - gsmtp
// QUIT
// 221 2.0.0 closing connection q19si445436wmj.37 - gsmtp
