package server

import (
	"crypto/ecdsa"
	"fmt"
	"time"

	"pbft-go/crypto/sha256"
	"pbft-go/transport"
	"pbft-go/transport/http"
	"pbft-go/transport/info"
	"pbft-go/transport/message"

	"go.uber.org/zap"
)

// Node is a local process
type Node struct {
	id info.IDType
	n  uint32

	f    uint32
	seq  map[int]bool
	view uint32

	tp            transport.Transport // to communicate with other nodes
	reqChan       chan []byte         // client's request
	consensusChan chan *message.ConsMessage
	replyChan     chan []byte // serialized client requests

	lastReplySeq int

	proposals map[int]*message.ConsMessage
	prepares  map[int]map[int]*message.ConsMessage
	commits   map[int]map[int]*message.ConsMessage

	hasSentProposal map[int]bool
	hasSentPrepare  map[int]bool
	hasSentCommit   map[int]bool
	hasSentReply    map[int]bool

	impeachTimer *time.Timer
	lg           *zap.Logger
}

// InitNode initiate a node for processing messages
func InitNode(lg *zap.Logger, id info.IDType, n uint32, port int, peers []http.Peer, pk *ecdsa.PrivateKey) *Node {
	tp, msgc, reqc, repc := transport.InitTransport(lg, id, port, peers, pk)
	// blocksStore := make(map[string]*message.Block)
	// blocksStore[getBlockName(b0)] = b0
	// fmt.Println("genesis block: ", getBlockName(b0), " ", blocksStore[getBlockName(b0)])
	node := &Node{
		id:            id,
		n:             n,
		f:             n / 3,
		tp:            tp,
		reqChan:       reqc,
		consensusChan: msgc,
		replyChan:     repc,
		view:          0,

		seq:       make(map[int]bool),
		proposals: make(map[int]*message.ConsMessage),
		prepares:  make(map[int]map[int]*message.ConsMessage),
		commits:   make(map[int]map[int]*message.ConsMessage),

		lastReplySeq: 0,

		hasSentProposal: make(map[int]bool),
		hasSentPrepare:  make(map[int]bool),
		hasSentCommit:   make(map[int]bool),
		hasSentReply:    make(map[int]bool),

		lg: lg,
	}

	return node

}

func (n *Node) Run() {

	n.lg.Info("Begin to Test")
	if n.getLeader(n.view) == n.id {
		go n.propose()
	}

	n.impeachTimer = time.NewTimer(5000 * time.Millisecond)

	for {
		select {
		case msg := <-n.consensusChan:
			n.lg.Info("Receive Message",
				zap.String("type", msg.Type.String()),
				zap.Int("msgView", int(msg.View)),
				zap.Int("view", int(n.view)),
				zap.Int("seq", int(msg.Seq)),
				zap.Int("From", int(msg.From)),
			)
			if msg.View != uint32(n.view) {
				continue
			}
			switch msg.Type {
			case message.ConsMessage_PREPREPARE:
				n.onReceiveProposal(msg)
			case message.ConsMessage_PREPARE:
				n.onReceivePrepare(msg)
			case message.ConsMessage_COMMIT:
				n.onReceiveCommit(msg)
			default:
				panic("invalid message type!")
			}
		case <-n.impeachTimer.C:
			n.localTimeout() // do nothing
			// n.impeachTimer.Reset(5000 * time.Millisecond)
		}

	}
}

func (n *Node) onReceiveProposal(msg *message.ConsMessage) {
	n.proposals[int(msg.Seq)] = msg
	if !n.hasSentPrepare[int(msg.Seq)] {
		n.hasSentPrepare[int(msg.Seq)] = true
		PrepareMsg := &message.ConsMessage{}
		PrepareMsg.Type = message.ConsMessage_PREPARE
		PrepareMsg.Digest = msg.Digest
		PrepareMsg.Seq = msg.Seq
		PrepareMsg.View = uint32(n.view)
		go n.tp.Broadcast(PrepareMsg)
	}
	if n.seq[int(msg.Seq)] && !n.hasSentReply[int(msg.Seq)] {
		n.hasSentReply[int(msg.Seq)] = true
		// n.replyChan <- n.proposals[int(msg.Seq)].Block
		n.tryCommit(int(msg.Seq))
	}
}

func (n *Node) onReceivePrepare(msg *message.ConsMessage) {
	if n.prepares[int(msg.Seq)] == nil {
		n.prepares[int(msg.Seq)] = make(map[int]*message.ConsMessage)
	}
	n.prepares[int(msg.Seq)][int(msg.From)] = msg
	if !n.hasSentCommit[int(msg.Seq)] && len(n.prepares[int(msg.Seq)]) >= int(2*n.f+1) {
		n.hasSentCommit[int(msg.Seq)] = true
		CommitMsg := &message.ConsMessage{}
		CommitMsg.Type = message.ConsMessage_COMMIT
		CommitMsg.Digest = msg.Digest
		CommitMsg.Seq = msg.Seq
		CommitMsg.View = uint32(n.view)
		go n.tp.Broadcast(CommitMsg)
	}

}

func (n *Node) onReceiveCommit(msg *message.ConsMessage) {
	if n.commits[int(msg.Seq)] == nil {
		n.commits[int(msg.Seq)] = make(map[int]*message.ConsMessage)
	}
	n.commits[int(msg.Seq)][int(msg.From)] = msg
	if !n.hasSentReply[int(msg.Seq)] && len(n.commits[int(msg.Seq)]) >= int(2*n.f+1) {
		if n.proposals[int(msg.Seq)] != nil {
			n.hasSentReply[int(msg.Seq)] = true
			n.tryCommit(int(msg.Seq))
			// n.replyChan <- n.proposals[int(msg.Seq)].Block
		}
		// seq finished
		n.seq[int(msg.Seq)] = true
	}
}

func (n *Node) getLeader(view uint32) info.IDType {
	return info.IDType(view % uint32(n.n))
}

func (n *Node) tryCommit(seq int) {
	if seq == n.lastReplySeq+1 && n.proposals[int(seq)] != nil {
		n.lastReplySeq = n.lastReplySeq + 1
		fmt.Println("to commit: ", seq)
		n.replyChan <- n.proposals[int(seq)].Payload
		if n.seq[seq+1] {
			n.tryCommit(seq + 1)
		}
	}
}

func (n *Node) propose() {
	seq := 1
	for {
		// time.Sleep(time.Duration(500) * time.Millisecond)
		content := <-n.reqChan
		hash, err := sha256.ComputeHash(content)
		if err != nil {
			panic("sha256 hash digest failed!")
		}
		msg := &message.ConsMessage{
			Type:    message.ConsMessage_PREPREPARE,
			View:    n.view,
			Seq:     uint32(seq),
			Payload: content,
			Digest:  hash,
		}
		n.lg.Info("PREPREPARE",
			zap.String("type", msg.Type.String()),
			zap.Int("proposer", int(n.id)),
			zap.Int("view", int(n.view)),
			zap.Int("Seq", int(seq)),
		)
		seq = seq + 1
		go n.tp.Broadcast(msg)
	}

}

func (n *Node) localTimeout() {
	n.lg.Info("Trigger Timeout")
}
