package maskcompat

import (
	"fmt"
	"strings"
)

// SalamanderMinPSK is xray-core's smPSKMinLen, in bytes; the core enforces it only when a socket
// opens, so a shorter password loads fine and then stops the listener or fails every dial.
const SalamanderMinPSK = 4

// CheckSalamanderPasswords refuses a salamander (or gecko) UDP mask the core would not open.
func CheckSalamanderPasswords(finalmask any) error {
	fm, _ := finalmask.(map[string]any)
	masks, _ := fm["udp"].([]any)
	for _, entry := range masks {
		mask, _ := entry.(map[string]any)
		if maskType, _ := mask["type"].(string); !strings.EqualFold(maskType, "salamander") {
			continue
		}
		settings, _ := mask["settings"].(map[string]any)
		if password, _ := settings["password"].(string); len(password) < SalamanderMinPSK {
			return fmt.Errorf("salamander finalmask password must be at least %d bytes; xray-core refuses a shorter one", SalamanderMinPSK)
		}
	}
	return nil
}
