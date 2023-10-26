package main

import (
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"bkr-go/client/proto/clientpb"

	"google.golang.org/protobuf/proto"
)

func main() {
	batchSize := flag.Int("batch", 2, "batch size")
	payloadSize := flag.Int("payload", 200, "payload size")
	keyPath := flag.String("key", "../crypto", "path of ECDSA private key")
	url := flag.String("url", "http://127.0.0.1:11300/client", "url of client")
	testTime := flag.Int("time", 60, "test time")
	output := flag.String("output", "client.log", "output file")
	flag.Parse()

	buffer, err := generatePayload(*batchSize, *payloadSize, *keyPath)
	if err != nil {
		panic(err)
	}

	client := &http.Client{
		Transport: &http.Transport{
			DialContext: (&net.Dialer{
				Timeout:   5 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			MaxIdleConns:        200,
			MaxIdleConnsPerHost: 200,
			IdleConnTimeout:     time.Duration(60),
		},
	}

	file, err := os.OpenFile(*output, os.O_CREATE|os.O_WRONLY|os.O_APPEND|os.O_TRUNC, 0666)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	startTime := time.Now()
	endTime := time.Now()
	oldTime := endTime
	for endTime.Sub(startTime) < time.Duration(*testTime)*time.Second {
		body := strings.NewReader(string(buffer))
		req, err := http.NewRequest("POST", *url, body)
		if err != nil {
			panic(err)
		}
		req.Header.Set("Content-Type", "application/x-protobuf")
		resp, err := client.Do(req)
		if err != nil {
			panic(err)
		}
		resp.Body.Close()
		endTime = time.Now()
		file.Write([]byte(fmt.Sprintf("%d\n", endTime.Sub(oldTime).Milliseconds())))
		oldTime = endTime
	}

	file.Write([]byte("start time: " + startTime.String() + "\n"))
	file.Write([]byte("end time: " + endTime.String() + "\n"))
	file.Write([]byte(fmt.Sprintf("%d\n", endTime.Sub(startTime).Milliseconds())))
}

func generatePayload(batchsize, payload int, key string) ([]byte, error) {
	message := &clientpb.ClientMessage{}
	for i := 0; i < payload; i++ {
		message.Payload += "a"
	}
	clientMessages := &clientpb.ClientMessages{}
	for i := 0; i < batchsize; i++ {
		clientMessages.Payload = append(clientMessages.Payload, message)
	}
	buffer, err := proto.Marshal(clientMessages)
	if err != nil {
		return nil, fmt.Errorf("proto.Marshal: %v", err)
	}
	return buffer, nil
}
