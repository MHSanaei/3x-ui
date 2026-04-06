package core

//go:generate go run github.com/golang/mock/mockgen -package mocks -destination ../testing/mocks/io.go -mock_names Reader=Reader,Writer=Writer io Reader,Writer
//go:generate go run github.com/golang/mock/mockgen -package mocks -destination ../testing/mocks/log.go -mock_names Handler=LogHandler github.com/xtls/xray-core/common/log Handler
//go:generate go run github.com/golang/mock/mockgen -package mocks -destination ../testing/mocks/mux.go -mock_names ClientWorkerFactory=MuxClientWorkerFactory github.com/xtls/xray-core/common/mux ClientWorkerFactory
//go:generate go run github.com/golang/mock/mockgen -package mocks -destination ../testing/mocks/dns.go -mock_names Client=DNSClient github.com/xtls/xray-core/features/dns Client
//go:generate go run github.com/golang/mock/mockgen -package mocks -destination ../testing/mocks/outbound.go -mock_names Manager=OutboundManager,HandlerSelector=OutboundHandlerSelector github.com/xtls/xray-core/features/outbound Manager,HandlerSelector
//go:generate go run github.com/golang/mock/mockgen -package mocks -destination ../testing/mocks/proxy.go -mock_names Inbound=ProxyInbound,Outbound=ProxyOutbound github.com/xtls/xray-core/proxy Inbound,Outbound
