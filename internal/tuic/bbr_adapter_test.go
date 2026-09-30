package tuic

import (
	"testing"
	"time"

	original "github.com/apernet/quic-go/congestion"
	originaltime "github.com/apernet/quic-go/monotime"
	fork "github.com/poise52/quic-go/congestion"
	forktime "github.com/poise52/quic-go/monotime"
)

type bbrBridgeRecorder struct {
	original.CongestionControlEx
	deadline        originaltime.Time
	eventTime       originaltime.Time
	pacingTime      originaltime.Time
	inFlight        original.ByteCount
	number          original.PacketNumber
	bytes           original.ByteCount
	retransmittable bool
	acked           []original.AckedPacketInfo
	lost            []original.LostPacketInfo
}

func (r *bbrBridgeRecorder) TimeUntilSend(inFlight original.ByteCount) originaltime.Time {
	r.inFlight = inFlight
	return r.deadline
}

func (r *bbrBridgeRecorder) HasPacingBudget(now originaltime.Time) bool {
	r.pacingTime = now
	return true
}

func (r *bbrBridgeRecorder) OnPacketSent(at originaltime.Time, inFlight original.ByteCount, number original.PacketNumber, bytes original.ByteCount, retransmittable bool) {
	r.eventTime, r.inFlight, r.number, r.bytes, r.retransmittable = at, inFlight, number, bytes, retransmittable
}

func (r *bbrBridgeRecorder) OnPacketAcked(number original.PacketNumber, bytes, inFlight original.ByteCount, at originaltime.Time) {
	r.number, r.bytes, r.inFlight, r.eventTime = number, bytes, inFlight, at
}

func (r *bbrBridgeRecorder) OnCongestionEventEx(inFlight original.ByteCount, at originaltime.Time, acked []original.AckedPacketInfo, lost []original.LostPacketInfo) {
	r.inFlight, r.eventTime = inFlight, at
	r.acked = append(r.acked[:0], acked...)
	r.lost = append(r.lost[:0], lost...)
}

func TestBBRBridgePreservesMonotonicInstantsAndZeroPacingSentinel(t *testing.T) {
	instant := time.Now().Add(time.Second)
	sender := &bbrBridgeRecorder{deadline: originaltime.FromTime(instant)}
	adapter := &xrayBBRAdapter{sender: sender}
	if got := adapter.TimeUntilSend(123); !got.ToTime().Equal(instant) || sender.inFlight != 123 {
		t.Fatalf("pacing instant changed: %v", got.ToTime())
	}
	sender.deadline = 0
	if got := adapter.TimeUntilSend(0); !got.IsZero() {
		t.Fatal("zero pacing sentinel changed")
	}
	if !adapter.HasPacingBudget(forktime.FromTime(instant)) || !sender.pacingTime.ToTime().Equal(instant) {
		t.Fatal("pacer input changed clock epoch")
	}
	adapter.OnPacketSent(forktime.FromTime(instant), 456, 7, 1200, true)
	if !sender.eventTime.ToTime().Equal(instant) || sender.inFlight != 456 || sender.number != 7 || sender.bytes != 1200 || !sender.retransmittable {
		t.Fatal("sent packet conversion changed event")
	}
	adapter.OnPacketAcked(8, 1300, 999, forktime.FromTime(instant))
	if !sender.eventTime.ToTime().Equal(instant) || sender.inFlight != 999 || sender.number != 8 || sender.bytes != 1300 {
		t.Fatal("ACK conversion changed event")
	}
}

func TestBBRBridgeConvertsBatchEventsAndReusesBuffers(t *testing.T) {
	instant := time.Now()
	sender := &bbrBridgeRecorder{}
	adapter := &xrayBBRAdapter{sender: sender}
	acked := []fork.AckedPacketInfo{{PacketNumber: 7, BytesAcked: 1200, ReceivedTime: forktime.FromTime(instant)}, {PacketNumber: 8, BytesAcked: 1300}}
	lost := []fork.LostPacketInfo{{PacketNumber: 9, BytesLost: 1400}}
	adapter.OnCongestionEventEx(5000, forktime.FromTime(instant), acked, lost)
	if sender.inFlight != 5000 || !sender.eventTime.ToTime().Equal(instant) || len(sender.acked) != 2 || len(sender.lost) != 1 {
		t.Fatal("batch shape changed")
	}
	if sender.acked[0].PacketNumber != 7 || sender.acked[0].BytesAcked != 1200 || !sender.acked[0].ReceivedTime.ToTime().Equal(instant) || !sender.acked[1].ReceivedTime.IsZero() {
		t.Fatal("ACK batch conversion changed event")
	}
	if sender.lost[0].PacketNumber != 9 || sender.lost[0].BytesLost != 1400 {
		t.Fatal("loss batch conversion changed event")
	}
	ackBuffer, lossBuffer := &adapter.acked[0], &adapter.lost[0]
	adapter.OnCongestionEventEx(1, 0, acked[:1], lost)
	if &adapter.acked[0] != ackBuffer || &adapter.lost[0] != lossBuffer {
		t.Fatal("steady-state event conversion allocated new buffers")
	}
	adapter.OnCongestionEventEx(0, 0, nil, nil)
	if len(sender.acked) != 0 || len(sender.lost) != 0 || !sender.eventTime.IsZero() {
		t.Fatal("empty batch retained old packet events")
	}
}

func TestBBRBridgePreservesActualXrayWindowWhenDatagramSizeIncreases(t *testing.T) {
	adapter := newXrayBBR(1200)
	before := adapter.GetCongestionWindow()
	adapter.SetMaxDatagramSize(1400)
	after := adapter.GetCongestionWindow()
	if before <= 0 || after*1200 != before*1400 {
		t.Fatalf("BBR window did not follow datagram size: %d -> %d", before, after)
	}
	// Path migration uses the registered factory to rebuild BBR at the new MTU.
	replacement := newXrayBBR(1200)
	if got := replacement.GetCongestionWindow(); got != before {
		t.Fatalf("BBR factory window at the initial MTU: %d", got)
	}
}
