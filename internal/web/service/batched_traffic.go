package service

import (
	"sync"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

// BatchedTrafficConfig configures the batched traffic writer
type BatchedTrafficConfig struct {
	FlushInterval time.Duration // How often to flush
	MaxBatchSize  int           // Max items per batch
}

// BatchedTrafficWriter accumulates traffic and flushes in batches
type BatchedTrafficWriter struct {
	svc      InboundServiceInterface
	config   BatchedTrafficConfig
	mu       sync.Mutex
	inbound  []*xray.Traffic
	client   []*xray.ClientTraffic
	ticker   *time.Ticker
	stopCh   chan struct{}
	wg       sync.WaitGroup
	closed   bool
}

// NewBatchedTrafficWriter creates a new batched traffic writer
func NewBatchedTrafficWriter(svc InboundServiceInterface, config BatchedTrafficConfig) *BatchedTrafficWriter {
	if config.FlushInterval <= 0 {
		config.FlushInterval = 100 * time.Millisecond
	}
	if config.MaxBatchSize <= 0 {
		config.MaxBatchSize = 1000
	}

	btw := &BatchedTrafficWriter{
		svc:      svc,
		config:   config,
		client:   make([]*xray.ClientTraffic, 0, config.MaxBatchSize),
		stopCh:   make(chan struct{}),
		closed:   false,
	}

	btw.ticker = time.NewTicker(config.FlushInterval)
	btw.wg.Add(1)
	go btw.run()

	return btw
}

// Submit adds traffic to the batch
func (b *BatchedTrafficWriter) Submit(inboundTraffics []*xray.Traffic, clientTraffics []*xray.ClientTraffic) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		// Fall back to direct write
		b.svc.AddTraffic(inboundTraffics, clientTraffics)
		return
	}

	if len(inboundTraffics) > 0 {
		b.inbound = append(b.inbound, inboundTraffics...)
	}
	if len(clientTraffics) > 0 {
		b.client = append(b.client, clientTraffics...)
	}

	// Flush if batch is full
	if len(b.client) >= b.config.MaxBatchSize {
		b.flushLocked()
	}
}

// flushLocked writes the current batch (must hold lock)
func (b *BatchedTrafficWriter) flushLocked() {
	if len(b.inbound) == 0 && len(b.client) == 0 {
		return
	}

	// Aggregate traffic to combine multiple entries for same email/tag
	aggInbound, aggClient := AggregateTraffic(b.inbound, b.client)

	// Copy current batch
	inboundBatch := aggInbound
	clientBatch := aggClient

	// Reset slices properly by creating new ones with capacity
	b.inbound = make([]*xray.Traffic, 0, b.config.MaxBatchSize)
	b.client = make([]*xray.ClientTraffic, 0, b.config.MaxBatchSize)

	// Submit async to avoid blocking
	b.wg.Add(1)
	go func() {
		defer b.wg.Done()
		if _, _, err := b.svc.AddTraffic(inboundBatch, clientBatch); err != nil {
			// Log error but don't panic
			_ = err
		}
	}()
}

// run flushes periodically
func (b *BatchedTrafficWriter) run() {
	defer b.wg.Done()
	for {
		select {
		case <-b.ticker.C:
			b.mu.Lock()
			b.flushLocked()
			b.mu.Unlock()
		case <-b.stopCh:
			b.mu.Lock()
			b.flushLocked()
			b.mu.Unlock()
			return
		}
	}
}

// Close stops the writer and flushes remaining
func (b *BatchedTrafficWriter) Close() error {
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return nil
	}
	b.closed = true
	close(b.stopCh)
	b.ticker.Stop()
	b.mu.Unlock()

	b.wg.Wait()
	return nil
}

// AggregateTraffic combines multiple traffic reports for the same inbound/client
func AggregateTraffic(inboundTraffics []*xray.Traffic, clientTraffics []*xray.ClientTraffic) ([]*xray.Traffic, []*xray.ClientTraffic) {
	inboundMap := make(map[string]*xray.Traffic)
	for _, t := range inboundTraffics {
		if existing, ok := inboundMap[t.Tag]; ok {
			existing.Up += t.Up
			existing.Down += t.Down
		} else {
			inboundMap[t.Tag] = t
		}
	}

	clientMap := make(map[string]*xray.ClientTraffic)
	for _, t := range clientTraffics {
		if existing, ok := clientMap[t.Email]; ok {
			existing.Up += t.Up
			existing.Down += t.Down
		} else {
			clientMap[t.Email] = t
		}
	}

	resultInbound := make([]*xray.Traffic, 0, len(inboundMap))
	for _, t := range inboundMap {
		resultInbound = append(resultInbound, t)
	}

	resultClient := make([]*xray.ClientTraffic, 0, len(clientMap))
	for _, t := range clientMap {
		resultClient = append(resultClient, t)
	}

	return resultInbound, resultClient
}