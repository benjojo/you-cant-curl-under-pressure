package main

import (
	"crypto/hmac"
	"crypto/md5"
	"crypto/sha256"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

func rootTestHandler(w http.ResponseWriter, r *http.Request) {
	vm := locateVM(r)

	if vm == nil {
		http.Error(w, "Failed to locate the VM", 500)
		return
	}

	passed := false

	if vm.TestAssigned == "REDIRECT" {
		passed = testFollowRedirect(w, r)
	} else {
		switch vm.TestAssigned {
		case "BASIC":
			passed = testBasicReq(w, r)
		case "BASICAUTH":
			passed = testBasicAuthReq(w, r)
		case "HEADER":
			passed = testBasicHeaderReq(w, r)
		case "DELETE":
			passed = testDeleteReq(w, r)
		case "UPLOAD":
			passed = testUploadText(w, r)
		case "UPLOADBINARY":
			passed = testUploadBinary(w, r)
		case "UPLOADMIME":
			passed = testPOSTMIME(w, r)
		case "UPLOADMIMEFILE":
			passed = testPOSTMIMEFile(w, r)
		case "UPLOADVALUES":
			passed = testPOSTvalues(w, r)
		}
	}

	if passed {
		vm.TestComplete <- true
		vm.QEMU.Process.Kill()
		vm.QEMU.Process.Wait()
	} else {
		http.Error(w, "Incorrect", 400)
	}

}

func testBasicReq(w http.ResponseWriter, r *http.Request) bool {
	return true
}

func testBasicAuthReq(w http.ResponseWriter, r *http.Request) bool {
	_, _, ok := r.BasicAuth()
	return ok
}

func testBasicHeaderReq(w http.ResponseWriter, r *http.Request) bool {
	if r.Header.Get("X-Hello") == "World" {
		return true
	}
	return false
}

func testDeleteReq(w http.ResponseWriter, r *http.Request) bool {
	return r.Method == "DELETE"
}

func testUploadText(w http.ResponseWriter, r *http.Request) bool {
	b, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return false
	}
	sb := string(b)
	lines := len(strings.Split(sb, "\n"))
	if strings.Contains(sb, "root:x:0:0:root:") && lines > 3 {
		return true
	}
	return false
}

func testUploadBinary(w http.ResponseWriter, r *http.Request) bool {
	b, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return false
	}

	bhash := md5.Sum(b)
	hash := fmt.Sprintf("%x", bhash)
	if strings.ToLower(hash) == "9348516a4738c8ffcc8a4bbcc8a873f5" {
		return true
	}

	return false
}

func testPOSTMIME(w http.ResponseWriter, r *http.Request) bool {
	err := r.ParseMultipartForm(10 * 1000000)
	if err != nil {
		return false
	}

	if r.FormValue("a") != "b" {
		return false
	}

	return true
}

func testPOSTvalues(w http.ResponseWriter, r *http.Request) bool {
	err := r.ParseForm()
	if err != nil {
		return false
	}

	if r.FormValue("a") != "b" {
		return false
	}

	return true
}

func testPOSTMIMEFile(w http.ResponseWriter, r *http.Request) bool {
	err := r.ParseMultipartForm(10 * 1000000)
	if err != nil {
		return false
	}

	if r.FormValue("a") != "b" {
		return false
	}

	mpfile, _, err := r.FormFile("curl")
	if err != nil {
		return false
	}

	b, err := ioutil.ReadAll(mpfile)
	if err != nil {
		return false
	}

	bhash := md5.Sum(b)
	hash := fmt.Sprintf("%x", bhash)
	if strings.ToLower(hash) == "9348516a4738c8ffcc8a4bbcc8a873f5" {
		return true
	}

	return false
}

func testFollowRedirect(w http.ResponseWriter, r *http.Request) bool {
	if r.URL.Query().Get("s") == "" {
		key := fmt.Sprint(time.Now().Unix())
		mac := hmac.New(sha256.New, []byte("noodles"))
		mac.Write([]byte(key))
		expectedMAC := mac.Sum(nil)
		http.Redirect(w, r, fmt.Sprintf("/?s=%s-%s", key, url.QueryEscape(string(expectedMAC))), 301)
	} else {
		ch := r.URL.Query().Get("s")
		chb := strings.Split(ch, "-")
		i, err := strconv.ParseInt(chb[0], 10, 64)
		if err != nil {
			return false
		}

		if len(chb) != 2 {
			return false
		}

		if time.Since(time.Unix(i, 0)) > time.Second*2 {
			return false
		}

		mac := hmac.New(sha256.New, []byte("noodles"))
		mac.Write([]byte(chb[0]))
		if hmac.Equal(mac.Sum(nil), []byte(chb[1])) {
			return true
		}
	}
	return false
}
