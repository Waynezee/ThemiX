package http

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/rpc"
	"strconv"
	"strings"
	"sync"

	myecdsa "SyncHS-go/crypto/ecdsa"
	"SyncHS-go/crypto/sha256"
	"SyncHS-go/transport/info"
	"SyncHS-go/transport/message"

	"github.com/perlin-network/noise"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
)

var (
	streamBufSize = 40960 * 16
)

type Peer struct {
	PeerID    uint32
	Addr      string
	PublicKey *ecdsa.PrivateKey
}

// HTTPTransport is responsible for message exchange among nodes
type HTTPTransport struct {
	id         info.IDType
	node       *noise.Node
	peers      map[uint32]*Peer
	msgc       chan *message.ConsMessage
	PrivateKey ecdsa.PrivateKey
	mu         sync.Mutex
}

type NoiseMessage struct {
	Msg *message.ConsMessage
}

func (m NoiseMessage) Marshal() []byte {
	data, err := proto.Marshal(m.Msg)
	if err != nil {
		log.Fatal(err)
	}
	return data
}

func UnmarshalNoiseMessage(buf []byte) (NoiseMessage, error) {
	m := NoiseMessage{Msg: new(message.ConsMessage)}
	err := proto.Unmarshal(buf, m.Msg)
	if err != nil {
		return NoiseMessage{}, err
	}
	return m, nil
}

// Broadcast msg to all peers
func (tp *HTTPTransport) Broadcast(msg *message.ConsMessage) {
	// fmt.Println("Broadcast:", msg.Type.String())
	msg.From = uint32(tp.id)
	Sign(msg, &tp.PrivateKey)
	tp.msgc <- msg

	for _, p := range tp.peers {
		if p != nil {
			go tp.SendMessage(p.PeerID, msg)
		}
	}
}

// Send a message to a specific node
func (tp *HTTPTransport) Send(id uint32, msg *message.ConsMessage) {
	msg.From = uint32(tp.id)
	if id == uint32(tp.id) {
		tp.msgc <- msg
		return
	}
	tp.SendMessage(id, msg)
}

// InitTransport executes transport layer initiliazation, which returns transport, a channel
// for received ConsMessage, a channel for received requests, and a channel for reply
func InitTransport(lg *zap.Logger, id info.IDType, port int, peers []Peer, ck *ecdsa.PrivateKey) (*HTTPTransport,
	chan *message.ConsMessage, chan []byte, chan []byte) {
	msgc := make(chan *message.ConsMessage, streamBufSize)
	tp := &HTTPTransport{id: id, peers: make(map[uint32]*Peer), msgc: msgc, mu: sync.Mutex{}}
	clients := make(map[uint32]*Peer)
	for i, p := range peers {
		if index := uint32(i); index != uint32(id) {
			tp.peers[index] = &Peer{PeerID: uint32(index), Addr: p.Addr[7:], PublicKey: p.PublicKey}
			addr := strings.Split(p.Addr[7:], ":")
			cport, _ := strconv.Atoi(addr[1])
			cport = cport + 200
			clientAddr := addr[0] + ":" + strconv.Itoa(cport)
			clients[index] = &Peer{PeerID: uint32(index), Addr: clientAddr, PublicKey: p.PublicKey}
		} else {
			tp.PrivateKey = *p.PublicKey
			ip := strings.Split(p.Addr, ":")
			node_port, _ := strconv.ParseUint(ip[2], 10, 64)
			node, _ := noise.NewNode(noise.WithNodeBindHost(net.ParseIP("127.0.0.1")),
				noise.WithNodeBindPort(uint16(node_port)), noise.WithNodeMaxRecvMessageSize(64*1024*1024))
			tp.node = node
		}
	}
	tp.node.RegisterMessage(NoiseMessage{}, UnmarshalNoiseMessage)
	tp.node.Handle(tp.Handler)
	err := tp.node.Listen()
	if err != nil {
		panic(err)
	}
	log.Printf("listening on %v\n", tp.node.Addr())

	reqChan := make(chan *message.ClientReq, streamBufSize)
	reqc := make(chan []byte, streamBufSize)
	repc := make(chan []byte, streamBufSize)

	rprocessor := &ClientMsgProcessor{
		n:       len(peers),
		lg:      lg,
		id:      id,
		reqChan: reqChan,
		reqc:    reqc,
		repc:    repc,
		port:    port,
		reqs:    new(message.Batch),
		reqNum:  0,
		startId: 1,
		clients: clients,
	}
	go rprocessor.run()
	// mux := http.NewServeMux()
	// mux.HandleFunc("/", http.NotFound)
	// mux.Handle(clientPrefix, rprocessor)
	// mux.Handle(clientPrefix+"/", rprocessor)
	// server := &http.Server{Addr: ":" + strconv.Itoa(port), Handler: mux}
	// server.SetKeepAlivesEnabled(true)

	// go server.ListenAndServe()

	return tp, msgc, reqc, repc
}

func (tp *HTTPTransport) SendMessage(id uint32, msg *message.ConsMessage) {
	m := NoiseMessage{Msg: msg}
	tp.node.SendMessage(context.TODO(), tp.peers[id].Addr, m)
}

func (tp *HTTPTransport) Handler(ctx noise.HandlerContext) error {
	obj, err := ctx.DecodeMessage()
	if err != nil {
		log.Fatal(err)
	}
	msg, ok := obj.(NoiseMessage)
	if !ok {
		log.Fatal(err)
	}
	go tp.OnReceiveMessage(msg.Msg)
	return nil
}

func (tp *HTTPTransport) OnReceiveMessage(msg *message.ConsMessage) {
	if msg.From == uint32(tp.id) {
		tp.msgc <- msg
		return
	}
	// fmt.Println("Receive:", msg.Type.String(), ",From:", msg.From)
	if Verify(msg, tp.peers[msg.From].PublicKey) {
		tp.msgc <- msg
	}
}

func Verify(msg *message.ConsMessage, pub *ecdsa.PrivateKey) bool {
	toVerify := &message.ConsMessage{
		Type:    msg.Type,
		From:    msg.From,
		View:    msg.View,
		Block:   msg.Block,
		VoteFor: msg.VoteFor,
		Vote:    msg.Vote,
	}
	content, err := proto.Marshal(toVerify)
	if err != nil {
		panic(err)
	}

	hash, err := sha256.ComputeHash(content)
	if err != nil {
		panic("sha256 computeHash failed")
	}
	b, err := myecdsa.VerifyECDSA(&pub.PublicKey, msg.Signature, hash)
	if err != nil {
		fmt.Println("Failed to verify a consmsgpb: ", err)
	}
	return b
}

func Sign(msg *message.ConsMessage, priv *ecdsa.PrivateKey) {
	msg.Signature = nil
	content, err := proto.Marshal(msg)
	if err != nil {
		panic(err)
	}
	hash, err := sha256.ComputeHash(content)
	if err != nil {
		panic("sha256 computeHash failed")
	}
	sig, err := myecdsa.SignECDSA(priv, hash)
	if err != nil {
		panic("myecdsa signECDSA failed")
	}
	msg.Signature = sig
}

// ClientMsgProcessor is responsible for listening and processing requests from clients
type ClientMsgProcessor struct {
	n       int
	port    int
	lg      *zap.Logger
	id      info.IDType
	reqs    *message.Batch
	reqNum  int
	startId int32
	clients map[uint32]*Peer

	reqChan chan *message.ClientReq
	reqc    chan []byte // send to proposer;
	repc    chan []byte // receive from state
}

func (cp *ClientMsgProcessor) run() {
	cp.startRpcServer()
	cp.ReplyServer()
}

func (cp *ClientMsgProcessor) startRpcServer() {
	rpc.Register(cp)
	rpc.HandleHTTP()
	listener, err := net.Listen("tcp", ":"+strconv.Itoa(cp.port))
	if err != nil {
		panic(err)
	}
	go http.Serve(listener, nil)
}

func (cp *ClientMsgProcessor) Request(req *message.ClientReq, resp *message.Response) error {
	payloadBytes, err := proto.Marshal(req)
	if err != nil {
		panic("Protobuf Marshal Failed")
	}
	cp.reqc <- payloadBytes
	return nil
}

func (cp *ClientMsgProcessor) ReplyServer() {
	var cli *rpc.Client
	var err error

	for {
		req := <-cp.repc
		clientReq := new(message.ClientReq)
		proto.Unmarshal(req, clientReq)
		if clientReq.ClientId == int32(cp.id) {
			if cli == nil {
				cli, err = rpc.DialHTTP("tcp", "127.0.0.1:"+strconv.Itoa(cp.port+100))
				if err != nil {
					panic(err)
				}
			}
			arg := &message.NodeReply{
				From:      uint32(cp.id),
				RequestID: uint32(clientReq.RequestId),
				ReqNum:    uint32(clientReq.ReqNum),
			}
			resp := &message.Response{}
			go cli.Call("Client.NodeFinish", arg, resp)
		}
	}
}
