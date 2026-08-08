package sub

import (
	"reflect"
	"testing"
)

func TestExtraSalamanderKeys(t *testing.T) {
	if got := extraSalamanderKeys(map[string]any{"password": "pw"}); len(got) != 0 {
		t.Fatalf("expressible settings reported extras: %v", got)
	}
	got := extraSalamanderKeys(map[string]any{"password": "pw", "packetSize": "512-1200"})
	if want := []string{"packetSize"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("extraSalamanderKeys = %v, want %v", got, want)
	}
}
