package daemon

import (
	"context"
)

type daemonCtx struct {
	ctx    context.Context
	cancel context.CancelFunc
	errc   chan error
}

func (dc *daemonCtx) Cancel(err error) {
	select {
	case dc.errc <- err:
		dc.cancel()
	default:
	}
}

func (dc *daemonCtx) Done() <-chan struct{} { return dc.ctx.Done() }

func newDaemonCtx() *daemonCtx {
	ctx, cancel := context.WithCancel(context.TODO())
	return &daemonCtx{
		ctx:    ctx,
		cancel: cancel,
		errc:   make(chan error, 1),
	}
}
