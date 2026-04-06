package command

import (
	"context"

	"github.com/xtls/xray-core/app/log"
	"github.com/xtls/xray-core/common"
	"github.com/xtls/xray-core/common/errors"
	"github.com/xtls/xray-core/core"
	grpc "google.golang.org/grpc"
)

type LoggerServer struct {
	V *core.Instance
}

// RestartLogger implements LoggerService.
func (s *LoggerServer) RestartLogger(ctx context.Context, request *RestartLoggerRequest) (*RestartLoggerResponse, error) {
	logger := s.V.GetFeature((*log.Instance)(nil))
	if logger == nil {
		return nil, errors.New("unable to get logger instance")
	}
	if err := logger.Close(); err != nil {
		return nil, errors.New("failed to close logger").Base(err)
	}
	if err := logger.Start(); err != nil {
		return nil, errors.New("failed to start logger").Base(err)
	}
	return &RestartLoggerResponse{}, nil
}

func (s *LoggerServer) mustEmbedUnimplementedLoggerServiceServer() {}

type service struct {
	v *core.Instance
}

func (s *service) Register(server *grpc.Server) {
	ls := &LoggerServer{
		V: s.v,
	}
	RegisterLoggerServiceServer(server, ls)

	// For compatibility purposes
	vCoreDesc := LoggerService_ServiceDesc
	vCoreDesc.ServiceName = "v2ray.core.app.log.command.LoggerService"
	server.RegisterService(&vCoreDesc, ls)
}

func init() {
	common.Must(common.RegisterConfig((*Config)(nil), func(ctx context.Context, cfg interface{}) (interface{}, error) {
		s := core.MustFromContext(ctx)
		return &service{v: s}, nil
	}))
}
