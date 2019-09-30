package main

import "net/http"

func handleRoot(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "https://blog.benjojo.co.uk/post/you-cant-curl-under-pressure", http.StatusTemporaryRedirect)
}
