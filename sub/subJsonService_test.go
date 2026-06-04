package sub

import (
	"encoding/json"
	"testing"
)

func TestSubJsonServiceKeepsDirectOutAndAddsFinalMask(t *testing.T) {
	fragment := `{"packets":"1-3","length":"100-200","interval":"10-20","maxSplit":"100-200"}`
	noises := `[{"type":"rand","packet":"10-20","delay":"10-16","applyTo":"ip"},{"type":"base64","packet":"SGVsbG8=","delay":"5"}]`
	svc := NewSubJsonService(fragment, noises, "", "", nil)

	var directOut map[string]any
	if err := json.Unmarshal(svc.defaultOutbounds[len(svc.defaultOutbounds)-1], &directOut); err != nil {
		t.Fatalf("failed to unmarshal compatibility direct_out: %v", err)
	}
	if directOut["tag"] != "direct_out" {
		t.Fatalf("direct_out tag = %v, want direct_out", directOut["tag"])
	}
	directSettings, _ := directOut["settings"].(map[string]any)
	if _, ok := directSettings["fragment"]; !ok {
		t.Fatal("compatibility direct_out is missing freedom fragment")
	}
	if _, ok := directSettings["noises"]; !ok {
		t.Fatal("compatibility direct_out is missing freedom noises")
	}

	stream := svc.streamData(`{"network":"tcp","security":"none","tcpSettings":{"header":{"type":"none"}}}`)
	if _, ok := stream["sockopt"]; !ok {
		t.Fatal("streamSettings is missing direct_out sockopt compatibility path")
	}

	finalmask, _ := stream["finalmask"].(map[string]any)
	if finalmask == nil {
		t.Fatal("streamSettings is missing finalmask")
	}

	tcpMasks, _ := finalmask["tcp"].([]any)
	if len(tcpMasks) != 1 {
		t.Fatalf("finalmask tcp masks len = %d, want 1", len(tcpMasks))
	}
	fragmentMask, _ := tcpMasks[0].(map[string]any)
	if fragmentMask["type"] != "fragment" {
		t.Fatalf("tcp mask type = %v, want fragment", fragmentMask["type"])
	}
	fragmentSettings, _ := fragmentMask["settings"].(map[string]any)
	if fragmentSettings["delay"] != "10-20" {
		t.Fatalf("fragment delay = %v, want 10-20", fragmentSettings["delay"])
	}
	if _, ok := fragmentSettings["interval"]; ok {
		t.Fatal("finalmask fragment should use delay, not interval")
	}

	udpMasks, _ := finalmask["udp"].([]any)
	if len(udpMasks) != 1 {
		t.Fatalf("finalmask udp masks len = %d, want 1", len(udpMasks))
	}
	noiseMask, _ := udpMasks[0].(map[string]any)
	if noiseMask["type"] != "noise" {
		t.Fatalf("udp mask type = %v, want noise", noiseMask["type"])
	}
	noiseSettings, _ := noiseMask["settings"].(map[string]any)
	noiseItems, _ := noiseSettings["noise"].([]any)
	if len(noiseItems) != 2 {
		t.Fatalf("noise items len = %d, want 2", len(noiseItems))
	}
	randItem, _ := noiseItems[0].(map[string]any)
	if randItem["rand"] != "10-20" {
		t.Fatalf("rand noise item rand = %v, want 10-20", randItem["rand"])
	}
	if _, ok := randItem["applyTo"]; ok {
		t.Fatal("finalmask noise should not carry freedom noises applyTo")
	}
	packetItem, _ := noiseItems[1].(map[string]any)
	if packetItem["type"] != "base64" || packetItem["packet"] != "SGVsbG8=" {
		t.Fatalf("packet noise item = %#v, want base64 packet", packetItem)
	}
}

func TestSubJsonServiceAppendsFinalMaskToExistingMasks(t *testing.T) {
	fragment := `{"packets":"tlshello","length":"100-200","interval":"0"}`
	svc := NewSubJsonService(fragment, "", "", "", nil)

	stream := svc.streamData(`{
		"network":"tcp",
		"security":"none",
		"tcpSettings":{"header":{"type":"none"}},
		"finalmask":{"tcp":[{"type":"sudoku"}],"udp":[{"type":"salamander","settings":{"password":"secret"}}]}
	}`)

	finalmask, _ := stream["finalmask"].(map[string]any)
	tcpMasks, _ := finalmask["tcp"].([]any)
	if len(tcpMasks) != 2 {
		t.Fatalf("finalmask tcp masks len = %d, want 2", len(tcpMasks))
	}
	firstTCP, _ := tcpMasks[0].(map[string]any)
	secondTCP, _ := tcpMasks[1].(map[string]any)
	if firstTCP["type"] != "sudoku" || secondTCP["type"] != "fragment" {
		t.Fatalf("tcp masks = %#v, want existing mask followed by subscription fragment", tcpMasks)
	}

	udpMasks, _ := finalmask["udp"].([]any)
	if len(udpMasks) != 1 {
		t.Fatalf("finalmask udp masks len = %d, want existing udp mask preserved", len(udpMasks))
	}
}
