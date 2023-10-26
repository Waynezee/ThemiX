package main

import (
	"flag"
	"fmt"
	"net"
	"net/http"
	"net/rpc"
	"strconv"
	"sync"
	"time"

	"SyncHS-go/transport/message"
)

type Client struct {
	Id           int
	n            int
	f            int
	startId      int32
	port         int
	payloadSize  int
	testTime     int
	reqNum       int
	outputFile   string
	targetServer string
	startTime    map[int32]uint64
	received     map[int32]map[uint32]int
	replyChan    chan *message.NodeReply
	endChan      chan struct{}
	lock         sync.RWMutex
}

func (c *Client) startRpcServer() {
	rpc.Register(c)
	rpc.HandleHTTP()
	listener, err := net.Listen("tcp", "127.0.0.1:"+strconv.Itoa(c.port))
	if err != nil {
		panic(err)
	}
	go http.Serve(listener, nil)
}

func (c *Client) NodeFinish(msg *message.NodeReply, resp *message.Response) error {
	c.replyChan <- msg
	return nil
}

func (c *Client) run() {
	testTimer := time.NewTimer(time.Duration(c.testTime*1000) * time.Millisecond)
	fmt.Println("test start")
	payload := make([]byte, c.payloadSize*c.reqNum)
	cli, err := rpc.DialHTTP("tcp", c.targetServer+":"+strconv.Itoa(6100))
	if err != nil {
		// return
		panic(err)
	}
	for {
		select {
		case <-testTimer.C:
			fmt.Println("test end")
			return
		default:
			// send reqs to server int close loop
			req := &message.ClientReq{
				ClientId:  int32(c.Id),
				RequestId: c.startId,
				ReqNum:    int32(c.reqNum),
				Payload:   payload,
			}

			var resp message.Response
			startTime := time.Now().UnixNano() / 1000000
			fmt.Printf("send: %d %d %d\n", c.startId, c.reqNum, startTime)
			go cli.Call("ClientMsgProcessor.Request", req, &resp)
			<-c.replyChan
			endTime := time.Now().UnixNano() / 1000000
			fmt.Printf("recv: %d %d %d %d\n", c.startId, c.reqNum, endTime, endTime-startTime)
			c.startId += 1
		}
	}
}

func main() {
	id := flag.Int("id", 0, "client id")
	n := flag.Int("n", 0, "number of nodes")
	payloadSize := flag.Int("payload", 200, "payload size")
	// keyPath := flag.String("key", "../../../crypto", "path of ECDSA private key")
	port := flag.Int("port", 6100, "url of client")
	reqNum := flag.Int("reqnum", 1, "batch size")
	targetServer := flag.String("target", "127.0.0.1", "target server ip")
	testTime := flag.Int("time", 60, "test time")
	output := flag.String("output", "client.log", "output file")
	flag.Parse()
	fmt.Println(*id)
	c := &Client{
		Id:           *id,
		n:            *n,
		f:            *n / 3,
		startId:      1,
		port:         *port,
		payloadSize:  *payloadSize,
		reqNum:       *reqNum,
		testTime:     *testTime,
		outputFile:   *output,
		targetServer: *targetServer,
		startTime:    make(map[int32]uint64),
		received:     make(map[int32]map[uint32]int),
		replyChan:    make(chan *message.NodeReply, 1024),
		endChan:      make(chan struct{}),
		lock:         sync.RWMutex{},
	}
	c.startRpcServer()
	c.run()

	// c.dealReply()

}
