package risk

import (
	"testing"
	"time"
)

func TestSlidingLimiter(t *testing.T) {
	l := newSlidingLimiter(3, 50*time.Millisecond)
	for i := 0; i < 3; i++ {
		if ok, _ := l.Allow("ip"); !ok {
			t.Fatalf("第 %d 次请求应放行", i+1)
		}
	}
	if ok, retry := l.Allow("ip"); ok || retry < 1 {
		t.Fatalf("超过限制应拒绝并给出重试秒数, ok=%v retry=%d", ok, retry)
	}
	if ok, _ := l.Allow("other"); !ok {
		t.Fatal("不同 key 不应互相影响")
	}
	time.Sleep(60 * time.Millisecond)
	if ok, _ := l.Allow("ip"); !ok {
		t.Fatal("窗口过期后应重新放行")
	}
}

func TestEndOfDay(t *testing.T) {
	until := endOfDay()
	if until.Hour() != 0 || until.Minute() != 0 {
		t.Fatalf("endOfDay 应为次日零点, got %v", until)
	}
	if !until.After(time.Now()) {
		t.Fatalf("endOfDay 必须在当前时间之后")
	}
}
