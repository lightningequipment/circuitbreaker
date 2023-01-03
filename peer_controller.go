package main

import (
	"container/list"
	"context"
	"time"

	"github.com/lightningnetwork/lnd/routing/route"
	"go.uber.org/zap"
	"golang.org/x/time/rate"
)

type peerController struct {
	cfg           *groupConfig
	limiter       *rate.Limiter
	logger        *zap.SugaredLogger
	interceptChan chan peerInterceptEvent
	resolvedChan  chan circuitKey

	htlcs map[circuitKey]struct{}

	lastChannelSync time.Time
	pubKey          route.Vertex
	lnd             lndclient
}

type peerInterceptEvent struct {
	interceptEvent

	peerInitiated bool
}

type peerControllerCfg struct {
	logger *zap.SugaredLogger
	cfg    *groupConfig
	htlcs  map[circuitKey]struct{}
	lnd    lndclient
	pubKey route.Vertex
}

func newPeerController(cfg *peerControllerCfg) *peerController {
	logger := cfg.logger

	var limiter *rate.Limiter

	// Skip if no interval set.
	limit := rate.Inf
	if cfg.cfg.HtlcMinInterval > 0 {
		limit = rate.Every(cfg.cfg.HtlcMinInterval)
	}
	limiter = rate.NewLimiter(limit, cfg.cfg.HtlcBurstSize)

	logger.Infow("Peer controller initialized",
		"htlcMinInterval", cfg.cfg.HtlcMinInterval,
		"htlcBurstSize", cfg.cfg.HtlcBurstSize,
		"maxPendingHtlcs", cfg.cfg.MaxPendingHtlcs,
		"mode", cfg.cfg.Mode)

	// Log initial pending htlcs.
	for h := range cfg.htlcs {
		logger.Infow("Initial pending htlc", "channel", h.channel, "htlc", h.htlc)
	}

	return &peerController{
		cfg:             cfg.cfg,
		limiter:         limiter,
		logger:          logger,
		interceptChan:   make(chan peerInterceptEvent),
		resolvedChan:    make(chan circuitKey),
		htlcs:           cfg.htlcs,
		lnd:             cfg.lnd,
		pubKey:          cfg.pubKey,
		lastChannelSync: time.Now(),
	}
}

func (p *peerController) newHtlcAllowed() bool {
	return p.cfg.MaxPendingHtlcs == 0 ||
		len(p.htlcs) < p.cfg.MaxPendingHtlcs
}

func (p *peerController) syncPendingHtlcs(ctx context.Context) (bool, error) {
	p.logger.Infow("Syncing pending htlcs")

	htlcs, err := p.lnd.getPendingIncomingHtlcs(ctx, &p.pubKey)
	if err != nil {
		return false, err
	}

	p.lastChannelSync = time.Now()

	deletes := false
	for key := range p.htlcs {
		_, ok := htlcs[key]
		if ok {
			continue
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

			// Determine whether the htlc can be forwarded immediately.
			forwardDirect := true
			switch {
			case !newHtlcAllowed:
				forwardDirect = false

				logger.Infow("Failed on pending htlc limit")

			case !p.limiter.Allow():
				forwardDirect = false

				logger.Infow("Failed on rate limit")
			}

			// Forward directly if possible.
			if forwardDirect {
				if err := p.forward(event.interceptEvent); err != nil {
					return err
				}

				continue
			}

			// Queue if in one of the queue modes.
			if p.cfg.Mode == ModeQueue ||
				(p.cfg.Mode == ModeQueuePeerInitiated && event.peerInitiated) {

				queue.PushFront(event)

				logger.Infow("Queued", "queueLen", queue.Len())

				continue
			}

			// Otherwise fail directly.
			if err := event.resume(false); err != nil {
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
		case key := <-p.resolvedChan:
			_, ok := p.htlcs[key]
			if !ok {
				// Do not log here, because the event is still coming even for
				// htlcs that were failed. We don't want to spam the log.

				continue
			}

			delete(p.htlcs, key)

			logger := p.keyLogger(key)
			logger.Infow("Resolved htlc", "pending_htlcs", len(p.htlcs))

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
	key circuitKey) error {

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
