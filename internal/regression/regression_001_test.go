package regression

import (
	"sync"
	"testing"
	"time"

	"github.com/jb843051627/ropeway-sentry/internal/cache"
)

func TestBug01_cache_set_write_race(t *testing.T) {
	c := cache.New(time.Minute)
	const workers = 64
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			<-start
			for j := 0; j < 50; j++ {
				c.SetWithTTL("hot-key", n*1000+j, time.Minute)
			}
		}(i)
	}
	close(start)
	wg.Wait()
	if v, ok := c.Get("hot-key"); !ok || v == nil {
		t.Fatalf("hot-key missing after concurrent SetWithTTL writes")
	}
}
