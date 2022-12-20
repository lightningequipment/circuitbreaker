package circuitbreaker

type Mode int

const (
	ModeFail Mode = iota
	ModeQueue
	ModeQueuePeerInitiated
)

func (m Mode) String() string {
	switch m {
	case ModeFail:
		return "fail"

	case ModeQueue:
		return "queue"

	case ModeQueuePeerInitiated:
		return "queue_peer_initiated"

	default:
		panic("unknown mode")
	}
}
