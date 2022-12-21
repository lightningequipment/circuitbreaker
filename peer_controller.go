package circuitbreaker

import (
	"container/list"
	"context"
	"time"

	"github.com/paulbellamy/ratecounter"
	"go.uber.org/zap"
	"golang.org/x/time/rate"
)

type eventCounter struct {
	total     *ratecounter.RateCounter
	successes *ratecounter.RateCounter
}

func newEventCounter(interval time.Duration) *eventCounter {
	return &eventCounter{
		total:     ratecounter.NewRateCounter(interval),
		successes: ratecounter.NewRateCounter(interval),
	}
}

func (e *eventCounter) Incr(success bool) {
	e.total.Incr(1)
	if success {
		e.successes.Incr(1)
	}
}

func (e *eventCounter) Rates() (int64, int64) {
	return e.successes.Rate(), e.total.Rate()
}

type peerController struct {
	cfg             Limit
	limiter         *rate.Limiter
	logger          *zap.SugaredLogger
	interceptChan   chan peerInterceptEvent
	resolvedChan    chan resolvedEvent
	updateLimitChan chan Limit

	rateCounters []*eventCounter

	htlcs map[circuitKey]struct{}
}

type peerInterceptEvent struct {
	interceptEvent

	peerInitiated bool
}

type rateCounts struct {
	total, success int64
}

var rateCounterIntervals = []time.Duration{5 * time.Minute, time.Hour, 24 * time.Hour}

func newPeerController(logger *zap.SugaredLogger, cfg Limit,
	htlcs map[circuitKey]struct{}) *peerController {

	var limiter *rate.Limiter

	// Skip if no interval set.
	limit := rate.Inf
	if cfg.MinIntervalMs > 0 {
		limit = rate.Every(time.Duration(cfg.MinIntervalMs) * time.Millisecond)
	}
	limiter = rate.NewLimiter(limit, int(cfg.BurstSize))

	logger.Infow("Peer controller initialized",
		"htlcMinInterval", cfg.MinIntervalMs,
		"htlcBurstSize", cfg.BurstSize,
		"maxPendingHtlcs", cfg.MaxPending)

	// "mode", cfg.Mode)

	// Log initial pending htlcs.
	for h := range htlcs {
		logger.Infow("Initial pending htlc", "channel", h.channel, "htlc", h.htlc)
	}

	rateCounters := make([]*eventCounter, len(rateCounterIntervals))
	for idx, interval := range rateCounterIntervals {
		rateCounters[idx] = newEventCounter(interval)
	}

	return &peerController{
		cfg:             cfg,
		limiter:         limiter,
		logger:          logger,
		interceptChan:   make(chan peerInterceptEvent),
		resolvedChan:    make(chan resolvedEvent),
		updateLimitChan: make(chan Limit),
		htlcs:           htlcs,
		rateCounters:    rateCounters,
	}
}

func (p *peerController) rate() []rateCounts {
	allRateCounts := make([]rateCounts, len(p.rateCounters))
	for idx, counter := range p.rateCounters {
		success, total := counter.Rates()
		allRateCounts[idx] = rateCounts{
			success: success,
			total:   total,
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

func (p *peerController) run(ctx context.Context) error {
	queue := list.New()

	var reservation *rate.Reservation

	for {
		// New htlcs are allowed when the number of pending htlcs is below the
		// limit, or no limit has been set.
		newHtlcAllowed := p.cfg.MaxPending == 0 ||
			len(p.htlcs) < int(p.cfg.MaxPending)

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

			_, ok := p.htlcs[event.circuitKey]
			if ok {
				logger.Infow("Replay")

				continue
			}

			//mode := p.cfg.Mode
			mode := ModeFail
			if mode == ModeQueue ||
				(mode == ModeQueuePeerInitiated && event.peerInitiated) {

				queue.PushFront(event)

				logger.Infow("Queued", "queueLen", queue.Len())

				continue
			}

			if !newHtlcAllowed {
				if err := event.resume(false); err != nil {
					return err
				}

				logger.Infow("Failed on pending htlc limit")

				continue
			}

			if !p.limiter.Allow() {
				if err := event.resume(false); err != nil {
					return err
				}

				logger.Infow("Failed on rate limit")

				continue
			}

			if err := p.forward(event.interceptEvent); err != nil {
				return err
			}

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
			key := resolvedEvent.circuitKey

			_, ok := p.htlcs[key]
			if !ok {
				// Do not log here, because the event is still coming even for
				// htlcs that were failed. We don't want to spam the log.

				continue
			}

			delete(p.htlcs, key)

			// Update rate counters.
			for _, counter := range p.rateCounters {
				counter.Incr(resolvedEvent.settled)
			}

			logger := p.keyLogger(key)
			logger.Infow("Resolved htlc", "settled", resolvedEvent.settled,
				"pending_htlcs", len(p.htlcs))

		case limit := <-p.updateLimitChan:
			p.cfg = limit

			p.limiter.SetBurst(int(limit.BurstSize))
			p.limiter.SetLimit(rate.Limit(limit.MinIntervalMs))

		case <-ctx.Done():
			return ctx.Err()
		}
	}
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
