package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	_ "github.com/mattn/go-sqlite3"
	"github.com/zdgeier/jamhub/internal/jamenv"
	"github.com/zdgeier/jamhub/internal/jamhubgrpc"
)

var (
	version string
	built   string
)

func main() {
	log.Println("version: " + version)
	log.Println("built: " + built)
	log.Println("env: " + jamenv.Env().String())
	closer, err := jamhubgrpc.New()
	if err != nil {
		log.Panic(err)
	}
	log.Println("JamHub server is running...")

	done := make(chan os.Signal, 1)
	signal.Notify(done, syscall.SIGINT, syscall.SIGTERM)
	<-done

	log.Println("JamHub server is stopping...")

	closer()
}
