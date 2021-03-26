package main

import (
	"context"
	"os"
	"os/signal"
)

func main() {
	lb := NewLoadBalancer(context.Background())

	terminate := make(chan os.Signal, 1)
	signal.Notify(terminate, os.Interrupt)
	<-terminate

	lb.Shutdown()
}
