package main

import (
	"flag"
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

}
