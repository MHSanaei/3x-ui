package runtime

import (
	"sync"

	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

// One pooled gRPC client serves local runtime operations; never use it for traffic stats, whose
// unlocked StatsLastValues a second reader would corrupt, so the traffic poll keeps its own client.
var (
	localAPIMu     sync.Mutex
	localAPIPort   int
	localAPIClient *xray.XrayAPI
)

// localAPIForPort returns the pooled client for the API port, replacing (and closing) it when a core
// restart moved the port; callers must not close it.
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

// evictLocalAPIClient closes the pooled client so the next call dials a fresh connection, unless
// another call already replaced the one that failed.
func evictLocalAPIClient(api *xray.XrayAPI) {
	localAPIMu.Lock()
	defer localAPIMu.Unlock()

	if localAPIClient != api {
		return
	}
	localAPIClient.Close()
	localAPIClient = nil
	localAPIPort = 0
}
