package main

import (
	"go.uber.org/zap"
	"golang.org/x/time/rate"
)

type peerController struct {
	htlcs   map[circuitKey]struct{}
	cfg     *groupConfig
	limiter *rate.Limiter
	logger  *zap.SugaredLogger
}

func newPeerController(logger *zap.SugaredLogger, cfg *groupConfig,
	htlcs map[circuitKey]struct{}) *peerController {
	var limiter *rate.Limiter

	// Skip if no interval set.
	limit := rate.Inf
	if cfg.HtlcMinInterval > 0 {
		limit = rate.Every(cfg.HtlcMinInterval)
	}
	limiter = rate.NewLimiter(limit, cfg.HtlcBurstSize)

	logger.Infow("Peer controller initialized",
		"htlcMinInterval", cfg.HtlcMinInterval,
		"htlcBurstSize", cfg.HtlcBurstSize,
		"maxPendingHtlcs", cfg.MaxPendingHtlcs)

	return &peerController{
		cfg:     cfg,
		htlcs:   htlcs,
		limiter: limiter,
		logger:  logger,
	}
}

func (p *peerController) process(event interceptEvent) error {
	resume := p.resume(event.circuitKey)

	return event.resume(resume)
}

func (p *peerController) resume(key circuitKey) bool {
	logger := p.keyLogger(key)

	// If htlc is known, let it through. This can happen with htlcs that are
	// already pending when circuit breaker starts.
	if _, exists := p.htlcs[key]; exists {
		logger.Infow("Resume replay")

		return true
	}

	// Check rate limit.
	if !p.limiter.Allow() {
		logger.Infow("Rate limit hit")

		return false
	}

	// Check max pending.
	if p.cfg.MaxPendingHtlcs > 0 && len(p.htlcs) >= p.cfg.MaxPendingHtlcs {
		logger.Infow("Max pending htlcs hit", "max", p.cfg.MaxPendingHtlcs)

		return false
	}

	// Mark as pending.
	p.htlcs[key] = struct{}{}

	// Let through.
	logger.Infow("Resume")

	return true
}

func (p *peerController) resolved(key circuitKey) {
	_, ok := p.htlcs[key]
	if !ok {
		return
	}

	delete(p.htlcs, key)

	logger := p.keyLogger(key)
	logger.Infow("Resolving htlc",
		"pending_htlcs", len(p.htlcs),
	)
}

func (p *peerController) keyLogger(key circuitKey) *zap.SugaredLogger {
	return p.logger.With(
		"htlc", key.htlc,
		"channel", key.channel)
}
