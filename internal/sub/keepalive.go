package sub

import (
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func keepAliveForScalarExport(client model.Client) (int, bool) {
	if client.KeepAlive == nil {
		return 0, false
	}
	if strings.Contains(client.KeepAliveText(), "-") && client.KeepAlive.Validate(true) == nil {
		return client.KeepAlive.MinSeconds(), true
	}
	value := client.KeepAliveSeconds()
	return value, value > 0
}
