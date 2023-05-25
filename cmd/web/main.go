package main

import (
	"log"
	"net/http"

	"github.com/zdgeier/jamsync/internal/jamenv"
	"github.com/zdgeier/jamsync/internal/web"
	"github.com/zdgeier/jamsync/internal/web/authenticator"
)

var (
	version string
	built   string
)

func main() {
	log.Println("version: " + version)
	log.Println("built: " + built)
	log.Println("env: " + jamenv.Env().String())
	auth, err := authenticator.New()
	if err != nil {
		log.Panicf("Failed to initialize the authenticator: %v", err)
	}

	rtr := web.New(auth)

	log.Print("Server listening on http://0.0.0.0:8081/")

	if err := http.ListenAndServe("0.0.0.0:8081", rtr); err != nil {
		log.Panicf("There was an error with the http server: %v", err)
	}
}
