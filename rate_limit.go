package main

import (
	"time"

	"golang.org/x/time/rate"
)

type rateLimit struct {
	limiter *rate.Limiter

	baseRate time.Duration
}

func newRateLimit(baseRate time.Duration, burstSize int) *rateLimit {
	limiter := rate.NewLimiter(
		rate.Every(baseRate),
		burstSize,
	)

	return &rateLimit{
		limiter:  limiter,
		baseRate: baseRate,
	}
}

func (r *rateLimit) Allow() bool {
	return r.limiter.Allow()
}
