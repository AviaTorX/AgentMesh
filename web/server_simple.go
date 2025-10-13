// Simple HTTP server for static files
// Run: go run web/server_simple.go
package main

import (
	"fmt"
	"log"
	"net/http"
)

func main() {
	fs := http.FileServer(http.Dir("web/static"))
	http.Handle("/", fs)

	port := 8080
	fmt.Printf("Web server running on http://localhost:%d\n", port)
	fmt.Println("Note: WebSocket features require full server (see web/server.go)")
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", port), nil))
}
