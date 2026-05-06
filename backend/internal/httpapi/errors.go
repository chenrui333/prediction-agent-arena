package httpapi

import "errors"

var (
	errNoActiveRound = errors.New("no active round")
	errPausedRound   = errors.New("round is paused")
	errInvalidMarket = errors.New("invalid market")
	errMarketNotOpen = errors.New("market is not open")
)
