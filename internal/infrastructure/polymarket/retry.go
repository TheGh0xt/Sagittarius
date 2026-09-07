package polymarket

import (
	"crypto/rand"
	"math/big"
	"time"
)

const (
	// maxAttempts is the total number of tries for one logical request,
	// including the first: 1 initial attempt plus up to 2 retries.
	maxAttempts = 3

	retryBaseDelay = 150 * time.Millisecond
	retryMaxDelay  = 1 * time.Second
)

// isRetryableStatus reports whether an upstream HTTP status is worth retrying.
//
// Only 5xx: a 4xx means the request itself was rejected (bad params, not
// found, rate limited in a way retrying-with-backoff won't fix within this
// budget), and retrying it just repeats the same failure three times slower.
func isRetryableStatus(status int) bool {
	return status >= 500
}

// backoff returns the delay before retry attempt n (0-based: 0 is the delay
// before the first retry), exponential with full jitter and a cap, so a
// burst of failing requests does not retry in lockstep against an already
// struggling upstream.
func backoff(attempt int) time.Duration {
	d := retryBaseDelay << attempt
	if d > retryMaxDelay || d <= 0 {
		d = retryMaxDelay
	}
	// crypto/rand rather than math/rand: this package has no seeded RNG
	// elsewhere and pulling one in for jitter alone is not worth a second
	// source of randomness to reason about.
	n, err := rand.Int(rand.Reader, big.NewInt(int64(d)+1))
	if err != nil {
		return d
	}
	return time.Duration(n.Int64())
}
