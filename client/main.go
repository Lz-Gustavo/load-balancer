package main

import (
	"bufio"
	"errors"
	"flag"
	"load-balancer/pb"
	"log"
	"net"
	"os"
	"strconv"
	"strings"

	"google.golang.org/protobuf/proto"
)

var sendAddr string

func init() {
	flag.StringVar(&sendAddr, "send", "", "Set the address to send load requests")
}

func main() {
	flag.Parse()
	if sendAddr == "" {
		log.Fatalln("must set an address to send requests, example: ./client -send 127.0.0.1:9000")
	}

	con, err := net.Dial("tcp", sendAddr)
	if err != nil {
		log.Fatalln("failed to establish connection, err:", err.Error())
	}
	defer con.Close()

	rd := bufio.NewReader(os.Stdin)
	for {
		msg, err := rd.ReadString('\n')
		if err != nil {
			log.Println("failed to read:", msg, "got err:", err.Error(), "continuing...")
		}

		raw, err := marshalIntoProtobuffWithDelimeter(msg)
		if err != nil {
			log.Println("failed to marshal:", msg, "to protobuf, got err:", err.Error(), "continuing...")
		}

		_, err = con.Write(raw)
		if err != nil {
			log.Println("failed to send:", msg, "got err:", err.Error())
		}
	}
}

func marshalIntoProtobuffWithDelimeter(msg string) ([]byte, error) {
	s := strings.Split(msg, "-")
	if len(s) != 2 {
		return nil, errors.New("invalid message format, must send following LOAD-MAXEXECTIME example")
	}

	// parses load value, X value from X-Y input
	load, err := strconv.ParseInt(s[0], 10, 32)
	if err != nil {
		return nil, err
	}

	// parses exec time value, Y value from X-Y input
	t := strings.TrimSuffix(s[1], "\n")
	maxTime, err := strconv.ParseInt(t, 10, 32)
	if err != nil {
		return nil, err
	}

	r := &pb.Request{
		Load:        int32(load),
		MaxExecTime: int32(maxTime),
	}

	raw, err := proto.Marshal(r)
	if err != nil {
		return nil, err
	}

	m := append(raw, []byte("\n")...)
	return m, nil
}
