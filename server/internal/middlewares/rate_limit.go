package middlewares

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/coderianx/Skillora/server/internal/store"
	"github.com/redis/go-redis/v9"
)

var rateLimitScript = redis.NewScript(`
local current = redis.call("INCR", KEYS[1])

if current == 1 then
	redis.call("PEXPIRE", KEYS[1], ARGV[1])
end

return current
`)

// RateLimitByIP, IP başına sabit pencere (fixed window) rate limit uygular.
// r.With(middlewares.RateLimitByIP(5, time.Minute)).Post(...) şeklinde kullanılır.
func RateLimitByIP(
	limit int,
	window time.Duration,
) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(
			w http.ResponseWriter,
			r *http.Request,
		) {
			if limit <= 0 || window <= 0 {
				next.ServeHTTP(w, r)
				return
			}

			ctx := r.Context()

			key := fmt.Sprintf(
				"rate_limit:%s:%s:%s",
				clientIP(r),
				r.Method,
				r.URL.Path,
			)

			count, err := rateLimitScript.Run(
				ctx,
				store.RedisClient,
				[]string{key},
				window.Milliseconds(),
			).Int64()

			// Redis erişilemiyorsa rate limit yüzünden tüm istekleri
			// kesmemek için fail-open davranıyoruz.
			if err != nil {
				next.ServeHTTP(w, r)
				return
			}

			remaining := limit - int(count)

			if remaining < 0 {
				remaining = 0
			}

			w.Header().Set("X-RateLimit-Limit", strconv.Itoa(limit))
			w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(remaining))

			if count > int64(limit) {
				if pttl, err := store.RedisClient.PTTL(ctx, key).Result(); err == nil && pttl > 0 {
					retryAfter := int((pttl + time.Second - 1) / time.Second)

					w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
				}

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)

				json.NewEncoder(w).Encode(map[string]any{
					"error": "rate_limit_exceeded",
				})

				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)

	if err != nil {
		return r.RemoteAddr
	}

	return host
}
