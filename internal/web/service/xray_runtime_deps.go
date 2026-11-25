package service

import (
	"github.com/mhsanaei/3x-ui/v3/internal/web/runtime"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

// LocalRuntimeDeps bridges the local runtime to the core this service owns.
func (s *XrayService) LocalRuntimeDeps() runtime.LocalDeps {
	return runtime.LocalDeps{
		APIPort:          s.GetXrayAPIPort,
		SetNeedRestart:   s.SetToNeedRestart,
		RequestReconcile: s.SetToNeedReconcile,
		InboundApplied: func(inbound *xray.InboundConfig) {
			if process := currentXrayProcess(); process != nil {
				process.PutInbound(inbound)
			}
		},
		InboundRemoved: func(tag string) {
			if process := currentXrayProcess(); process != nil {
				process.DropInbound(tag)
			}
		},
	}
}
