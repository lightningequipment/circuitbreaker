package main

type Mode int

const (
	ModeFail Mode = iota
	ModeQueue
	ModeQueuePeerInitiated
)

func (m Mode) String() string {
	switch m {
	case ModeFail:
		return "FAIL"

	case ModeQueue:
		return "QUEUE"

	case ModeQueuePeerInitiated:
		return "QUEUE_PEER_INITIATED"

	default:
		panic("unknown mode")
	}
}
