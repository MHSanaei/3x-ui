package runtime

import (
	"sync"

	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

// The local core is reached over one gRPC connection. Opening one per
// operation made a bulk client delete pay a TCP + HTTP/2 handshake per client.
//
// The pooled client must not be used for traffic stats: XrayAPI.StatsLastValues
// is written without a lock, so a second concurrent reader would corrupt it
// (a fatal runtime throw). The traffic poll keeps its own client.
var (
	localAPIMu     sync.Mutex
	localAPIPort   int
	localAPIClient *xray.XrayAPI
)

// localAPIForPort returns the pooled client for the core's API port, rebuilding
// it when the port moved (a core restart can change it) and closing the client
// it replaces. The caller must not close what it gets back.
func localAPIForPort(port int) (*xray.XrayAPI, error) {
	localAPIMu.Lock()
	defer localAPIMu.Unlock()

	if localAPIClient != nil {
		if localAPIPort == port {
			return localAPIClient, nil
		}
		localAPIClient.Close()
		localAPIClient = nil
	}

	api := &xray.XrayAPI{}
	if err := api.Init(port); err != nil {
		return nil, err
	}
	localAPIClient = api
	localAPIPort = port
	return api, nil
}
