package server

import (
	"sync"

	"go.themix.io/client/proto/clientpb"
	"go.themix.io/transport"
	"go.themix.io/transport/proto/consmsgpb"
	"go.uber.org/zap"
)

const channelSize = 4096 * 32

// Proposer is responsible for proposing requests
type Proposer struct {
	lg         *zap.Logger
	reqc       chan []byte
	verifyReq  chan *clientpb.ClientMessage
	verifyResp chan int
	tp         transport.Transport
	seq        uint64
	id         uint32
	lock       sync.Mutex
}

func initProposer(lg *zap.Logger, tp transport.Transport, id uint32, reqc chan []byte, pkPath string) *Proposer {
	proposer := &Proposer{lg: lg, tp: tp, id: id, reqc: reqc, lock: sync.Mutex{}}
	proposer.verifyReq = make(chan *clientpb.ClientMessage, channelSize)
	proposer.verifyResp = make(chan int, channelSize)
	go proposer.run()
	return proposer
}

func (proposer *Proposer) proceed(seq uint64) {
	if proposer.seq <= seq {
		proposer.reqc <- []byte{} // insert an empty reqeust
	}
}

func (proposer *Proposer) run() {
	var req []byte
	for {
		req = <-proposer.reqc
		proposer.propose(req)
	}
}

// Propose broadcast a propose consmsgpb with the given request and the current sequence number
func (proposer *Proposer) propose(request []byte) {
	msg := &consmsgpb.WholeMessage{
		ConsMsg: &consmsgpb.ConsMessage{
			Type:     consmsgpb.MessageType_VAL,
			Proposer: proposer.id,
			Sequence: proposer.seq,
			Content:  request,
		},
		From: proposer.id,
	}
	if len(request) > 0 {
		proposer.lg.Info("propose",
			zap.Int("proposer", int(msg.ConsMsg.Proposer)),
			zap.Int("seq", int(msg.ConsMsg.Sequence)),
			zap.Int("content", int(msg.ConsMsg.Content[0])))
	}
	proposer.seq++
	proposer.tp.Broadcast(msg)
}
