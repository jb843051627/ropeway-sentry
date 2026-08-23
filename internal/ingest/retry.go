package ingest

import (
	"context"
	"time"

	"github.com/jb843051627/ropeway-sentry/internal/clock"
)

// Retry 以线性递增退避重试 fn 至多 attempts 次。
// 返回最后一次错误；ctx 取消立即终止。
func Retry(ctx context.Context, attempts int, fn func() error) error {
	var err error
	for i := 0; i < attempts; i++ {
		if err = fn(); err == nil {
			return nil
		}
		backoff := time.Duration(i+1) * 20 * time.Millisecond
		if err = clock.Wait(ctx, backoff); err != nil {
			return err
		}
	}
	return err
}
