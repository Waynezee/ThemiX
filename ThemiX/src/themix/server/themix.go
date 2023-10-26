package server

import (
	"fmt"
	"sync"

	"go.themix.io/crypto/bls"
	"go.themix.io/transport"
	"go.themix.io/transport/info"
	"go.themix.io/transport/proto/consmsgpb"
	"go.uber.org/zap"
)

type asyncCommSubset struct {
	st            *state
	lg            *zap.Logger
	tp            transport.Transport
	n             uint64
	thld          uint64
	sequence      uint64
	numDecided    uint64
	numDecidedOne uint64
	instances     []*instance
	proposer      *Proposer
	reqc          chan *consmsgpb.WholeMessage
	lock          sync.Mutex
	isFinished    bool
	isCollected   bool
	proposals     []*consmsgpb.ConsMessage
	decidedValues []byte
}

func initACS(st *state,
	lg *zap.Logger,
	tp transport.Transport,
	blsSig *bls.BlsSig,
	pkPath string,
	proposer *Proposer,
	seq uint64, n uint64,
	reqc chan *consmsgpb.WholeMessage) *asyncCommSubset {
	re := &asyncCommSubset{
		st:            st,
		lg:            lg,
		tp:            tp,
		proposer:      proposer,
		n:             n,
		sequence:      seq,
		instances:     make([]*instance, n),
		reqc:          reqc,
		lock:          sync.Mutex{},
		decidedValues: make([]byte, n),
		proposals:     make([]*consmsgpb.ConsMessage, n),
	}
	re.thld = n/2 + 1
	for i := info.IDType(0); i < info.IDType(n); i++ {
		re.instances[i] = initInstance(uint32(i), proposer.id, lg, tp, blsSig, pkPath, seq, n, re.thld)
	}
	return re
}

func (acs *asyncCommSubset) insertMsg(msg *consmsgpb.WholeMessage) {
	if msg.ConsMsg.Type == consmsgpb.MessageType_ACTIVE_REQ && msg.From != acs.proposer.id {
		if acs.proposals[msg.ConsMsg.Proposer] != nil {
			m := acs.proposals[msg.ConsMsg.Proposer]
			m.Type = consmsgpb.MessageType_ACTIVE_VAL
			acs.tp.SendMessage(msg.From, &consmsgpb.WholeMessage{
				From:    acs.proposer.id,
				ConsMsg: m,
			})
		}
		return
	}
	if acs.isFinished {
		acs.lock.Lock()
		defer acs.lock.Unlock()
		if !acs.isCollected {
			acs.clearInstance()
			acs.st.garbageCollect(acs.sequence)
			acs.isCollected = true
		}
		return
	}
	isDecided, isFinished := acs.instances[msg.ConsMsg.Proposer].insertMsg(msg)

	if isDecided {
		acs.lock.Lock()
		defer acs.lock.Unlock()
		if acs.isFinished {
			return
		}
		if !acs.instances[msg.ConsMsg.Proposer].decidedOne() && msg.ConsMsg.Proposer == acs.proposer.id {
			fmt.Printf("ID %d decided zero at %d\n", msg.ConsMsg.Proposer, msg.ConsMsg.Sequence)
		}
		acs.numDecided++
		if acs.instances[msg.ConsMsg.Proposer].decidedOne() {
			acs.decidedValues[msg.ConsMsg.Proposer] = 1
		} else {
			acs.decidedValues[msg.ConsMsg.Proposer] = 0
		}
		if acs.numDecided == 1 {
			acs.proposer.proceed(acs.sequence)
		}
		if acs.instances[msg.ConsMsg.Proposer].decidedOne() {
			acs.numDecidedOne++
		}
		if acs.numDecidedOne == acs.thld {
			for i, inst := range acs.instances {
				inst.canVoteZero(uint32(i), acs.sequence)
			}
		}
		if acs.numDecided == acs.n {
			b := true
			for i := 0; i < int(acs.n); i++ {
				if acs.decidedValues[i] == 1 && acs.proposals[i] == nil {
					acs.tp.Broadcast(&consmsgpb.WholeMessage{
						ConsMsg: &consmsgpb.ConsMessage{
							Type:     consmsgpb.MessageType_ACTIVE_REQ,
							Proposer: uint32(i),
						},
					})
					b = false
				} else if acs.decidedValues[i] == 0 && acs.proposals[i] == nil && int(acs.proposer.id) == i {
					b = false
				}
			}
			if !b {
				return
			}
			for i, inst := range acs.instances {
				proposal := acs.proposals[i]
				if inst.decidedOne() && len(proposal.Content) != 0 {
					inst.lg.Info("executed",
						zap.Int("proposer", int(proposal.Proposer)),
						zap.Int("seq", int(msg.ConsMsg.Sequence)),
						zap.Int("content", int(proposal.Content[0])))
					// zap.Int("content", int(binary.LittleEndian.Uint32(proposal.Content))))
					acs.reqc <- &consmsgpb.WholeMessage{ConsMsg: proposal}
				} else if inst.id == acs.proposer.id && len(proposal.Content) != 0 {
					inst.lg.Info("repropose",
						zap.Int("proposer", int(proposal.Proposer)),
						zap.Int("seq", int(proposal.Sequence)),
						zap.Int("content", int(proposal.Content[0])))
					// zap.Int("content", int(binary.LittleEndian.Uint32(proposal.Content))))
					acs.proposer.propose(proposal.Content)
				} else if inst.decidedOne() {
					inst.lg.Info("empty",
						zap.Int("proposer", int(proposal.Proposer)),
						zap.Int("seq", int(proposal.Sequence)))
				} else {
					inst.lg.Info("decide 0 without proposal",
						zap.Int("proposer", int(inst.id)),
						zap.Int("seq", int(inst.sequence)))
				}
			}
			acs.isFinished = true
		}
	} else if isFinished {

	}

	if msg.ConsMsg.Type == consmsgpb.MessageType_VAL {
		acs.proposals[msg.ConsMsg.Proposer] = msg.ConsMsg
	}
	if (msg.ConsMsg.Type == consmsgpb.MessageType_VAL || msg.ConsMsg.Type == consmsgpb.MessageType_ACTIVE_VAL) && acs.numDecided == acs.n {
		acs.proposals[msg.ConsMsg.Proposer] = msg.ConsMsg

		acs.lock.Lock()
		defer acs.lock.Unlock()
		if acs.isFinished {
			return
		}
		for i := 0; i < int(acs.n); i++ {
			if acs.decidedValues[i] == 1 && acs.proposals[i] == nil {
				return
			} else if acs.decidedValues[i] == 0 && acs.proposals[i] == nil && int(acs.proposer.id) == i {
				return
			}
		}
		for i, inst := range acs.instances {
			proposal := acs.proposals[i]
			if inst.decidedOne() && len(proposal.Content) != 0 {
				inst.lg.Info("executed",
					zap.Int("proposer", int(proposal.Proposer)),
					zap.Int("seq", int(msg.ConsMsg.Sequence)),
					zap.Int("content", int(proposal.Content[0])))
				// zap.Int("content", int(binary.LittleEndian.Uint32(proposal.Content))))
				acs.reqc <- &consmsgpb.WholeMessage{ConsMsg: proposal}
			} else if inst.id == acs.proposer.id && len(proposal.Content) != 0 {
				inst.lg.Info("repropose",
					zap.Int("proposer", int(proposal.Proposer)),
					zap.Int("seq", int(proposal.Sequence)),
					zap.Int("content", int(proposal.Content[0])))
				// zap.Int("content", int(binary.LittleEndian.Uint32(proposal.Content))))
				acs.proposer.propose(proposal.Content)
			} else if inst.decidedOne() {
				inst.lg.Info("empty",
					zap.Int("proposer", int(proposal.Proposer)),
					zap.Int("seq", int(proposal.Sequence)))
			} else {
				inst.lg.Info("decide 0 with empty proposal",
					zap.Int("proposer", int(inst.id)),
					zap.Int("seq", int(inst.sequence)))
			}
		}
		acs.isFinished = true
		return
	}
}

func (acs *asyncCommSubset) clearInstance() {
	for i := info.IDType(0); i < info.IDType(acs.n); i++ {
		acs.instances[i] = &instance{isFinished: true}
	}
}
