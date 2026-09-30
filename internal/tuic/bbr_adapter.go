package tuic

import (
	original "github.com/apernet/quic-go/congestion"
	originaltime "github.com/apernet/quic-go/monotime"
	fork "github.com/poise52/quic-go/congestion"
	forktime "github.com/poise52/quic-go/monotime"
	"github.com/xtls/xray-core/transport/internet/hysteria/congestion/bbr"
)

var _ fork.CongestionControlEx = (*xrayBBRAdapter)(nil)

type xrayBBRAdapter struct {
	sender original.CongestionControlEx
	acked  []original.AckedPacketInfo
	lost   []original.LostPacketInfo
}

func newXrayBBR(size fork.ByteCount) *xrayBBRAdapter {
	return &xrayBBRAdapter{sender: bbr.NewBbrSender(bbr.DefaultClock{}, original.ByteCount(size), bbr.ProfileStandard)}
}

func (a *xrayBBRAdapter) SetRTTStatsProvider(provider fork.RTTStatsProvider) {
	a.sender.SetRTTStatsProvider(provider)
}

// Each QUIC module has its own monotonic epoch. Convert through time.Time to
// preserve the instant, including the zero sentinel used by the pacer.
func (a *xrayBBRAdapter) TimeUntilSend(bytesInFlight fork.ByteCount) forktime.Time {
	return forktime.FromTime(a.sender.TimeUntilSend(original.ByteCount(bytesInFlight)).ToTime())
}

func (a *xrayBBRAdapter) HasPacingBudget(now forktime.Time) bool {
	return a.sender.HasPacingBudget(originaltime.FromTime(now.ToTime()))
}

func (a *xrayBBRAdapter) OnPacketSent(sentTime forktime.Time, bytesInFlight fork.ByteCount, packetNumber fork.PacketNumber, bytes fork.ByteCount, retransmittable bool) {
	a.sender.OnPacketSent(originaltime.FromTime(sentTime.ToTime()), original.ByteCount(bytesInFlight), original.PacketNumber(packetNumber), original.ByteCount(bytes), retransmittable)
}

func (a *xrayBBRAdapter) CanSend(bytesInFlight fork.ByteCount) bool {
	return a.sender.CanSend(original.ByteCount(bytesInFlight))
}

func (a *xrayBBRAdapter) MaybeExitSlowStart() { a.sender.MaybeExitSlowStart() }

func (a *xrayBBRAdapter) OnPacketAcked(number fork.PacketNumber, ackedBytes, priorInFlight fork.ByteCount, eventTime forktime.Time) {
	a.sender.OnPacketAcked(original.PacketNumber(number), original.ByteCount(ackedBytes), original.ByteCount(priorInFlight), originaltime.FromTime(eventTime.ToTime()))
}

func (a *xrayBBRAdapter) OnCongestionEvent(number fork.PacketNumber, lostBytes, priorInFlight fork.ByteCount) {
	a.sender.OnCongestionEvent(original.PacketNumber(number), original.ByteCount(lostBytes), original.ByteCount(priorInFlight))
}

func (a *xrayBBRAdapter) OnRetransmissionTimeout(retransmitted bool) {
	a.sender.OnRetransmissionTimeout(retransmitted)
}

func (a *xrayBBRAdapter) SetMaxDatagramSize(size fork.ByteCount) {
	a.sender.SetMaxDatagramSize(original.ByteCount(size))
}
func (a *xrayBBRAdapter) InSlowStart() bool { return a.sender.InSlowStart() }
func (a *xrayBBRAdapter) InRecovery() bool  { return a.sender.InRecovery() }
func (a *xrayBBRAdapter) GetCongestionWindow() fork.ByteCount {
	return fork.ByteCount(a.sender.GetCongestionWindow())
}

func (a *xrayBBRAdapter) OnCongestionEventEx(priorInFlight fork.ByteCount, eventTime forktime.Time, acked []fork.AckedPacketInfo, lost []fork.LostPacketInfo) {
	a.acked = resizeCongestionBatch(a.acked, len(acked))
	a.lost = resizeCongestionBatch(a.lost, len(lost))
	for i, packet := range acked {
		a.acked[i] = original.AckedPacketInfo{PacketNumber: original.PacketNumber(packet.PacketNumber), BytesAcked: original.ByteCount(packet.BytesAcked), ReceivedTime: originaltime.FromTime(packet.ReceivedTime.ToTime())}
	}
	for i, packet := range lost {
		a.lost[i] = original.LostPacketInfo{PacketNumber: original.PacketNumber(packet.PacketNumber), BytesLost: original.ByteCount(packet.BytesLost)}
	}
	a.sender.OnCongestionEventEx(original.ByteCount(priorInFlight), originaltime.FromTime(eventTime.ToTime()), a.acked, a.lost)
}

func resizeCongestionBatch[T any](batch []T, size int) []T {
	if cap(batch) < size {
		return make([]T, size)
	}
	return batch[:size]
}
