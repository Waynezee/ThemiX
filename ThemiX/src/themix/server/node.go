package server

import (
	"crypto/ecdsa"
	"runtime"

	"go.themix.io/crypto/bls"
	"go.themix.io/transport"
	"go.themix.io/transport/http"
	"go.uber.org/zap"
)

// Node is a local process
type Node struct {
	reply     chan []byte
	proposer  *Proposer
	transport *transport.Transport
}

// InitNode initiate a node for processing messages
func InitNode(lg *zap.Logger, blsSig *bls.BlsSig, pkPath string, id uint32, n uint64, port int, peers []http.Peer, batchsize int, ck *ecdsa.PrivateKey, sign bool) {
	tp, msgc, reqc, repc := transport.InitTransport(lg, id, port, peers, ck, sign, batchsize)
	proposer := initProposer(lg, tp, id, reqc, pkPath)
	state := initState(lg, tp, blsSig, pkPath, id, proposer, n, repc, batchsize)
	for i := 0; i < runtime.NumCPU()-1; i++ {
		go func() {
			for {
				msg := <-msgc
				state.insertMsg(msg)
			}
		}()
	}
	for {
		msg := <-msgc
		state.insertMsg(msg)
	}
}
