// Package ingest 承担遥测批次入库后的异步后处理：
// 快照刷新、指标回填等轻量任务经有界队列交给独立协程执行，
// 队列满时丢弃计数，绝不阻塞同步入库路径。
package ingest

import (
	"context"
	"log"
	"sync"

	"github.com/jb843051627/ropeway-sentry/internal/metrics"
)

// Task 一次后处理任务。
type Task struct {
	BatchID int64
	Run     func(ctx context.Context) error
}

// Pipeline 单工作协程的任务管道。
type Pipeline struct {
	tasks   chan Task
	stop    chan struct{}
	wg      sync.WaitGroup
	metrics *metrics.Metrics
	logger  *log.Logger
}

// New 启动 worker 协程并返回管道。
func New(buffer int, m *metrics.Metrics, logger *log.Logger) *Pipeline {
	if buffer <= 0 {
		buffer = 64
	}
	if logger == nil {
		logger = log.Default()
	}
	p := &Pipeline{
		tasks:   make(chan Task, buffer),
		stop:    make(chan struct{}),
		metrics: m,
		logger:  logger,
	}
	p.wg.Add(1)
	go p.loop()
	return p
}

func (p *Pipeline) loop() {
	defer p.wg.Done()
	for {
		select {
		case task := <-p.tasks:
			p.reportDepth(len(p.tasks))
			func() {
				defer func() {
					if r := recover(); r != nil {
						p.logger.Printf("ingest post-processing panic batch=%d: %v", task.BatchID, r)
					}
				}()
				if err := task.Run(context.Background()); err != nil {
					p.logger.Printf("ingest post-processing failed batch=%d: %v", task.BatchID, err)
				}
			}()
		case <-p.stop:
			return
		}
	}
}

func (p *Pipeline) reportDepth(n int) {
	if p.metrics != nil {
		p.metrics.SetGauge(metrics.IngestQueueDepth, int64(n))
	}
}

// Submit 尝试投递任务；队列满或已停止时返回 false（非阻塞）。
func (p *Pipeline) Submit(task Task) bool {
	select {
	case p.tasks <- task:
		p.reportDepth(len(p.tasks))
		return true
	default:
		return false
	}
}

// Close 停止 worker 并等待退出；幂等可安全多次调用。
func (p *Pipeline) Close() {
	close(p.stop)
	p.wg.Wait()
}
