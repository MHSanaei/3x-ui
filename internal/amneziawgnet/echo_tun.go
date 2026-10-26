package amneziawgnet

import "gvisor.dev/gvisor/pkg/buffer"

func (t *stackTun) emitEcho(p []byte) bool {
	view := buffer.NewViewWithData(p)
	select {
	case <-t.done:
		view.Release()
		return false
	default:
	}
	select {
	case t.incomingPacket <- view:
		return true
	default:
		view.Release()
		return false
	}
}
