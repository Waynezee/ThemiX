package server

import (
	"bytes"
	"crypto/ecdsa"
	"encoding/base64"
	"fmt"
	"log"
	"time"

	myecdsa "SyncHS-go/crypto/ecdsa"
	"SyncHS-go/crypto/sha256"
	"SyncHS-go/transport"
	"SyncHS-go/transport/http"
	"SyncHS-go/transport/info"
	"SyncHS-go/transport/message"

	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
)

// Node is a local process
type Node struct {
	id info.IDType
	n  uint64

	tp             transport.Transport       // to communicate with other nodes
	reqc           chan []byte               // client's request
	consensusChan  chan *message.ConsMessage // consensus message type
	replyChan      chan []byte               // serialized client requests
	proposedHeight map[uint32]bool           // to prevent repeated proposals in a round

	view            uint32
	votes           map[string]map[int][]byte // store all votes of each block
	echos           map[string]map[int]bool   // store all echos of each block
	commits         map[string]map[int]bool   // store all commits of each block
	payloadsStore   [][]byte
	blocksStore     map[string]*message.Block // store all (blocks) proposals, only one in every round
	hasSentEcho     map[string]bool           // send ECHO message only one time
	hasVoted        map[string]bool           // send VOTE message only one time
	hasPreCommitted map[string]bool           // send COMMIT message only one time
	hasCommitted    map[string]bool           // send COMMIT message only one time
	blockCert       map[string]bool

	lastCommittedRound uint32
	highestQC          *message.QuorumCert
	leaf               *message.Block
	lockedBlock        *message.Block
	lastVoteFor        *message.Block

	impeachTimer *time.Timer

	lg *zap.Logger
	pk *ecdsa.PrivateKey
}

// InitNode initiate a node for processing messages
func InitNode(lg *zap.Logger, id info.IDType, n uint64, port int, peers []http.Peer, pk *ecdsa.PrivateKey) *Node {
	tp, msgc, reqc, repc := transport.InitTransport(lg, id, port, peers, pk)
	paylaodSample := []byte("Gensis Block")
	b0 := &message.Block{
		Height: 0,
	}
	b0.Payload = append(b0.Payload, paylaodSample)
	b0.Digest = getBlockHash(b0)
	qc0 := &message.QuorumCert{
		Block:  b0.Digest,
		Height: 0,
	}
	// blocksStore := make(map[string]*message.Block)
	// blocksStore[getBlockName(b0)] = b0
	// fmt.Println("genesis block: ", getBlockName(b0), " ", blocksStore[getBlockName(b0)])
	node := &Node{
		id:                 id,
		n:                  n,
		tp:                 tp,
		reqc:               reqc,
		consensusChan:      msgc,
		replyChan:          repc,
		proposedHeight:     make(map[uint32]bool),
		view:               0,
		votes:              make(map[string]map[int][]byte),
		echos:              make(map[string]map[int]bool),
		commits:            make(map[string]map[int]bool),
		blocksStore:        make(map[string]*message.Block),
		hasSentEcho:        make(map[string]bool),
		hasVoted:           make(map[string]bool),
		hasPreCommitted:    make(map[string]bool),
		hasCommitted:       make(map[string]bool),
		blockCert:          make(map[string]bool),
		lastCommittedRound: 0,
		highestQC:          qc0,
		leaf:               b0,
		lockedBlock:        b0,
		lastVoteFor:        b0,

		lg: lg,
		pk: pk,
	}
	node.blocksStore[getName(b0.Digest)] = b0
	fmt.Println("genesis block: ", getName(b0.Digest), " ", node.blocksStore[getName(b0.Digest)])
	return node

}

func (n *Node) Run() {

	n.lg.Info("Begin to Test")
	firstPropose := false
	n.impeachTimer = time.NewTimer(5000 * time.Millisecond)
	for {
		select {
		case payloadBytes := <-n.reqc:
			n.payloadsStore = append(n.payloadsStore, payloadBytes)
			if !firstPropose {
				firstPropose = true
				if n.getLeader(n.view) == n.id {
					n.propose()
				}
			}
		case msg := <-n.consensusChan:
			n.lg.Info("Receive Message",
				zap.String("type", msg.Type.String()),
				zap.Int("msgView", int(msg.View)),
				zap.Int("view", int(n.view)),
				zap.String("voteFor", getName(msg.VoteFor)),
				zap.Int("From", int(msg.From)),
			)
			if msg.View != uint32(n.view) {
				continue
			}
			switch msg.Type {
			case message.ConsMessage_PROPOSE:
				n.onReceiveProposal(msg)
			case message.ConsMessage_ECHO:
				n.onReceiveEcho(msg)
			// case message.ConsMessage_VECHO:
			// 	n.onReceiveVerifiedEcho(msg)
			case message.ConsMessage_VOTE:
				n.onReceiveVote(msg)
			case message.ConsMessage_COMMIT:
				n.onReceiveCommit(msg)
			default:
				panic("invalid message type!")
			}
		case <-n.impeachTimer.C:
			n.localTimeout() // do nothing
			n.impeachTimer.Reset(5000 * time.Millisecond)
		}

	}
}

func getBlockHash(block *message.Block) []byte {
	buffer, err := proto.Marshal(block)
	if err != nil {
		log.Fatal("protobuf marshal error: ", err)
	}
	hash, err := sha256.ComputeHash(buffer)
	if err != nil {
		log.Fatal("sha256 hash error: ", err)
	}
	return hash
}

// func getBlockName(block *message.Block) string {
// 	buffer, err := proto.Marshal(block)
// 	if err != nil {
// 		log.Fatal("protobuf marshal error: ", err)
// 	}
// 	hash, err := sha256.ComputeHash(buffer)
// 	if err != nil {
// 		log.Fatal("sha256 hash error: ", err)
// 	}
// 	name := base64.StdEncoding.EncodeToString(hash)
// 	return string(name)
// }

func getName(blockHash []byte) string {
	name := base64.StdEncoding.EncodeToString(blockHash)
	return string(name)
}

func (n *Node) onReceiveProposal(msg *message.ConsMessage) {
	blk := msg.Block
	blkName := getName(blk.Digest)
	n.blocksStore[blkName] = blk
	if n.echos[blkName] == nil {
		n.echos[blkName] = make(map[int]bool)
	}
	if n.votes[blkName] == nil {
		n.votes[blkName] = make(map[int][]byte)
	}
	if n.commits[blkName] == nil {
		n.commits[blkName] = make(map[int]bool)
	}
	if !n.hasSentEcho[blkName] {
		n.hasSentEcho[blkName] = true
		echoMsg := &message.ConsMessage{}
		echoMsg.Type = message.ConsMessage_ECHO
		echoMsg.Block = &message.Block{
			Parent: blk.Parent,
			Digest: blk.Digest,
			Height: blk.Height,
			Qc:     blk.Qc,
		}
		echoMsg.View = n.view

		n.lg.Info("SEND ECHO",
			zap.String("Type", echoMsg.Type.String()),
			zap.Int("View", int(n.view)),
			zap.String("Block", blkName),
		)
		go n.tp.Broadcast(echoMsg)
	}
	if !n.hasCommitted[blkName] && len(n.commits[blkName]) >= int(n.n/2+1) && n.blocksStore[blkName] != nil {
		n.hasCommitted[blkName] = true
		n.lg.Info("COMMIT",
			zap.String("blockName", blkName),
		)
		if n.blocksStore[blkName].Height != 0 {
			n.onCommit(n.blocksStore[blkName])
		}
	}

}

// verify cert in Echo. In the test, every node uses the same (pk,sk)
func (n *Node) onReceiveEcho(msg *message.ConsMessage) {
	if msg.Block.Qc.Height == 0 {
		n.onReceiveVerifiedEcho(msg)
	}
	qcBlock := getName(msg.Block.Qc.Block)
	sigs := msg.Block.Qc.Signature
	num := 0
	for i := 0; i < len(sigs); i++ {
		sig := sigs[i]
		if sig != nil {
			if n.votes[qcBlock][i] != nil && bytes.Equal(n.votes[qcBlock][i], sig) {
				num = num + 1
			} else {
				if n.VerifyVote(msg.Block.Qc.Block, sig) {
					num = num + 1
					n.votes[qcBlock][i] = sig
				}
			}
		}
	}
	// fmt.Println("num:", num, ", len(sig):", len(sigs))
	if num >= int(n.n*3/4+1) {
		// msg.Type = message.ConsMessage_VECHO
		// n.consensusChan <- msg
		n.onReceiveVerifiedEcho(msg)
	}

}

func (n *Node) VerifyVote(blockHash []byte, sig []byte) bool {
	b, err := myecdsa.VerifyECDSA(&n.pk.PublicKey, sig, blockHash)
	if err != nil {
		fmt.Println("Failed to verify a consmsgpb: ", err)
	}
	return b
}

func (n *Node) SignVote(blockHash []byte) []byte {
	sig, err := myecdsa.SignECDSA(n.pk, blockHash)
	if err != nil {
		panic("myecdsa signECDSA failed")
	}
	return sig
}

func (n *Node) onReceiveVerifiedEcho(msg *message.ConsMessage) {
	blk := msg.Block
	blkName := getName(blk.Digest)
	qcBlock := getName(blk.Qc.Block)
	if n.echos[blkName] == nil {
		n.echos[blkName] = make(map[int]bool)
	}
	n.echos[blkName][int(msg.From)] = true
	//On receiving <propose, Bk, v, C(Bk-1)> from f + 1 replicas, pre-commit Bk-1 and broadcast <commit, Bk-1, v> right away
	if !n.hasPreCommitted[qcBlock] && len(n.echos[blkName]) >= int(n.n/2+1) && getName(blk.Parent) == qcBlock {
		n.hasPreCommitted[qcBlock] = true
		commitMsg := &message.ConsMessage{
			Type:    message.ConsMessage_COMMIT,
			View:    n.view,
			VoteFor: blk.Qc.Block,
		}
		n.lg.Info("SEND COMMIT",
			zap.String("Type", commitMsg.Type.String()),
			zap.Int("View", int(n.view)),
			zap.String("Block", qcBlock),
		)
		go n.tp.Broadcast(commitMsg)
	}
	// if no leader equivocation is detected, broadcast a vote in the form of <vote, Bk, v>
	if !n.hasVoted[blkName] && getName(blk.Parent) == qcBlock && len(n.echos[blkName]) >= int(n.n*3/4+1) {
		//vote
		n.hasVoted[blkName] = true
		voteMsg := &message.ConsMessage{
			Type:    message.ConsMessage_VOTE,
			View:    n.view,
			VoteFor: blk.Digest,
			Vote:    n.SignVote(blk.Digest),
		}
		n.lg.Info("Vote",
			zap.String("Type", voteMsg.Type.String()),
			zap.Int("View", int(n.view)),
			zap.String("Block", blkName),
			zap.Int("BlockHeight", int(blk.Height)),
		)
		go n.tp.Broadcast(voteMsg)
	}
}

func (n *Node) onReceiveVote(msg *message.ConsMessage) {
	voteFor := getName(msg.VoteFor)
	if v := n.votes[voteFor]; v == nil {
		n.votes[voteFor] = make(map[int][]byte)
	}
	n.votes[voteFor][int(msg.From)] = msg.Vote
	if !n.blockCert[voteFor] && len(n.votes[voteFor]) >= int(n.n*3/4+1) && n.getLeader(n.view) == n.id {
		n.blockCert[voteFor] = true
		cert := n.createCert(msg.VoteFor)
		n.highestQC = cert
		n.leaf = n.blocksStore[voteFor]
		n.lg.Info("TO PROPOSE",
			zap.String("leafblockName", voteFor),
		)
		n.propose()
	}

}

func (n *Node) createCert(blockHash []byte) *message.QuorumCert {
	// var sigShares [][]byte
	sigShares := make([][]byte, n.n)
	for id, value := range n.votes[getName(blockHash)] {
		sigShares[id] = value
	}
	n.lg.Info("CREATE CERT",
		zap.String("blockName", getName(blockHash)),
	)
	return &message.QuorumCert{
		Block:     blockHash,
		Signature: sigShares,
	}
}

func (n *Node) onReceiveCommit(msg *message.ConsMessage) {
	blk := getName(msg.VoteFor)
	if n.commits[blk] == nil {
		n.commits[blk] = make(map[int]bool)
	}
	n.commits[blk][int(msg.From)] = true
	if !n.hasCommitted[blk] && len(n.commits[blk]) >= int(n.n/2+1) && n.blocksStore[blk] != nil {
		n.hasCommitted[blk] = true
		n.lg.Info("COMMIT",
			zap.String("blockName", blk),
		)
		if n.blocksStore[blk].Height != 0 {
			n.onCommit(n.blocksStore[blk])
		}
	}
}

func (n *Node) getLeader(view uint32) info.IDType {
	return info.IDType(view % uint32(n.n))
}

func (n *Node) onCommit(blk *message.Block) {
	//reply to clients
	for i := 0; i < len(blk.Payload); i++ {
		n.replyChan <- blk.Payload[i]
	}
}

func (n *Node) propose() {
	if n.proposedHeight[n.leaf.Height+1] {
		return
	}
	n.proposedHeight[n.leaf.Height+1] = true
	blk := &message.Block{
		Parent: n.leaf.Digest,
		Height: n.leaf.Height + 1,
		Qc:     n.highestQC,
	}
	blk.Payload = n.payloadsStore
	n.payloadsStore = make([][]byte, 0)
	blk.Digest = getBlockHash(blk)
	msg := &message.ConsMessage{
		Type:  message.ConsMessage_PROPOSE,
		View:  n.view,
		Block: blk,
	}
	n.lg.Info("PROPOSE",
		zap.String("type", msg.Type.String()),
		zap.Int("proposer", int(n.id)),
		zap.Int("view", int(n.view)),
		zap.Int("blockHeight", int(blk.Height)),
		zap.Int("payloadNumber", len(blk.Payload)),
		zap.String("Block", getName(blk.Digest)),
	)
	go n.tp.Broadcast(msg)
	// time.Sleep(100 * time.Millisecond)
}

func (n *Node) localTimeout() {
	n.lg.Info("Trigger Timeout")
}
