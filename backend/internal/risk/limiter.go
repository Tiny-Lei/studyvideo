package risk

import (
	"sync"
	"time"
)

// slidingLimiter 是一个按 key（通常是 IP）的滑动窗口计数器。
type slidingLimiter struct {
	mu     sync.Mutex
	hits   map[string][]int64
	limit  int
	window time.Duration
}

func newSlidingLimiter(limit int, window time.Duration) *slidingLimiter {
	return &slidingLimiter{
		hits:   make(map[string][]int64),
		limit:  limit,
		window: window,
	}
}

// Allow 返回是否放行；被拒绝时同时返回建议的重试等待秒数。
func (l *slidingLimiter) Allow(key string) (bool, int) {
	if l.limit <= 0 || l.window <= 0 {
		return true, 0
	}
	now := time.Now().UnixNano()
	cutoff := now - l.window.Nanoseconds()

	l.mu.Lock()
	defer l.mu.Unlock()

	arr := l.hits[key]
	idx := 0
	for idx < len(arr) && arr[idx] < cutoff {
		idx++
	}
	if idx > 0 {
		arr = append([]int64(nil), arr[idx:]...)
	}
	if len(arr) >= l.limit {
		retry := int((arr[0] + l.window.Nanoseconds() - now) / int64(time.Second))
		if retry < 1 {
			retry = 1
		}
		l.hits[key] = arr
		return false, retry
	}
	arr = append(arr, now)
	l.hits[key] = arr
	return true, 0
}

func (l *slidingLimiter) Reset(key string) {
	l.mu.Lock()
	delete(l.hits, key)
	l.mu.Unlock()
}

func (l *slidingLimiter) Cleanup() {
	cutoff := time.Now().Add(-l.window).UnixNano()
	l.mu.Lock()
	defer l.mu.Unlock()
	for k, arr := range l.hits {
		idx := 0
		for idx < len(arr) && arr[idx] < cutoff {
			idx++
		}
		if idx >= len(arr) {
			delete(l.hits, k)
		} else if idx > 0 {
			l.hits[k] = append([]int64(nil), arr[idx:]...)
		}
	}
}
