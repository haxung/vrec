package middleware

import (
	"context"
	"net/http"
	"sync"
	"time"

	"vrec/internal/config"
	"vrec/pkg/response"

	"github.com/gin-gonic/gin"
)

type userBucket struct {
	tokens     int
	lastRefill time.Time
	mu         sync.Mutex
}

type QPSLimiter struct {
	buckets             map[int64]*userBucket
	mu                  sync.RWMutex
	refillMs            int64
	userService         UserBalanceChecker
	defaultLimit        int
	lowBalanceQPS       int
	lowBalanceThreshold float64
}

type UserBalanceChecker interface {
	GetBalanceFloat(ctx context.Context, userID int64) (float64, error)
}

func NewQPSLimiter(limit, burst int, userService UserBalanceChecker, cfg *config.PricingConfig) *QPSLimiter {
	limiter := &QPSLimiter{
		buckets:             make(map[int64]*userBucket),
		refillMs:            100,
		userService:         userService,
		defaultLimit:        limit,
		lowBalanceQPS:       cfg.LowBalanceQPS,
		lowBalanceThreshold: cfg.LowBalanceThreshold,
	}
	if limiter.lowBalanceQPS == 0 {
		limiter.lowBalanceQPS = 3
	}
	go limiter.refillLoop()
	return limiter
}

func (l *QPSLimiter) refillLoop() {
	ticker := time.NewTicker(time.Millisecond * time.Duration(l.refillMs))
	for range ticker.C {
		l.mu.Lock()
		for userID, bucket := range l.buckets {
			bucket.mu.Lock()
			// 获取用户当前的 QPS 限制
			limit := l.getLimitForUser(userID)
			burst := limit * 2 // burst 设置为 limit 的 2 倍
			elapsed := time.Since(bucket.lastRefill).Milliseconds()
			refill := elapsed / l.refillMs * int64(limit)
			if refill > 0 {
				bucket.tokens = min(bucket.tokens+int(refill), burst)
				bucket.lastRefill = time.Now()
			}
			bucket.mu.Unlock()
		}
		l.mu.Unlock()
	}
}

func (l *QPSLimiter) Allow(userID int64) bool {
	// 先检查用户是否是欠费状态
	limit := l.getLimitForUser(userID)
	if limit == 0 {
		return false // 欠费用户不允许创建订单
	}

	l.mu.RLock()
	bucket, exists := l.buckets[userID]
	l.mu.RUnlock()

	if !exists {
		l.mu.Lock()
		bucket, exists = l.buckets[userID]
		if !exists {
			burst := limit * 2
			bucket = &userBucket{
				tokens:     burst, // 根据用户 limit 初始化 burst
				lastRefill: time.Now(),
			}
			l.buckets[userID] = bucket
		}
		l.mu.Unlock()
	}

	bucket.mu.Lock()
	defer bucket.mu.Unlock()

	if bucket.tokens > 0 {
		bucket.tokens--
		return true
	}
	return false
}

func (l *QPSLimiter) getLimitForUser(userID int64) int {
	if l.userService == nil {
		return l.defaultLimit
	}

	// 如果未配置低余额阈值，使用默认行为
	if l.lowBalanceThreshold <= 0 {
		return l.defaultLimit
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	balance, err := l.userService.GetBalanceFloat(ctx, userID)
	if err != nil {
		return l.defaultLimit
	}

	if balance <= 0 {
		return 0 // 欠费用户不允许创建订单
	}

	if balance <= l.lowBalanceThreshold {
		return l.lowBalanceQPS
	}

	return l.defaultLimit
}

func QPSLimitMiddleware(limiter *QPSLimiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := GetUserID(c)
		if userID == 0 {
			c.Next()
			return
		}

		if !limiter.Allow(userID) {
			limit := limiter.getLimitForUser(userID)
			if limit == 0 {
				response.ErrorWithStatus(c, http.StatusForbidden, 403, "insufficient balance, cannot create orders")
			} else {
				response.ErrorWithStatus(c, http.StatusTooManyRequests, 429, "rate limit exceeded")
			}
			c.Abort()
			return
		}
		c.Next()
	}
}
