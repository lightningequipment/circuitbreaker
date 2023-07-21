package main

import (
	"container/list"
	"context"
	"time"

	"github.com/lightningnetwork/lnd/routing/route"
	"github.com/paulbellamy/ratecounter"
	"go.uber.org/zap"
	"golang.org/x/time/rate"
)

type eventCounter struct {
	fail    *ratecounter.RateCounter
	success *ratecounter.RateCounter
	reject  *ratecounter.RateCounter
}

type eventType int

const (
	eventSuccess eventType = iota
	eventFail
	eventReject
)

func newEventCounter(interval time.Duration) *eventCounter {
	return &eventCounter{
		fail:    ratecounter.NewRateCounter(interval),
		success: ratecounter.NewRateCounter(interval),
		reject:  ratecounter.NewRateCounter(interval),
	}
}

func (e *eventCounter) Incr(event eventType) {
	switch event {
	case eventSuccess:
		e.success.Incr(1)

	case eventFail:
		e.fail.Incr(1)

	case eventReject:
		e.reject.Incr(1)

	default:
		panic("unknown event type")
	}
}

func (e *eventCounter) Rates() (int64, int64, int64) {
	return e.success.Rate(), e.fail.Rate(), e.reject.Rate()
}

type peerController struct {
	cfg             Limit
	limiter         *rate.Limiter
	logger          *zap.SugaredLogger
	interceptChan   chan peerInterceptEvent
	resolvedChan    chan resolvedEvent
	updateLimitChan chan Limit
	getStateChan    chan chan *peerState

	rateCounters []*eventCounter

	htlcs map[circuitKey]struct{}

	lastChannelSync time.Time
	pubKey          route.Vertex
	lnd             lndclient
}

type peerInterceptEvent struct {
	interceptEvent

	peerInitiated bool
}

type peerState struct {
	counts           []rateCounts
	queueLen         int64
	pendingHtlcCount int64
}

type rateCounts struct {
	success, fail, reject int64
}

var rateCounterIntervals = []time.Duration{time.Hour, 24 * time.Hour}

type peerControllerCfg struct {
	logger    *zap.SugaredLogger
	limit     Limit
	burstSize int
	htlcs     map[circuitKey]struct{}
	lnd       lndclient
	pubKey    route.Vertex
}

func newPeerController(cfg *peerControllerCfg) *peerController {
	logger := cfg.logger.With(
		"peer", cfg.pubKey.String(),
	)

	// Skip if no interval set.
	limiter := rate.NewLimiter(getRate(cfg.limit.MaxHourlyRate), cfg.burstSize)

	logger.Infow("Peer controller initialized",
		"maxHourlyRate", cfg.limit.MaxHourlyRate,
		"maxPendingHtlcs", cfg.limit.MaxPending,
		"mode", cfg.limit.Mode)

	// Log initial pending htlcs.
	for h := range cfg.htlcs {
		logger.Infow("Initial pending htlc", "channel", h.channel, "htlc", h.htlc)
	}

	rateCounters := make([]*eventCounter, len(rateCounterIntervals))
	for idx, interval := range rateCounterIntervals {
		rateCounters[idx] = newEventCounter(interval)
	}

	return &peerController{
		cfg:             cfg.limit,
		limiter:         limiter,
		logger:          logger,
		interceptChan:   make(chan peerInterceptEvent),
		resolvedChan:    make(chan resolvedEvent),
		updateLimitChan: make(chan Limit),
		getStateChan:    make(chan chan *peerState),
		htlcs:           cfg.htlcs,
		rateCounters:    rateCounters,
		lnd:             cfg.lnd,
		pubKey:          cfg.pubKey,
		lastChannelSync: time.Now(),
	}
}

func (p *peerController) state(ctx context.Context) (*peerState, error) {
	respChan := make(chan *peerState)
	select {
	case p.getStateChan <- respChan:
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	select {
	case state := <-respChan:
		return state, nil

	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (p *peerController) rateInternal() []rateCounts {
	allRateCounts := make([]rateCounts, len(p.rateCounters))
	for idx, counter := range p.rateCounters {
		success, fail, reject := counter.Rates()
		allRateCounts[idx] = rateCounts{
			success: success,
			fail:    fail,
			reject:  reject,
		}
	}

	return allRateCounts
}

func (p *peerController) updateLimit(ctx context.Context, limit Limit) error {
	select {
	case p.updateLimitChan <- limit:
		return nil

	case <-ctx.Done():
		return ctx.Err()
	}
}

func (p *peerController) newHtlcAllowed() bool {
	return p.cfg.MaxPending == 0 ||
		len(p.htlcs) < int(p.cfg.MaxPending)
}

func (p *peerController) syncPendingHtlcs(ctx context.Context) (bool, error) {
	p.logger.Infow("Syncing pending htlcs")

	allHtlcs, err := p.lnd.getPendingIncomingHtlcs(ctx, &p.pubKey)
	if err != nil {
		return false, err
	}

	htlcs := allHtlcs[p.pubKey]

	p.lastChannelSync = time.Now()

	deletes := false
	for key := range p.htlcs {
		if htlcs != nil {
			if _, ok := htlcs[key]; ok {
				continue
			}
		}

		// Htlc is no longer pending on incoming side. Must have missed
		// an htlc event. Clear it from our list.
		delete(p.htlcs, key)

		logger := p.keyLogger(key)
		logger.Infow("Cleaning up dangling htlc")

		deletes = true
	}

	return deletes, nil
}

func (p *peerController) run(ctx context.Context) error {
	queue := list.New()

	var reservation *rate.Reservation

	for {
		// New htlcs are allowed when the number of pending htlcs is below the
		// limit, or no limit has been set.
		newHtlcAllowed := p.newHtlcAllowed()

		// If no new htlcs are allowed and we've not synced recently, re-sync.
		// Sometimes htlc events aren't broadcast by lnd, and this keeps our
		// pending htlc count accurate.
		if !newHtlcAllowed && time.Since(p.lastChannelSync) > time.Minute {
			deletes, err := p.syncPendingHtlcs(ctx)
			if err != nil {
				return err
			}

			// When dangling htlcs are removed, re-evaluate whether a new htlc
			// is allowed.
			if deletes {
				newHtlcAllowed = p.newHtlcAllowed()
			}
		}

		// If an htlc can be forwarded, make a reservation on the rate limiter
		// if it does not already exist.
		if queue.Len() > 0 && newHtlcAllowed && reservation == nil {
			reservation = p.limiter.Reserve()
		}

		// Create a delay channel based on the rate limiter delay. If there is
		// no htlc to forward or the pending limit has been reached, use a nil
		// channel to skip the select case.
		var delayChan <-chan time.Time
		if reservation != nil {
			delayChan = time.After(reservation.Delay())
		}

		select {
		// A new htlc is intercepted. Depending on the mode the controller is
		// running in, the htlc will either be queued or handled immediately.
		case event := <-p.interceptChan:
			logger := p.keyLogger(event.circuitKey)

			// Replays can happen when the htlcs map is initialized with a
			// pending htlc on startup, and then a forward event happens for
			// that htlc. For those htlcs, just resume.
			_, ok := p.htlcs[event.circuitKey]
			if ok {
				if err := event.resume(true); err != nil {
					return err
				}

				logger.Infow("Replay")

				continue
			}

			mode := p.cfg.Mode

			switch {
			// Don't check limits in block mode and move onwards to failing the
			// htlc.
			case mode == ModeBlock:
				logger.Infow("Htlc blocked")

			// If there is a queue, then don't jump the queue.
			case queue.Len() > 0:

			// Check if new htlcs are allowed.
			case !newHtlcAllowed:
				logger.Infow("Pending htlc limit exceeded")

			// Check the rate limit.
			case !p.limiter.Allow():
				logger.Infow("Rate limit exceeded")

			// All signs green, forward the htlc.
			default:
				if err := p.forward(event.interceptEvent); err != nil {
					return err
				}

				continue
			}

			// Queue if in one of the queue modes.
			if mode == ModeQueue ||
				(mode == ModeQueuePeerInitiated && event.peerInitiated) {

				queue.PushFront(event)

				logger.Infow("Queued", "queueLen", queue.Len())

				continue
			}

			// Otherwise fail directly.
			if err := event.resume(false); err != nil {
				return err
			}

			p.incrCounter(eventReject)

		// There are items in the queue, max pending htlcs has not yet been
		// reached, and the rate limit delay has passed. Take the oldest item
		// from the queue and forward it.
		case <-delayChan:
			listItem := queue.Back()
			if listItem == nil {
				panic("list empty")
			}
			queue.Remove(listItem)

			event := listItem.Value.(peerInterceptEvent)

			if err := p.forward(event.interceptEvent); err != nil {
				return err
			}

			// Reservation has been used. Clear it so that a new reservation can
			// be requested.
			reservation = nil

		// An htlc has been resolved in lnd. Remove it from the pending htlcs
		// map to free up the slot for another htlc.
		case resolvedEvent := <-p.resolvedChan:
			key := resolvedEvent.incomingCircuitKey

			_, ok := p.htlcs[key]
			if !ok {
				// Do not log here, because the event is still coming even for
				// htlcs that were failed. We don't want to spam the log.

				continue
			}

			delete(p.htlcs, key)

			// Update rate counters.
			if resolvedEvent.settled {
				p.incrCounter(eventSuccess)
			} else {
				p.incrCounter(eventFail)
			}

			logger := p.keyLogger(key)
			logger.Infow("Resolved htlc", "settled", resolvedEvent.settled,
				"pending_htlcs", len(p.htlcs))

		case limit := <-p.updateLimitChan:
			p.logger.Infow("Updating peer controller", "limit", limit)
			p.cfg = limit

			p.limiter.SetLimit(getRate(limit.MaxHourlyRate))

		case respChan := <-p.getStateChan:
			counts := p.rateInternal()

			select {
			case respChan <- &peerState{
				counts:           counts,
				queueLen:         int64(queue.Len()),
				pendingHtlcCount: int64(len(p.htlcs)),
			}:

			case <-ctx.Done():
				return ctx.Err()
			}

		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (p *peerController) incrCounter(event eventType) {
	for _, counter := range p.rateCounters {
		counter.Incr(event)
	}
}

func getRate(maxHourlyRate int64) rate.Limit {
	if maxHourlyRate == 0 {
		return rate.Inf
	}

	return rate.Limit(float64(maxHourlyRate) / 3600)
}

func (p *peerController) forward(event interceptEvent) error {
	p.htlcs[event.circuitKey] = struct{}{}

	err := event.resume(true)
	if err != nil {
		return err
	}

	logger := p.keyLogger(event.circuitKey)
	logger.Infow("Forwarded", "pending_htlcs", len(p.htlcs))

	return nil
}

func (p *peerController) process(ctx context.Context,
	event peerInterceptEvent) error {

	select {
	case p.interceptChan <- event:
		return nil

	case <-ctx.Done():
		return ctx.Err()
	}
}

func (p *peerController) resolved(ctx context.Context,
	key resolvedEvent) error {

	select {
	case p.resolvedChan <- key:
		return nil

	case <-ctx.Done():
		return ctx.Err()
	}
}

func (p *peerController) keyLogger(key circuitKey) *zap.SugaredLogger {
	return p.logger.With(
		"htlc", key.htlc,
		"channel", key.channel)
}
