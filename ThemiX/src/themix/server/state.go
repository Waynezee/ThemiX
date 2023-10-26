package server

import (
	"sync"

	"go.themix.io/crypto/bls"
	"go.themix.io/transport"
	"go.themix.io/transport/proto/consmsgpb"
	"go.uber.org/zap"
)

type state struct {
	lg          *zap.Logger
	tp          transport.Transport
	blsSig      *bls.BlsSig
	pkPath      string
	proposer    *Proposer
	id          uint32
	n           uint64
	collected   uint64
	execs       map[uint64]*asyncCommSubset
	lock        sync.RWMutex
	reqc        chan *consmsgpb.WholeMessage
	repc        chan []byte
	finishexecs map[uint64]bool
}

func initState(lg *zap.Logger,
	tp transport.Transport,
	blsSig *bls.BlsSig,
	pkPath string,
	id uint32,
	proposer *Proposer,
	n uint64, repc chan []byte,
	batchsize int) *state {
	st := &state{
		lg:          lg,
		tp:          tp,
		blsSig:      blsSig,
		pkPath:      pkPath,
		id:          id,
		proposer:    proposer,
		n:           n,
		collected:   0,
		execs:       make(map[uint64]*asyncCommSubset),
		lock:        sync.RWMutex{},
		reqc:        make(chan *consmsgpb.WholeMessage, 2*int(n)*batchsize),
		repc:        repc,
		finishexecs: make(map[uint64]bool),
	}
	go st.run()
	return st
}

func (st *state) insertMsg(msg *consmsgpb.WholeMessage) {
	st.lock.RLock()

	if _, ok := st.finishexecs[msg.ConsMsg.Sequence]; ok {
		st.lock.RUnlock()
		return
	}
	if exec, ok := st.execs[msg.ConsMsg.Sequence]; ok {
		st.lock.RUnlock()
		exec.insertMsg(msg)
	} else {
		if st.collected <= msg.ConsMsg.Sequence {
			st.lock.RUnlock()
			exec := initACS(st, st.lg, st.tp, st.blsSig, st.pkPath, st.proposer, msg.ConsMsg.Sequence, st.n, st.reqc)
			st.lock.Lock()
			if e, ok := st.execs[msg.ConsMsg.Sequence]; ok {
				exec = e
			} else {
				st.execs[msg.ConsMsg.Sequence] = exec
			}
			st.lock.Unlock()
			exec.insertMsg(msg)
		}
	}
}

func (st *state) garbageCollect(seq uint64) {
	st.lock.Lock()
	defer st.lock.Unlock()
	st.finishexecs[seq] = true
	if st.collected == seq {
		_, b := st.finishexecs[st.collected]
		for b {
			st.collected++
			_, b = st.finishexecs[st.collected]
		}
	}
}

// execute requests by a single thread
func (st *state) run() {
	for {
		req := <-st.reqc
		if req.ConsMsg.Proposer == st.id {
			st.repc <- []byte{}
		}
	}
}
