package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
)

var (
	listenAddr string
	joinAddr   string
)

func init() {
	flag.StringVar(&listenAddr, "listen", "", "Set the address to listen for new request")
	flag.StringVar(&joinAddr, "join", "", "Set the load balancer address to join")
}

func main() {
	flag.Parse()
	svr := NewNodeInstance(context.Background())

	err := svr.Connect()
	if err != nil {
		log.Fatalln("Failed to connect to server '", joinAddr, "', got error:", err.Error())
	}

	terminate := make(chan os.Signal, 1)
	signal.Notify(terminate, os.Interrupt)
	<-terminate

	svr.Shutdown()
}
