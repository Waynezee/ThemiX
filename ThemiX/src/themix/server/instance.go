package server

import (
	"bytes"
	"crypto/ecdsa"
	"encoding/binary"
	"log"
	"math"
	"sync"
	"time"

	"go.themix.io/crypto/bls"
	myecdsa "go.themix.io/crypto/ecdsa"
	"go.themix.io/crypto/sha256"
	"go.themix.io/transport"
	"go.themix.io/transport/proto/consmsgpb"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
)

// the maximum expected round that terminates consensus, P = 1 - pow(0.5, maxround)
var maxround = 30

type instance struct {
	id                 uint32
	proposer           uint32
	tp                 transport.Transport
	blsSig             *bls.BlsSig
	fastRBC            bool
	hasEcho            bool
	hasEchoCollection  bool
	hasReadyCollection bool
	hasSentCoin        bool
	fastAuxZero        bool
	fastAuxOne         bool
	isDecided          bool
	isFinished         bool
	sequence           uint64
	n                  uint64
	thld               uint64
	f                  uint64
	fastgroup          uint64
	round              uint32
	numEcho            uint64
	numReady           uint64
	binVals            uint8
	lastCoin           uint8
	hash               []byte
	sin                []bool
	hasSentAux         []bool
	hasVotedZero       []bool
	hasVotedOne        []bool
	zeroEndorsed       []bool
	oneEndorsed        []bool
	canSkipCoin        []bool
	numBvalZero        []uint64
	numBvalOne         []uint64
	numAuxZero         []uint64
	numAuxOne          []uint64
	numOneSkip         []uint64
	numZeroSkip        []uint64
	numCoin            []uint64
	echoSigns          *consmsgpb.Collections
	readySigns         *consmsgpb.Collections
	proposal           *consmsgpb.WholeMessage
	valMsgs            [][]byte
	singleAuxZero      []bool
	singleAuxOne       []bool
	promiseZero        []bool
	promiseOne         []bool
	bvalZeroSigns      []*consmsgpb.Collections
	bvalOneSigns       []*consmsgpb.Collections
	auxZeroSigns       []*consmsgpb.Collections
	auxOneSigns        []*consmsgpb.Collections
	coinMsgs           [][]*consmsgpb.WholeMessage
	startR             bool
	expireR            bool
	startB             []bool
	startS             []bool
	expireB            []bool
	delta              int64
	deltaBar           int64
	priv               *ecdsa.PrivateKey
	lg                 *zap.Logger
	lock               sync.Mutex
}

func initInstance(id uint32, proposer uint32, lg *zap.Logger, tp transport.Transport, blsSig *bls.BlsSig, pkPath string, sequence uint64, n uint64, thld uint64) *instance {
	inst := &instance{
		id:            id,
		proposer:      proposer,
		lg:            lg,
		tp:            tp,
		blsSig:        blsSig,
		sequence:      sequence,
		n:             n,
		thld:          thld,
		f:             n / 2,
		delta:         500,
		deltaBar:      2500,
		echoSigns:     &consmsgpb.Collections{Collections: make([][]byte, n)},
		readySigns:    &consmsgpb.Collections{Collections: make([][]byte, n)},
		hasSentAux:    make([]bool, maxround),
		hasVotedZero:  make([]bool, maxround),
		hasVotedOne:   make([]bool, maxround),
		zeroEndorsed:  make([]bool, maxround),
		oneEndorsed:   make([]bool, maxround),
		sin:           make([]bool, maxround),
		canSkipCoin:   make([]bool, maxround),
		startB:        make([]bool, maxround),
		startS:        make([]bool, maxround),
		expireB:       make([]bool, maxround),
		singleAuxZero: make([]bool, maxround),
		singleAuxOne:  make([]bool, maxround),
		promiseZero:   make([]bool, maxround),
		promiseOne:    make([]bool, maxround),
		valMsgs:       make([][]byte, n),
		bvalZeroSigns: make([]*consmsgpb.Collections, maxround),
		bvalOneSigns:  make([]*consmsgpb.Collections, maxround),
		auxZeroSigns:  make([]*consmsgpb.Collections, maxround),
		auxOneSigns:   make([]*consmsgpb.Collections, maxround),
		coinMsgs:      make([][]*consmsgpb.WholeMessage, maxround),
		numBvalZero:   make([]uint64, maxround),
		numBvalOne:    make([]uint64, maxround),
		numAuxZero:    make([]uint64, maxround),
		numAuxOne:     make([]uint64, maxround),
		numCoin:       make([]uint64, maxround),
		numZeroSkip:   make([]uint64, maxround),
		numOneSkip:    make([]uint64, maxround),
		lock:          sync.Mutex{}}
	inst.fastgroup = uint64(math.Ceil(3*float64(inst.f)/2)) + 1
	inst.priv, _ = myecdsa.LoadKey(pkPath)
	for i := 0; i < maxround; i++ {
		inst.singleAuxZero[i] = true
		inst.singleAuxOne[i] = true
		inst.bvalZeroSigns[i] = &consmsgpb.Collections{Collections: make([][]byte, n)}
		inst.bvalOneSigns[i] = &consmsgpb.Collections{Collections: make([][]byte, n)}
		inst.auxZeroSigns[i] = &consmsgpb.Collections{Collections: make([][]byte, n)}
		inst.auxOneSigns[i] = &consmsgpb.Collections{Collections: make([][]byte, n)}
		inst.coinMsgs[i] = make([]*consmsgpb.WholeMessage, n)
		inst.canSkipCoin[i] = true
	}
	return inst
}

// return true if the instance is decided or finished at the first time
func (inst *instance) insertMsg(msg *consmsgpb.WholeMessage) (bool, bool) {
	inst.lock.Lock()
	defer inst.lock.Unlock()

	if inst.isFinished {
		return false, false
	}

	// if len(msg.ConsMsg.Content) > 0 {
	// 	inst.lg.Info("receive msg",
	// 		zap.String("type", consmsgpb.MessageType_name[int32(msg.ConsMsg.Type)]),
	// 		zap.Int("proposer", int(msg.ConsMsg.Proposer)),
	// 		zap.Int("seq", int(msg.ConsMsg.Sequence)),
	// 		zap.Int("round", int(msg.ConsMsg.Round)),
	// 		zap.Int("from", int(msg.From)),
	// 		zap.Int("content", int(msg.ConsMsg.Content[0])))
	// } else {
	// 	inst.lg.Info("receive msg",
	// 		zap.String("type", consmsgpb.MessageType_name[int32(msg.ConsMsg.Type)]),
	// 		zap.Int("proposer", int(msg.ConsMsg.Proposer)),
	// 		zap.Int("seq", int(msg.ConsMsg.Sequence)),
	// 		zap.Int("round", int(msg.ConsMsg.Round)),
	// 		zap.Int("from", int(msg.From)))
	// }

	switch msg.ConsMsg.Type {
	case consmsgpb.MessageType_VAL:
		inst.proposal = msg
		hash, _ := sha256.ComputeHash(msg.ConsMsg.Content)
		inst.hash = hash
		if !inst.hasEcho {
			inst.hasEcho = true
			m := &consmsgpb.WholeMessage{
				ConsMsg: &consmsgpb.ConsMessage{
					Type:     consmsgpb.MessageType_ECHO,
					Proposer: msg.ConsMsg.Proposer,
					Sequence: msg.ConsMsg.Sequence,
					Content:  hash,
				}}
			inst.tp.Broadcast(m)
		}
		if inst.round == 0 && ((inst.numReady >= inst.thld && inst.proposal != nil && !inst.hasVotedOne[inst.round]) ||
			(inst.numEcho >= inst.fastgroup && inst.proposal != nil && !inst.hasVotedOne[inst.round])) &&
			!inst.promiseZero[inst.round] && !inst.hasSentAux[inst.round] {
			inst.hasVotedOne[inst.round] = true
			inst.fastRBC = true
			m := &consmsgpb.WholeMessage{
				ConsMsg: &consmsgpb.ConsMessage{
					Type:     consmsgpb.MessageType_BVAL,
					Proposer: msg.ConsMsg.Proposer,
					Sequence: msg.ConsMsg.Sequence,
					Content:  []byte{1}, // vote 1
				},
			}
			inst.tp.Broadcast(m)
		}
		inst.isReadyToSendCoin()
		return inst.isReadyToEnterNewRound()
	case consmsgpb.MessageType_ECHO:
		if inst.echoSigns.Collections[msg.From] != nil || inst.round != 0 {
			return false, false
		}
		for _, digest := range inst.valMsgs {
			if digest != nil && !bytes.Equal(digest, msg.ConsMsg.Content) {
				return false, false
			}
		}
		inst.valMsgs[msg.From] = msg.ConsMsg.Content
		if !inst.hasEcho {
			inst.hasEcho = true
			m := &consmsgpb.WholeMessage{
				ConsMsg: &consmsgpb.ConsMessage{
					Type:     consmsgpb.MessageType_ECHO,
					Proposer: msg.ConsMsg.Proposer,
					Sequence: msg.ConsMsg.Sequence,
					Content:  msg.ConsMsg.Content,
				},
			}
			inst.tp.Broadcast(m)
		}
		inst.numEcho++
		inst.echoSigns.Collections[msg.From] = msg.Signature
		if inst.numEcho >= inst.f+1 && !inst.startR {
			inst.startR = true
			go func() {
				time.Sleep(time.Duration(inst.deltaBar) * time.Millisecond)
				inst.expireR = true
				if inst.numEcho >= inst.f+1 {
					inst.tp.Broadcast(&consmsgpb.WholeMessage{
						ConsMsg: &consmsgpb.ConsMessage{
							Type:     consmsgpb.MessageType_READY,
							Proposer: msg.ConsMsg.Proposer,
							Sequence: msg.ConsMsg.Sequence,
							Content:  msg.ConsMsg.Content,
						},
					})
				}
			}()
		}
		if inst.numEcho >= inst.fastgroup && !inst.fastRBC && inst.round == 0 {
			if !inst.hasEchoCollection {
				inst.hasEchoCollection = true
				collection := serialCollection(inst.echoSigns)
				m := &consmsgpb.WholeMessage{
					ConsMsg: &consmsgpb.ConsMessage{
						Type:     consmsgpb.MessageType_ECHO_COLLECTION,
						Proposer: msg.ConsMsg.Proposer,
						Round:    msg.ConsMsg.Round,
						Sequence: msg.ConsMsg.Sequence,
					},
					Collection: collection,
				}
				inst.tp.Broadcast(m)
			}
			if inst.round == 0 && !inst.hasVotedOne[inst.round] && inst.proposal != nil &&
				!inst.promiseZero[inst.round] && !inst.hasSentAux[inst.round] {
				inst.hasVotedOne[inst.round] = true
				inst.fastRBC = true
				m := &consmsgpb.WholeMessage{
					ConsMsg: &consmsgpb.ConsMessage{
						Type:     consmsgpb.MessageType_BVAL,
						Proposer: msg.ConsMsg.Proposer,
						Sequence: msg.ConsMsg.Sequence,
						Content:  []byte{1}, // vote 1
					},
				}
				inst.tp.Broadcast(m)
			}
		}
		return inst.isReadyToEnterNewRound()
	case consmsgpb.MessageType_READY:
		if inst.readySigns.Collections[msg.From] != nil || inst.round != 0 {
			return false, false
		}
		inst.numReady++
		inst.readySigns.Collections[msg.From] = msg.Signature
		if inst.numReady >= inst.f+1 && inst.round == 0 && !inst.fastRBC {
			if !inst.hasReadyCollection {
				inst.hasReadyCollection = true
				collection := serialCollection(inst.readySigns)
				m := &consmsgpb.WholeMessage{
					ConsMsg: &consmsgpb.ConsMessage{
						Type:     consmsgpb.MessageType_READY_COLLECTION,
						Proposer: msg.ConsMsg.Proposer,
						Round:    msg.ConsMsg.Round,
						Sequence: msg.ConsMsg.Sequence,
					},
					Collection: collection,
				}
				inst.tp.Broadcast(m)
			}
			if inst.round == 0 && !inst.hasVotedOne[inst.round] && inst.proposal != nil &&
				!inst.promiseZero[inst.round] && !inst.hasSentAux[inst.round] {
				inst.fastRBC = true
				inst.hasVotedOne[inst.round] = true
				m := &consmsgpb.WholeMessage{
					ConsMsg: &consmsgpb.ConsMessage{
						Type:     consmsgpb.MessageType_BVAL,
						Proposer: msg.ConsMsg.Proposer,
						Sequence: msg.ConsMsg.Sequence,
						Content:  []byte{1}, // vote 1
					},
				}
				inst.tp.Broadcast(m)
			}
		}
		return inst.isReadyToEnterNewRound()
	case consmsgpb.MessageType_BVAL:
		var b bool
		switch msg.ConsMsg.Content[0] {
		case 0:
			inst.numBvalZero[msg.ConsMsg.Round]++
			inst.bvalZeroSigns[msg.ConsMsg.Round].Collections[msg.From] = msg.Signature
		case 1:
			inst.numBvalOne[msg.ConsMsg.Round]++
			inst.bvalOneSigns[msg.ConsMsg.Round].Collections[msg.From] = msg.Signature
		}
		if msg.ConsMsg.Content[0] == 0 && !inst.zeroEndorsed[msg.ConsMsg.Round] && inst.numBvalZero[msg.ConsMsg.Round] >= inst.thld {
			collection := serialCollection(inst.bvalZeroSigns[msg.ConsMsg.Round])
			m := &consmsgpb.WholeMessage{
				ConsMsg: &consmsgpb.ConsMessage{
					Type:     consmsgpb.MessageType_BVAL_ZERO_COLLECTION,
					Proposer: msg.ConsMsg.Proposer,
					Round:    msg.ConsMsg.Round,
					Sequence: msg.ConsMsg.Sequence,
				},
				Collection: collection,
			}
			inst.tp.Broadcast(m)
			inst.zeroEndorsed[msg.ConsMsg.Round] = true
			if !inst.hasSentAux[msg.ConsMsg.Round] {
				inst.hasSentAux[msg.ConsMsg.Round] = true
				if !inst.hasVotedOne[msg.ConsMsg.Round] {
					inst.promiseZero[msg.ConsMsg.Round] = true
				}
				m := &consmsgpb.WholeMessage{
					ConsMsg: &consmsgpb.ConsMessage{
						Type:     consmsgpb.MessageType_AUX,
						Proposer: msg.ConsMsg.Proposer,
						Round:    msg.ConsMsg.Round,
						Sequence: msg.ConsMsg.Sequence,
						Content:  []byte{0}, // aux 0
						Single:   inst.promiseZero[msg.ConsMsg.Round],
					},
				}
				inst.tp.Broadcast(m)
			}
			inst.isReadyToSendCoin()
			b = true
		}
		if msg.ConsMsg.Content[0] == 1 && !inst.oneEndorsed[msg.ConsMsg.Round] && inst.numBvalOne[msg.ConsMsg.Round] >= inst.thld {
			collection := serialCollection(inst.bvalOneSigns[msg.ConsMsg.Round])
			m := &consmsgpb.WholeMessage{
				ConsMsg: &consmsgpb.ConsMessage{
					Type:     consmsgpb.MessageType_BVAL_ONE_COLLECTION,
					Proposer: msg.ConsMsg.Proposer,
					Round:    msg.ConsMsg.Round,
					Sequence: msg.ConsMsg.Sequence,
				},
				Collection: collection,
			}
			inst.tp.Broadcast(m)
			inst.oneEndorsed[msg.ConsMsg.Round] = true
			if !inst.hasSentAux[msg.ConsMsg.Round] {
				inst.hasSentAux[msg.ConsMsg.Round] = true
				if !inst.hasVotedZero[msg.ConsMsg.Round] {
					inst.promiseOne[msg.ConsMsg.Round] = true
				}
				m := &consmsgpb.WholeMessage{
					ConsMsg: &consmsgpb.ConsMessage{
						Type:     consmsgpb.MessageType_AUX,
						Proposer: msg.ConsMsg.Proposer,
						Round:    msg.ConsMsg.Round,
						Sequence: msg.ConsMsg.Sequence,
						Content:  []byte{1}, // aux 1
						Single:   inst.promiseOne[msg.ConsMsg.Round],
					},
				}
				inst.tp.Broadcast(m)
			}
			inst.isReadyToSendCoin()
			b = true
		}
		if b {
			if !inst.startS[msg.ConsMsg.Round] {
				inst.startS[msg.ConsMsg.Round] = true
				go func() {
					time.Sleep(time.Duration(inst.delta) * time.Millisecond)
					inst.canSkipCoin[msg.ConsMsg.Round] = false
				}()
			}
			return inst.isReadyToEnterNewRound()
		}
		if inst.round == msg.ConsMsg.Round+1 {
			switch msg.ConsMsg.Content[0] {
			case 0:
				if inst.numBvalZero[msg.ConsMsg.Round] >= inst.f+1 && inst.lastCoin == 0 &&
					(!inst.hasVotedZero[inst.round] || !inst.hasSentAux[inst.round]) &&
					!inst.sin[msg.ConsMsg.Round] &&
					!inst.promiseOne[inst.round] {
					inst.hasVotedZero[inst.round] = true
					inst.tp.Broadcast(&consmsgpb.WholeMessage{
						ConsMsg: &consmsgpb.ConsMessage{
							Type:     consmsgpb.MessageType_BVAL,
							Proposer: msg.ConsMsg.Proposer,
							Round:    inst.round,
							Sequence: msg.ConsMsg.Sequence,
							Content:  []byte{0},
						},
					})
				}
			case 1:
				if inst.numBvalOne[msg.ConsMsg.Round] >= inst.f+1 && inst.lastCoin == 1 &&
					(!inst.hasVotedOne[inst.round] || !inst.hasSentAux[inst.round]) &&
					!inst.sin[msg.ConsMsg.Round] &&
					!inst.promiseZero[inst.round] {
					inst.hasVotedOne[inst.round] = true
					inst.tp.Broadcast(&consmsgpb.WholeMessage{
						ConsMsg: &consmsgpb.ConsMessage{
							Type:     consmsgpb.MessageType_BVAL,
							Proposer: msg.ConsMsg.Proposer,
							Round:    inst.round,
							Sequence: msg.ConsMsg.Sequence,
							Content:  []byte{1},
						},
					})
				}
			}
		}
	case consmsgpb.MessageType_AUX:
		switch msg.ConsMsg.Content[0] {
		case 0:
			inst.numAuxZero[msg.ConsMsg.Round]++
			inst.auxZeroSigns[msg.ConsMsg.Round].Collections[msg.From] = msg.Signature
			inst.singleAuxOne[msg.ConsMsg.Round] = false
			if !msg.ConsMsg.Single {
				inst.singleAuxZero[msg.ConsMsg.Round] = false
			}
		case 1:
			inst.numAuxOne[msg.ConsMsg.Round]++
			inst.auxOneSigns[msg.ConsMsg.Round].Collections[msg.From] = msg.Signature
			inst.singleAuxZero[msg.ConsMsg.Round] = false
			if !msg.ConsMsg.Single {
				inst.singleAuxOne[msg.ConsMsg.Round] = false
			}
		}
		if inst.round == msg.ConsMsg.Round && inst.numAuxZero[inst.round]+inst.numAuxOne[inst.round] >= inst.thld && !inst.startB[inst.round] {
			inst.startB[inst.round] = true
			go func() {
				time.Sleep((time.Duration(inst.delta + inst.deltaBar)) * time.Millisecond)
				inst.expireB[inst.round] = true
				if inst.numAuxZero[inst.round]+inst.numAuxOne[inst.round] > inst.f &&
					((inst.numAuxZero[inst.round] != 0 && inst.numBvalZero[inst.round] > inst.f) ||
						(inst.numAuxOne[inst.round] != 0 && inst.numBvalOne[inst.round] > inst.f)) {
					inst.lock.Lock()
					inst.isReadyToSendCoin()
					inst.lock.Unlock()
				}
			}()
		} else if inst.round == msg.ConsMsg.Round && inst.numAuxOne[inst.round]+inst.numAuxZero[inst.round] >= inst.thld && inst.expireB[inst.round] {
			inst.isReadyToSendCoin()
		}
		if inst.round == msg.ConsMsg.Round && msg.ConsMsg.Content[0] == 0 &&
			!inst.fastAuxZero && inst.numAuxZero[msg.ConsMsg.Round] >= inst.fastgroup {
			inst.fastAuxZero = true
			collection := serialCollection(inst.auxZeroSigns[msg.ConsMsg.Round])
			inst.tp.Broadcast(&consmsgpb.WholeMessage{
				ConsMsg: &consmsgpb.ConsMessage{
					Type:     consmsgpb.MessageType_AUX_ZERO_COLLECTION,
					Proposer: msg.ConsMsg.Proposer,
					Round:    inst.round,
					Sequence: msg.ConsMsg.Sequence,
				},
				Collection: collection,
			})
			inst.zeroEndorsed[inst.round] = true
			inst.sin[inst.round] = true
			if inst.canSkipCoin[inst.round] {
				inst.tp.Broadcast(&consmsgpb.WholeMessage{
					ConsMsg: &consmsgpb.ConsMessage{
						Type:     consmsgpb.MessageType_SKIP,
						Proposer: msg.ConsMsg.Proposer,
						Round:    inst.round,
						Sequence: msg.ConsMsg.Sequence,
						Content:  msg.ConsMsg.Content,
					},
				})
			}
			inst.isReadyToSendCoin()
			return inst.isReadyToEnterNewRound()
		}
		if inst.round == msg.ConsMsg.Round && msg.ConsMsg.Content[0] == 1 &&
			!inst.fastAuxOne && inst.numAuxOne[msg.ConsMsg.Round] >= inst.fastgroup {
			inst.fastAuxOne = true
			collection := serialCollection(inst.auxOneSigns[msg.ConsMsg.Round])
			inst.tp.Broadcast(&consmsgpb.WholeMessage{
				ConsMsg: &consmsgpb.ConsMessage{
					Type:     consmsgpb.MessageType_AUX_ONE_COLLECTION,
					Proposer: msg.ConsMsg.Proposer,
					Round:    inst.round,
					Sequence: msg.ConsMsg.Sequence,
				},
				Collection: collection,
			})
			inst.oneEndorsed[inst.round] = true
			inst.sin[inst.round] = true
			if inst.canSkipCoin[inst.round] {
				inst.tp.Broadcast(&consmsgpb.WholeMessage{
					ConsMsg: &consmsgpb.ConsMessage{
						Type:     consmsgpb.MessageType_SKIP,
						Proposer: msg.ConsMsg.Proposer,
						Round:    inst.round,
						Sequence: msg.ConsMsg.Sequence,
						Content:  msg.ConsMsg.Content,
					},
				})
			}
			inst.isReadyToSendCoin()
			return inst.isReadyToEnterNewRound()
		}
		if inst.round == msg.ConsMsg.Round+1 {
			switch msg.ConsMsg.Content[0] {
			case 0:
				if (!inst.hasVotedZero[inst.round] || !inst.hasSentAux[inst.round]) &&
					inst.numAuxZero[msg.ConsMsg.Round] >= inst.fastgroup &&
					!inst.promiseOne[inst.round] {
					inst.hasVotedZero[inst.round] = true
					inst.tp.Broadcast(&consmsgpb.WholeMessage{
						ConsMsg: &consmsgpb.ConsMessage{
							Type:     consmsgpb.MessageType_BVAL,
							Proposer: msg.ConsMsg.Proposer,
							Round:    inst.round,
							Sequence: msg.ConsMsg.Sequence,
							Content:  []byte{0},
						},
					})
				}
			case 1:
				if (!inst.hasVotedOne[inst.round] || !inst.hasSentAux[inst.round]) &&
					inst.numAuxOne[msg.ConsMsg.Round] >= inst.fastgroup &&
					!inst.promiseZero[inst.round] {
					inst.hasVotedOne[inst.round] = true
					inst.tp.Broadcast(&consmsgpb.WholeMessage{
						ConsMsg: &consmsgpb.ConsMessage{
							Type:     consmsgpb.MessageType_BVAL,
							Proposer: msg.ConsMsg.Proposer,
							Round:    inst.round,
							Sequence: msg.ConsMsg.Sequence,
							Content:  []byte{1},
						},
					})
				}
			}
		}
		if inst.round == msg.ConsMsg.Round {
			inst.isReadyToSendCoin()
			return inst.isReadyToEnterNewRound()
		}
	case consmsgpb.MessageType_SKIP:
		switch msg.ConsMsg.Content[0] {
		case 0:
			inst.numZeroSkip[msg.ConsMsg.Round]++
		case 1:
			inst.numOneSkip[msg.ConsMsg.Round]++
		}
		if inst.round == msg.ConsMsg.Round {
			return inst.isReadyToEnterNewRound()
		}
	case consmsgpb.MessageType_COIN:
		inst.coinMsgs[msg.ConsMsg.Round][msg.From] = msg
		inst.numCoin[msg.ConsMsg.Round]++
		if inst.round == msg.ConsMsg.Round && inst.numCoin[inst.round] >= inst.f+1 {
			return inst.isReadyToEnterNewRound()
		}
	case consmsgpb.MessageType_ECHO_COLLECTION:
		if inst.fastRBC || inst.hasVotedOne[inst.round] || inst.hash == nil || inst.round != 0 {
			return false, false
		}
		if !inst.verifyECHOCollection(msg) {
			return false, false
		}
		inst.fastRBC = true
		inst.tp.Broadcast(msg)
		if !inst.promiseZero[inst.round] && !inst.hasSentAux[inst.round] && inst.proposal != nil {
			inst.hasVotedOne[inst.round] = true
			m := &consmsgpb.WholeMessage{
				ConsMsg: &consmsgpb.ConsMessage{
					Type:     consmsgpb.MessageType_BVAL,
					Proposer: msg.ConsMsg.Proposer,
					Sequence: msg.ConsMsg.Sequence,
					Content:  []byte{1}, // vote 1
				},
			}
			inst.tp.Broadcast(m)
		}
		return inst.isReadyToEnterNewRound()
	case consmsgpb.MessageType_READY_COLLECTION:
		if inst.fastRBC || inst.hasVotedOne[inst.round] || inst.hash == nil || inst.round != 0 {
			return false, false
		}
		if !inst.verifyREADYCollection(msg) {
			return false, false
		}
		inst.fastRBC = true
		inst.tp.Broadcast(msg)
		if !inst.promiseZero[inst.round] && !inst.hasSentAux[inst.round] && inst.proposal != nil {
			inst.hasVotedOne[inst.round] = true
			m := &consmsgpb.WholeMessage{
				ConsMsg: &consmsgpb.ConsMessage{
					Type:     consmsgpb.MessageType_BVAL,
					Proposer: msg.ConsMsg.Proposer,
					Sequence: msg.ConsMsg.Sequence,
					Content:  []byte{1}, // vote 1
				},
			}
			inst.tp.Broadcast(m)
		}
		return inst.isReadyToEnterNewRound()
	case consmsgpb.MessageType_BVAL_ZERO_COLLECTION:
		if inst.zeroEndorsed[msg.ConsMsg.Round] || inst.oneEndorsed[msg.ConsMsg.Round] || inst.hasSentAux[msg.ConsMsg.Round] {
			return false, false
		}
		if !inst.VerifyCollection(msg) {
			return false, false
		}
		inst.tp.Broadcast(msg)
		inst.zeroEndorsed[msg.ConsMsg.Round] = true
		if !inst.hasSentAux[msg.ConsMsg.Round] {
			inst.hasSentAux[msg.ConsMsg.Round] = true
			if !inst.hasVotedOne[msg.ConsMsg.Round] {
				inst.promiseZero[msg.ConsMsg.Round] = true
			}
			m := &consmsgpb.WholeMessage{
				ConsMsg: &consmsgpb.ConsMessage{
					Type:     consmsgpb.MessageType_AUX,
					Proposer: msg.ConsMsg.Proposer,
					Round:    msg.ConsMsg.Round,
					Sequence: msg.ConsMsg.Sequence,
					Content:  []byte{0}, // aux 0
					Single:   inst.promiseZero[msg.ConsMsg.Round],
				},
			}
			inst.tp.Broadcast(m)
			if !inst.startS[msg.ConsMsg.Round] {
				inst.startS[msg.ConsMsg.Round] = true
				go func() {
					time.Sleep(time.Duration(inst.delta) * time.Millisecond)
					inst.canSkipCoin[msg.ConsMsg.Round] = false
				}()
			}
		}
		inst.isReadyToSendCoin()
		return inst.isReadyToEnterNewRound()
	case consmsgpb.MessageType_BVAL_ONE_COLLECTION:
		if inst.oneEndorsed[msg.ConsMsg.Round] || inst.zeroEndorsed[msg.ConsMsg.Round] || inst.hasSentAux[msg.ConsMsg.Round] {
			return false, false
		}
		if !inst.VerifyCollection(msg) {
			return false, false
		}
		inst.tp.Broadcast(msg)
		inst.oneEndorsed[msg.ConsMsg.Round] = true
		if !inst.hasSentAux[msg.ConsMsg.Round] {
			inst.hasSentAux[msg.ConsMsg.Round] = true
			if !inst.hasVotedZero[msg.ConsMsg.Round] {
				inst.promiseOne[msg.ConsMsg.Round] = true
			}
			m := &consmsgpb.WholeMessage{
				ConsMsg: &consmsgpb.ConsMessage{
					Type:     consmsgpb.MessageType_AUX,
					Proposer: msg.ConsMsg.Proposer,
					Round:    msg.ConsMsg.Round,
					Sequence: msg.ConsMsg.Sequence,
					Content:  []byte{1}, // aux 1
					Single:   inst.promiseOne[msg.ConsMsg.Round],
				},
			}
			inst.tp.Broadcast(m)
			if !inst.startS[msg.ConsMsg.Round] {
				inst.startS[msg.ConsMsg.Round] = true
				go func() {
					time.Sleep(time.Duration(inst.delta) * time.Millisecond)
					inst.canSkipCoin[msg.ConsMsg.Round] = false
				}()
			}
		}
		inst.isReadyToSendCoin()
		return inst.isReadyToEnterNewRound()
	case consmsgpb.MessageType_AUX_ZERO_COLLECTION:
		if inst.fastAuxZero || inst.fastAuxOne || inst.round != msg.ConsMsg.Round {
			return false, false
		}
		if !inst.VerifyCollection(msg) {
			return false, false
		}
		inst.fastAuxZero = true
		inst.zeroEndorsed[inst.round] = true
		if inst.canSkipCoin[inst.round] {
			inst.tp.Broadcast(&consmsgpb.WholeMessage{
				ConsMsg: &consmsgpb.ConsMessage{
					Type:     consmsgpb.MessageType_SKIP,
					Proposer: msg.ConsMsg.Proposer,
					Round:    inst.round,
					Sequence: msg.ConsMsg.Sequence,
					Content:  []byte{0},
				},
			})
		}
		inst.isReadyToSendCoin()
		return inst.isReadyToEnterNewRound()
	case consmsgpb.MessageType_AUX_ONE_COLLECTION:
		if inst.fastAuxZero || inst.fastAuxOne || inst.round != msg.ConsMsg.Round {
			return false, false
		}
		if !inst.VerifyCollection(msg) {
			return false, false
		}
		inst.fastAuxOne = true
		inst.oneEndorsed[inst.round] = true
		if inst.canSkipCoin[inst.round] {
			inst.tp.Broadcast(&consmsgpb.WholeMessage{
				ConsMsg: &consmsgpb.ConsMessage{
					Type:     consmsgpb.MessageType_SKIP,
					Proposer: msg.ConsMsg.Proposer,
					Round:    inst.round,
					Sequence: msg.ConsMsg.Sequence,
					Content:  []byte{1},
				},
			})
		}
		inst.isReadyToSendCoin()
		return inst.isReadyToEnterNewRound()
	default:
		return false, false
	}
	return false, false
}

func (inst *instance) isReadyToEnterNewRound() (bool, bool) {
	if !inst.isDecided &&
		(inst.numOneSkip[inst.round] >= inst.fastgroup || inst.numZeroSkip[inst.round] >= inst.fastgroup) {
		if inst.numOneSkip[inst.round] >= inst.fastgroup {
			inst.binVals = 1
			inst.isDecided = true
		} else if inst.numZeroSkip[inst.round] >= inst.fastgroup {
			inst.binVals = 0
			inst.isDecided = true
		}
		inst.lg.Info("fast decide",
			zap.Int("nextVote", int(inst.binVals)),
			zap.Int("proposer", int(inst.id)),
			zap.Int("round", int(inst.round)),
		)
		nextVote := inst.binVals
		if nextVote == 0 {
			inst.hasVotedZero[inst.round+1] = true
			inst.hasVotedOne[inst.round+1] = false
		} else {
			inst.hasVotedZero[inst.round+1] = false
			inst.hasVotedOne[inst.round+1] = true
		}
		inst.hasSentCoin = false
		inst.fastAuxZero = false
		inst.fastAuxOne = false
		inst.round++
		inst.tp.Broadcast(&consmsgpb.WholeMessage{
			ConsMsg: &consmsgpb.ConsMessage{
				Type:     consmsgpb.MessageType_BVAL,
				Proposer: inst.id,
				Round:    inst.round,
				Sequence: inst.sequence,
				Content:  []byte{nextVote},
			},
		})
		return true, false
	}
	if inst.hasSentCoin &&
		inst.numCoin[inst.round] > inst.f &&
		inst.numAuxZero[inst.round]+inst.numAuxOne[inst.round] >= inst.thld &&
		((inst.oneEndorsed[inst.round] && inst.numAuxOne[inst.round] >= inst.thld) ||
			(inst.zeroEndorsed[inst.round] && inst.numAuxZero[inst.round] >= inst.thld) ||
			(inst.oneEndorsed[inst.round] && inst.zeroEndorsed[inst.round])) {
		singleCONOne := 0
		singleCONZero := 0
		sigShares := make([][]byte, 0)
		for _, m := range inst.coinMsgs[inst.round] {
			if m != nil {
				sigShares = append(sigShares, m.ConsMsg.Content)
			}
			if m != nil && m.ConsMsg.Single && m.ConsMsg.BinVals[0] == 1 {
				singleCONOne++
			} else if m != nil && m.ConsMsg.Single && m.ConsMsg.BinVals[0] == 0 {
				singleCONZero++
			}
		}
		if !inst.isDecided && ((singleCONOne >= int(inst.thld) && inst.numBvalOne[inst.round] >= inst.thld) ||
			(singleCONZero >= int(inst.thld) && inst.numBvalZero[inst.round] >= inst.thld)) {
			inst.isDecided = true
			if singleCONOne >= int(inst.thld) {
				inst.binVals = 1
			} else {
				inst.binVals = 0
			}
			nextVote := inst.binVals
			if nextVote == 0 {
				inst.hasVotedZero[inst.round+1] = true
				inst.hasVotedOne[inst.round+1] = false
			} else {
				inst.hasVotedZero[inst.round+1] = false
				inst.hasVotedOne[inst.round+1] = true
			}
			inst.hasSentCoin = false
			inst.fastAuxZero = false
			inst.fastAuxOne = false
			inst.round++
			inst.tp.Broadcast(&consmsgpb.WholeMessage{
				ConsMsg: &consmsgpb.ConsMessage{
					Type:     consmsgpb.MessageType_BVAL,
					Proposer: inst.id,
					Round:    inst.round,
					Sequence: inst.sequence,
					Content:  []byte{nextVote},
				},
			})
			return true, false
		}
		if len(sigShares) < int(inst.thld) {
			return false, false
		}
		coin := inst.blsSig.Recover(inst.getCoinInfo(), sigShares, int(inst.f+1), int(inst.n))
		var nextVote byte
		if coin[0]%2 == inst.binVals {
			if inst.isDecided {
				inst.isFinished = true
				return false, true
			}
			inst.lg.Info("coin result",
				zap.Int("proposer", int(inst.id)),
				zap.Int("seq", int(inst.sequence)),
				zap.Int("round", int(inst.round)),
				zap.Int("result", int(coin[0]%2)))
			inst.isDecided = true
			nextVote = inst.binVals
		} else if inst.binVals != 2 { // nextVote should insist the single value
			nextVote = inst.binVals
		} else {
			nextVote = coin[0] % 2
		}
		if nextVote == 0 {
			inst.hasVotedZero[inst.round+1] = true
			inst.hasVotedOne[inst.round+1] = false
		} else {
			inst.hasVotedZero[inst.round+1] = false
			inst.hasVotedOne[inst.round+1] = true
		}
		inst.hasSentCoin = false
		inst.fastAuxZero = false
		inst.fastAuxOne = false
		inst.round++
		inst.tp.Broadcast(&consmsgpb.WholeMessage{
			ConsMsg: &consmsgpb.ConsMessage{
				Type:     consmsgpb.MessageType_BVAL,
				Proposer: inst.id,
				Round:    inst.round,
				Sequence: inst.sequence,
				Content:  []byte{nextVote},
			},
		})
		if coin[0]%2 == inst.binVals && inst.isDecided {
			return true, false
		}
	}
	return false, false
}

// must be executed within inst.lock
func (inst *instance) isReadyToSendCoin() {
	if !inst.hasSentCoin &&
		(inst.expireB[inst.round] ||
			((inst.numOneSkip[inst.round] >= inst.fastgroup || inst.numZeroSkip[inst.round] >= inst.fastgroup) && inst.expireR)) {
		if !inst.isDecided {
			if inst.oneEndorsed[inst.round] && inst.numAuxOne[inst.round] >= inst.thld {
				inst.binVals = 1
			} else if inst.zeroEndorsed[inst.round] && inst.numAuxZero[inst.round] >= inst.thld {
				inst.binVals = 0
			} else if inst.oneEndorsed[inst.round] && inst.zeroEndorsed[inst.round] &&
				inst.numAuxOne[inst.round]+inst.numAuxZero[inst.round] >= inst.thld {
				inst.binVals = 2
			} else {
				return
			}
		}

		single := inst.singleAuxZero[inst.round] || inst.singleAuxOne[inst.round]
		inst.hasSentCoin = true
		inst.tp.Broadcast(&consmsgpb.WholeMessage{
			ConsMsg: &consmsgpb.ConsMessage{
				Type:     consmsgpb.MessageType_COIN,
				Proposer: inst.id,
				Round:    inst.round,
				Sequence: inst.sequence,
				Content:  inst.blsSig.Sign(inst.getCoinInfo()),
				Single:   single,
				BinVals:  []byte{inst.binVals},
			},
		}) // threshold bls sig share
	}
}

func (inst *instance) decidedOne() bool {
	inst.lock.Lock()
	defer inst.lock.Unlock()

	return inst.isDecided && inst.binVals == 1
}

func (inst *instance) getProposal() *consmsgpb.WholeMessage {
	inst.lock.Lock()
	defer inst.lock.Unlock()

	return inst.proposal
}

func (inst *instance) getCoinInfo() []byte {
	bsender := make([]byte, 8)
	binary.LittleEndian.PutUint64(bsender, uint64(inst.id))
	bseq := make([]byte, 8)
	binary.LittleEndian.PutUint64(bseq, inst.sequence)

	b := make([]byte, 17)
	b = append(b, bsender...)
	b = append(b, bseq...)
	b = append(b, uint8(inst.round))

	return b
}

func (inst *instance) canVoteZero(sender uint32, seq uint64) {
	inst.lock.Lock()
	defer inst.lock.Unlock()

	if inst.round == 0 && !inst.hasVotedOne[inst.round] && !inst.hasVotedZero[inst.round] {
		inst.hasVotedZero[inst.round] = true
		m := &consmsgpb.WholeMessage{
			ConsMsg: &consmsgpb.ConsMessage{
				Type:     consmsgpb.MessageType_BVAL,
				Proposer: sender,
				Round:    inst.round,
				Sequence: seq,
				Content:  []byte{0}, // vote 0
			},
		}
		inst.tp.Broadcast(m)
	}
}

func serialCollection(collection *consmsgpb.Collections) []byte {
	mar_collection, err := proto.Marshal(collection)
	if err != nil {
		panic("Marshal collection failed")
	}
	return mar_collection
}

func deserialCollection(data []byte) consmsgpb.Collections {
	collection := consmsgpb.Collections{}
	err := proto.Unmarshal(data, &collection)
	if err != nil {
		panic("Unmarshal collection failed")
	}
	return collection
}

func verify(content, sign []byte, pk *ecdsa.PrivateKey) bool {
	hash, err := sha256.ComputeHash(content)
	if err != nil {
		panic("sha256 computeHash failed")
	}
	b, err := myecdsa.VerifyECDSA(&pk.PublicKey, sign, hash)
	if err != nil {
		log.Println("Failed to verify a consmsgpb: ", sign[0])
	}
	return b
}

func (inst *instance) verifyECHOCollection(msg *consmsgpb.WholeMessage) bool {
	if inst.proposal == nil {
		log.Println("Have not received a proposal")
		return false
	}
	hash, _ := sha256.ComputeHash(inst.proposal.ConsMsg.Content)
	mc := &consmsgpb.ConsMessage{
		Type:     consmsgpb.MessageType_ECHO,
		Proposer: msg.ConsMsg.Proposer,
		Sequence: msg.ConsMsg.Sequence,
		Content:  hash,
	}
	content, err := proto.Marshal(mc)
	if err != nil {
		log.Printf("proto.Marshal: %v", err)
		return false
	}
	collection := deserialCollection(msg.Collection)
	for i, sign := range collection.Collections {
		if len(sign) == 0 || inst.echoSigns.Collections[i] != nil {
			continue
		}
		if !verify(content, sign, inst.priv) {
			log.Printf("echo collection verify failed: %v", sign[0])
			return false
		}
		inst.numEcho++
		inst.echoSigns.Collections[i] = sign
	}
	return true
}

func (inst *instance) verifyREADYCollection(msg *consmsgpb.WholeMessage) bool {
	if inst.proposal == nil {
		log.Println("Have not received a proposal")
		return false
	}
	hash, _ := sha256.ComputeHash(inst.proposal.ConsMsg.Content)
	mc := &consmsgpb.ConsMessage{
		Type:     consmsgpb.MessageType_READY,
		Proposer: msg.ConsMsg.Proposer,
		Round:    msg.ConsMsg.Round,
		Sequence: msg.ConsMsg.Sequence,
		Content:  hash,
	}
	content, err := proto.Marshal(mc)
	if err != nil {
		log.Printf("proto.Marshal: %v", err)
		return false
	}
	collection := deserialCollection(msg.Collection)
	for i, sign := range collection.Collections {
		if len(sign) == 0 || inst.readySigns.Collections[i] != nil {
			continue
		}
		if !verify(content, sign, inst.priv) {
			return false
		}
		inst.numReady++
		inst.readySigns.Collections[i] = sign
	}
	return true
}

func (inst *instance) VerifyCollection(msg *consmsgpb.WholeMessage) bool {
	mc := &consmsgpb.ConsMessage{
		Proposer: msg.ConsMsg.Proposer,
		Round:    msg.ConsMsg.Round,
		Sequence: msg.ConsMsg.Sequence,
	}
	if msg.ConsMsg.Type == consmsgpb.MessageType_BVAL_ZERO_COLLECTION ||
		msg.ConsMsg.Type == consmsgpb.MessageType_BVAL_ONE_COLLECTION {
		mc.Type = consmsgpb.MessageType_BVAL
	} else {
		mc.Type = consmsgpb.MessageType_AUX
	}
	if msg.ConsMsg.Type == consmsgpb.MessageType_BVAL_ZERO_COLLECTION ||
		msg.ConsMsg.Type == consmsgpb.MessageType_AUX_ZERO_COLLECTION {
		mc.Content = []byte{0}
	} else {
		mc.Content = []byte{1}
	}
	content, err := proto.Marshal(mc)
	if err != nil {
		log.Printf("proto.Marshal: %v", err)
		return false
	}
	collection := deserialCollection(msg.Collection)
	if mc.Content[0] == 0 && msg.ConsMsg.Type == consmsgpb.MessageType_BVAL {
		for i, sign := range collection.Collections {
			if len(sign) == 0 || inst.bvalZeroSigns[msg.ConsMsg.Round].Collections[i] != nil {
				continue
			}
			if !verify(content, sign, inst.priv) {
				log.Println("verify fail")
				return false
			}
			inst.numBvalZero[msg.ConsMsg.Round]++
			inst.bvalZeroSigns[msg.ConsMsg.Round].Collections[i] = sign
		}
	} else if mc.Content[0] == 1 && msg.ConsMsg.Type == consmsgpb.MessageType_BVAL {
		for i, sign := range collection.Collections {
			if len(sign) == 0 || inst.bvalOneSigns[msg.ConsMsg.Round].Collections[i] != nil {
				continue
			}
			if !verify(content, sign, inst.priv) {
				log.Println("verify fail")
				return false
			}
			inst.numBvalOne[msg.ConsMsg.Round]++
			inst.bvalOneSigns[msg.ConsMsg.Round].Collections[i] = sign
		}
	} else if mc.Content[0] == 0 && msg.ConsMsg.Type == consmsgpb.MessageType_AUX {
		for i, sign := range collection.Collections {
			if len(sign) == 0 || inst.auxZeroSigns[msg.ConsMsg.Round].Collections[i] != nil {
				continue
			}
			if !verify(content, sign, inst.priv) {
				log.Println("verify fail")
				return false
			}
			inst.numAuxZero[msg.ConsMsg.Round]++
			inst.auxZeroSigns[msg.ConsMsg.Round].Collections[i] = sign
		}
	} else if mc.Content[0] == 1 && msg.ConsMsg.Type == consmsgpb.MessageType_AUX {
		for i, sign := range collection.Collections {
			if len(sign) == 0 || inst.auxOneSigns[msg.ConsMsg.Round].Collections[i] != nil {
				continue
			}
			if !verify(content, sign, inst.priv) {
				log.Println("verify fail")
				return false
			}
			inst.numAuxOne[msg.ConsMsg.Round]++
			inst.auxOneSigns[msg.ConsMsg.Round].Collections[i] = sign
		}
	}
	return true
}
