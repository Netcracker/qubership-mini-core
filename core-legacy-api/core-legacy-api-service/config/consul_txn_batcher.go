package config

import (
	"context"

	"github.com/hashicorp/consul/api"
)

// txnBatcher accumulates Consul KV transaction operations and flushes them
// in batches that respect Consul's per-transaction size and count limits.
type txnBatcher struct {
	ctx   context.Context
	flush func(ctx context.Context, ops api.TxnOps) error

	ops  api.TxnOps
	size int
}

func newTxnBatcher(ctx context.Context, flush func(context.Context, api.TxnOps) error) *txnBatcher {
	return &txnBatcher{ctx: ctx, flush: flush}
}

// add appends an operation, flushing the current batch first if adding it
// would exceed the size or count limit.
func (b *txnBatcher) add(op *api.KVTxnOp, opSize int) error {
	if len(b.ops) > 0 && (b.size+opSize > txnMaxReqLen || len(b.ops) == consulTxOperationLimit) {
		logger.DebugC(b.ctx, "Batch limit reached (batch=%d, size=%d), flushing transaction", len(b.ops), b.size)
		if err := b.flushBatch(); err != nil {
			return err
		}
	}

	b.ops = append(b.ops, &api.TxnOp{KV: op})
	b.size += opSize
	return nil
}

// done flushes any remaining operations.
func (b *txnBatcher) done() error {
	if len(b.ops) == 0 {
		return nil
	}
	return b.flushBatch()
}

func (b *txnBatcher) flushBatch() error {
	if err := b.flush(b.ctx, b.ops); err != nil {
		return err
	}
	b.ops = nil
	b.size = 0
	return nil
}
