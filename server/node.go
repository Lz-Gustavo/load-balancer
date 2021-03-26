package main

import (
	"bufio"
	"context"
	"sync"
	"time"
)

const (
	heartbeatTimeout = time.Second
)

type NodeInstance struct {
	Load    int32
	Receive chan []byte
	reader  *bufio.Reader
	writer  *bufio.Writer

	mu     sync.Mutex
	t      *time.Timer
	cancel context.CancelFunc
}

// NewNodeInstance ...
func NewNodeInstance(ctx context.Context) *NodeInstance {
	return nil
}
