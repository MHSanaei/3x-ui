package bbr

import (
	"fmt"
	"math/rand"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/apernet/quic-go/congestion"
	"github.com/apernet/quic-go/monotime"

	"github.com/xtls/xray-core/transport/internet/hysteria/congestion/common"
)

// BbrSender implements BBR congestion control algorithm.  BBR aims to estimate
// the current available Bottleneck Bandwidth and RTT (hence the name), and
// regulates the pacing rate and the size of the congestion window based on
// those signals.
//
// BBR relies on pacing in order to function properly.  Do not use BBR when
// pacing is disabled.
//

const (
	minBps = 65536 // 64 KB/s

	invalidPacketNumber            = -1
	initialCongestionWindowPackets = 32
	minCongestionWindowPackets     = 4

	// Constants based on TCP defaults.
	// The minimum CWND to ensure delayed acks don't reduce bandwidth measurements.
	// Does not inflate the pacing rate.
	// The gain used for the STARTUP, equal to 2/ln(2).
	defaultHighGain = 2.885
	// The newly derived CWND gain for STARTUP, 2.
	derivedHighCWNDGain = 2.0

	debugEnv = "HYSTERIA_BBR_DEBUG"
)

// The cycle of gains used during the PROBE_BW stage.
var pacingGain = [...]float64{1.25, 0.75, 1.0, 1.0, 1.0, 1.0, 1.0, 1.0}

const (
	// The length of the gain cycle.
	gainCycleLength = len(pacingGain)
	// The size of the bandwidth filter window, in round-trips.
	bandwidthWindowSize = gainCycleLength + 2

	// The time after which the current min_rtt value expires.
	minRttExpiry = 10 * time.Second
	// The minimum time the connection can spend in PROBE_RTT mode.
	probeRttTime = 200 * time.Millisecond
	// If the bandwidth does not increase by the factor of |kStartupGrowthTarget|
	// within |kRoundTripsWithoutGrowthBeforeExitingStartup| rounds, the connection
	// will exit the STARTUP mode.
	startupGrowthTarget                         = 1.25
	roundTripsWithoutGrowthBeforeExitingStartup = int64(3)

	// Flag.
	defaultStartupFullLossCount  = 8
	quicBbr2DefaultLossThreshold = 0.02
)

type bbrMode int

const (
	// Startup phase of the connection.
	bbrModeStartup = iota
	// After achieving the highest possible bandwidth during the startup, lower
	// the pacing rate in order to drain the queue.
	bbrModeDrain
	// Cruising mode.
	bbrModeProbeBw
	// Temporarily slow down sending in order to empty the buffer and measure
	// the real minimum RTT.
	bbrModeProbeRtt
)

// Indicates how the congestion control limits the amount of bytes in flight.
type bbrRecoveryState int

const (
	// Do not limit.
	bbrRecoveryStateNotInRecovery = iota
	// Allow an extra outstanding byte for each byte acknowledged.
	bbrRecoveryStateConservation
	// Allow two extra outstanding bytes for each byte acknowledged (slow
	// start).
	bbrRecoveryStateGrowth
)

type Profile string

const (
	ProfileConservative Profile = "conservative"
	ProfileStandard     Profile = "standard"
	ProfileAggressive   Profile = "aggressive"
)

type profileConfig struct {
	highGain                            float64
	highCwndGain                        float64
	congestionWindowGainConstant        float64
	numStartupRtts                      int64
	drainToTarget                       bool
	detectOvershooting                  bool
	bytesLostMultiplier                 uint8
	enableAckAggregationStartup         bool
	expireAckAggregationStartup         bool
	enableOverestimateAvoidance         bool
	reduceExtraAckedOnBandwidthIncrease bool
}

func ParseProfile(profile string) (Profile, error) {
	switch normalized := strings.ToLower(profile); normalized {
	case "", string(ProfileStandard):
		return ProfileStandard, nil
	case string(ProfileConservative):
		return ProfileConservative, nil
	case string(ProfileAggressive):
		return ProfileAggressive, nil
	default:
		return "", fmt.Errorf("unsupported BBR profile %q", profile)
	}
}

func configForProfile(profile Profile) profileConfig {
	switch profile {
	case ProfileConservative:
		return profileConfig{
			highGain:                            2.25,
			highCwndGain:                        1.75,
			congestionWindowGainConstant:        1.75,
			numStartupRtts:                      2,
			drainToTarget:                       true,
			detectOvershooting:                  true,
			bytesLostMultiplier:                 1,
			enableOverestimateAvoidance:         true,
			reduceExtraAckedOnBandwidthIncrease: true,
		}
	case ProfileAggressive:
		return profileConfig{
			highGain:                     3.0,
			highCwndGain:                 2.25,
			congestionWindowGainConstant: 2.5,
			numStartupRtts:               4,
			bytesLostMultiplier:          2,
			enableAckAggregationStartup:  true,
			expireAckAggregationStartup:  true,
		}
	default:
		return profileConfig{
			highGain:                     defaultHighGain,
			highCwndGain:                 derivedHighCWNDGain,
			congestionWindowGainConstant: 2.0,
			numStartupRtts:               roundTripsWithoutGrowthBeforeExitingStartup,
			bytesLostMultiplier:          2,
		}
	}
}

type bbrSender struct {
	rttStats congestion.RTTStatsProvider
	clock    Clock
	pacer    *common.Pacer

	mode bbrMode

	// Bandwidth sampler provides BBR with the bandwidth measurements at
	// individual points.
	sampler *bandwidthSampler

	// The number of the round trips that have occurred during the connection.
	roundTripCount roundTripCount

	// The packet number of the most recently sent packet.
	lastSentPacket congestion.PacketNumber
	// Acknowledgement of any packet after |current_round_trip_end_| will cause
	// the round trip counter to advance.
	currentRoundTripEnd congestion.PacketNumber

	// Number of congestion events with some losses, in the current round.
	numLossEventsInRound uint64

	// Number of total bytes lost in the current round.
	bytesLostInRound congestion.ByteCount

	// The filter that tracks the maximum bandwidth over the multiple recent
	// round-trips.
	maxBandwidth *WindowedFilter[Bandwidth, roundTripCount]

	// Minimum RTT estimate.  Automatically expires within 10 seconds (and
	// triggers PROBE_RTT mode) if no new value is sampled during that period.
	minRtt time.Duration
	// The time at which the current value of |min_rtt_| was assigned.
	minRttTimestamp monotime.Time

	// The maximum allowed number of bytes in flight.
	congestionWindow congestion.ByteCount

	// The initial value of the |congestion_window_|.
	initialCongestionWindow congestion.ByteCount

	// The largest value the |congestion_window_| can achieve.
	maxCongestionWindow congestion.ByteCount

	// The smallest value the |congestion_window_| can achieve.
	minCongestionWindow congestion.ByteCount

	// The BBR profile used by the sender.
	profile Profile

	// The pacing gain applied during the STARTUP phase.
	highGain float64

	// The CWND gain applied during the STARTUP phase.
	highCwndGain float64

	// The pacing gain applied during the DRAIN phase.
	drainGain float64

	// The current pacing rate of the connection.
	pacingRate Bandwidth

	// The gain currently applied to the pacing rate.
	pacingGain float64
	// The gain currently applied to the congestion window.
	congestionWindowGain float64

	// The gain used for the congestion window during PROBE_BW.  Latched from
	// quic_bbr_cwnd_gain flag.
	congestionWindowGainConstant float64
	// The number of RTTs to stay in STARTUP mode.  Defaults to 3.
	numStartupRtts int64

	// Number of round-trips in PROBE_BW mode, used for determining the current
	// pacing gain cycle.
	cycleCurrentOffset int
	// The time at which the last pacing gain cycle was started.
	lastCycleStart monotime.Time

	// Indicates whether the connection has reached the full bandwidth mode.
	isAtFullBandwidth bool
	// Number of rounds during which there was no significant bandwidth increase.
	roundsWithoutBandwidthGain int64
	// The bandwidth compared to which the increase is measured.
	bandwidthAtLastRound Bandwidth

	// Set to true upon exiting quiescence.
	exitingQuiescence bool

	// Time at which PROBE_RTT has to be exited.  Setting it to zero indicates
	// that the time is yet unknown as the number of packets in flight has not
	// reached the required value.
	exitProbeRttAt monotime.Time
	// Indicates whether a round-trip has passed since PROBE_RTT became active.
	probeRttRoundPassed bool

	// Indicates whether the most recent bandwidth sample was marked as
	// app-limited.
	lastSampleIsAppLimited bool
	// Indicates whether any non app-limited samples have been recorded.
	hasNoAppLimitedSample bool

	// Current state of recovery.
	recoveryState bbrRecoveryState
	// Receiving acknowledgement of a packet after |end_recovery_at_| will cause
	// BBR to exit the recovery mode.  A value above zero indicates at least one
	// loss has been detected, so it must not be set back to zero.
	endRecoveryAt congestion.PacketNumber
	// A window used to limit the number of bytes in flight during loss recovery.
	recoveryWindow congestion.ByteCount
	// If true, consider all samples in recovery app-limited.
	isAppLimitedRecovery bool // not used

	// When true, pace at 1.5x and disable packet conservation in STARTUP.
	slowerStartup bool // not used
	// When true, disables packet conservation in STARTUP.
	rateBasedStartup bool // not used

	// When true, add the most recent ack aggregation measurement during STARTUP.
	enableAckAggregationDuringStartup bool
	// When true, expire the windowed ack aggregation values in STARTUP when
	// bandwidth increases more than 25%.
	expireAckAggregationInStartup bool

	// If true, will not exit low gain mode until bytes_in_flight drops below BDP
	// or it's time for high gain mode.
	drainToTarget bool

	// If true, slow down pacing rate in STARTUP when overshooting is detected.
	detectOvershooting bool
	// Bytes lost while detect_overshooting_ is true.
	bytesLostWhileDetectingOvershooting congestion.ByteCount
	// Slow down pacing rate if
	// bytes_lost_while_detecting_overshooting_ *
	// bytes_lost_multiplier_while_detecting_overshooting_ > IW.
	bytesLostMultiplierWhileDetectingOvershooting uint8
	// When overshooting is detected, do not drop pacing_rate_ below this value /
	// min_rtt.
	cwndToCalculateMinPacingRate congestion.ByteCount

	// Max congestion window when adjusting network parameters.
	maxCongestionWindowWithNetworkParametersAdjusted congestion.ByteCount // not used

	// Params.
	maxDatagramSize congestion.ByteCount
	// Recorded on packet sent. equivalent |unacked_packets_->bytes_in_flight()|
	bytesInFlight congestion.ByteCount

	debug bool
}

var _ congestion.CongestionControl = &bbrSender{}

func NewBbrSender(
	clock Clock,
	initialMaxDatagramSize congestion.ByteCount,
	profile Profile,
) *bbrSender {
	return newBbrSender(
		clock,
		initialMaxDatagramSize,
		initialCongestionWindowPackets*initialMaxDatagramSize,
		congestion.MaxCongestionWindowPackets*initialMaxDatagramSize,
		profile,
	)
}

func newBbrSender(
	clock Clock,
	initialMaxDatagramSize,
	initialCongestionWindow,
	initialMaxCongestionWindow congestion.ByteCount,
	profile Profile,
) *bbrSender {
	debug, _ := strconv.ParseBool(os.Getenv(debugEnv))
	b := &bbrSender{
		clock:                        clock,
		mode:                         bbrModeStartup,
		sampler:                      newBandwidthSampler(roundTripCount(bandwidthWindowSize)),
		lastSentPacket:               invalidPacketNumber,
		currentRoundTripEnd:          invalidPacketNumber,
		maxBandwidth:                 NewWindowedFilter(roundTripCount(bandwidthWindowSize), MaxFilter[Bandwidth]),
		congestionWindow:             initialCongestionWindow,
		initialCongestionWindow:      initialCongestionWindow,
		maxCongestionWindow:          initialMaxCongestionWindow,
		minCongestionWindow:          minCongestionWindowForMaxDatagramSize(initialMaxDatagramSize),
		profile:                      ProfileStandard,
		highGain:                     defaultHighGain,
		highCwndGain:                 derivedHighCWNDGain,
		drainGain:                    1.0 / defaultHighGain,
		pacingGain:                   1.0,
		congestionWindowGain:         1.0,
		congestionWindowGainConstant: 2.0,
		numStartupRtts:               roundTripsWithoutGrowthBeforeExitingStartup,
		recoveryState:                bbrRecoveryStateNotInRecovery,
		endRecoveryAt:                invalidPacketNumber,
		recoveryWindow:               initialMaxCongestionWindow,
		bytesLostMultiplierWhileDetectingOvershooting:    2,
		cwndToCalculateMinPacingRate:                     initialCongestionWindow,
		maxCongestionWindowWithNetworkParametersAdjusted: initialMaxCongestionWindow,
		maxDatagramSize: initialMaxDatagramSize,
		debug:           debug,
	}
	b.pacer = common.NewPacer(b.bandwidthForPacer)
	b.applyProfile(profile)
	if b.debug {
		b.debugPrint("Profile: %s", b.profile)
	}

	b.enterStartupMode(b.clock.Now())

	return b
}

func (b *bbrSender) applyProfile(profile Profile) {
	if profile == "" {
		profile = ProfileStandard
	}
	cfg := configForProfile(profile)
	b.profile = profile
	b.highGain = cfg.highGain
	b.highCwndGain = cfg.highCwndGain
	b.drainGain = 1.0 / cfg.highGain
	b.congestionWindowGainConstant = cfg.congestionWindowGainConstant
	b.numStartupRtts = cfg.numStartupRtts
	b.drainToTarget = cfg.drainToTarget
	b.detectOvershooting = cfg.detectOvershooting
	b.bytesLostMultiplierWhileDetectingOvershooting = cfg.bytesLostMultiplier
	b.enableAckAggregationDuringStartup = cfg.enableAckAggregationStartup
	b.expireAckAggregationInStartup = cfg.expireAckAggregationStartup
	if cfg.enableOverestimateAvoidance {
		b.sampler.EnableOverestimateAvoidance()
	}
	b.sampler.SetReduceExtraAckedOnBandwidthIncrease(cfg.reduceExtraAckedOnBandwidthIncrease)
}

func minCongestionWindowForMaxDatagramSize(maxDatagramSize congestion.ByteCount) congestion.ByteCount {
	return minCongestionWindowPackets * maxDatagramSize
}

func scaleByteWindowForDatagramSize(window, oldMaxDatagramSize, newMaxDatagramSize congestion.ByteCount) congestion.ByteCount {
	if oldMaxDatagramSize == newMaxDatagramSize {
		return window
	}
	return congestion.ByteCount(uint64(window) * uint64(newMaxDatagramSize) / uint64(oldMaxDatagramSize))
}

func (b *bbrSender) rescalePacketSizedWindows(maxDatagramSize congestion.ByteCount) {
	oldMaxDatagramSize := b.maxDatagramSize
	b.maxDatagramSize = maxDatagramSize
	b.initialCongestionWindow = scaleByteWindowForDatagramSize(b.initialCongestionWindow, oldMaxDatagramSize, maxDatagramSize)
	b.maxCongestionWindow = scaleByteWindowForDatagramSize(b.maxCongestionWindow, oldMaxDatagramSize, maxDatagramSize)
	b.minCongestionWindow = minCongestionWindowForMaxDatagramSize(maxDatagramSize)
	b.cwndToCalculateMinPacingRate = scaleByteWindowForDatagramSize(b.cwndToCalculateMinPacingRate, oldMaxDatagramSize, maxDatagramSize)
	b.maxCongestionWindowWithNetworkParametersAdjusted = scaleByteWindowForDatagramSize(
		b.maxCongestionWindowWithNetworkParametersAdjusted,
		oldMaxDatagramSize,
		maxDatagramSize,
	)
}

func (b *bbrSender) SetRTTStatsProvider(provider congestion.RTTStatsProvider) {
	b.rttStats = provider
}

// TimeUntilSend implements the SendAlgorithm interface.
func (b *bbrSender) TimeUntilSend(bytesInFlight congestion.ByteCount) monotime.Time {
	return b.pacer.TimeUntilSend()
}

// HasPacingBudget implements the SendAlgorithm interface.
func (b *bbrSender) HasPacingBudget(now monotime.Time) bool {
	return b.pacer.Budget(now) >= b.maxDatagramSize
}

// OnPacketSent implements the SendAlgorithm interface.
func (b *bbrSender) OnPacketSent(
	sentTime monotime.Time,
	bytesInFlight congestion.ByteCount,
	packetNumber congestion.PacketNumber,
	bytes congestion.ByteCount,
	isRetransmittable bool,
) {
	b.pacer.SentPacket(sentTime, bytes)

	b.lastSentPacket = packetNumber
	b.bytesInFlight = bytesInFlight

	if bytesInFlight == 0 {
		b.exitingQuiescence = true
	}

	b.sampler.OnPacketSent(sentTime, packetNumber, bytes, bytesInFlight, isRetransmittable)
}

// CanSend implements the SendAlgorithm interface.
func (b *bbrSender) CanSend(bytesInFlight congestion.ByteCount) bool {
	return bytesInFlight < b.GetCongestionWindow()
}

// MaybeExitSlowStart implements the SendAlgorithm interface.
func (b *bbrSender) MaybeExitSlowStart() {
	// Do nothing
}

// OnPacketAcked implements the SendAlgorithm interface.
func (b *bbrSender) OnPacketAcked(number congestion.PacketNumber, ackedBytes, priorInFlight congestion.ByteCount, eventTime monotime.Time) {
	// Do nothing.
}

// OnPacketLost implements the SendAlgorithm interface.
func (b *bbrSender) OnPacketLost(number congestion.PacketNumber, lostBytes, priorInFlight congestion.ByteCount) {
	// Do nothing.
}

// OnRetransmissionTimeout implements the SendAlgorithm interface.
func (b *bbrSender) OnRetransmissionTimeout(packetsRetransmitted bool) {
	// Do nothing.
}

// SetMaxDatagramSize implements the SendAlgorithm interface.
func (b *bbrSender) SetMaxDatagramSize(s congestion.ByteCount) {
	if b.debug {
		b.debugPrint("Max Datagram Size: %d", s)
	}
	if s < b.maxDatagramSize {
		panic(fmt.Sprintf("congestion BUG: decreased max datagram size from %d to %d", b.maxDatagramSize, s))
	}
	oldMinCongestionWindow := b.minCongestionWindow
	oldInitialCongestionWindow := b.initialCongestionWindow
	b.rescalePacketSizedWindows(s)
	switch b.congestionWindow {
	case oldMinCongestionWindow:
		b.congestionWindow = b.minCongestionWindow
	case oldInitialCongestionWindow:
		b.congestionWindow = b.initialCongestionWindow
	default:
		b.congestionWindow = min(b.maxCongestionWindow, max(b.congestionWindow, b.minCongestionWindow))
	}
	b.recoveryWindow = min(b.maxCongestionWindow, max(b.recoveryWindow, b.minCongestionWindow))
	b.pacer.SetMaxDatagramSize(s)
}

// InSlowStart implements the SendAlgorithmWithDebugInfos interface.
func (b *bbrSender) InSlowStart() bool {
	return b.mode == bbrModeStartup
}

// InRecovery implements the SendAlgorithmWithDebugInfos interface.
func (b *bbrSender) InRecovery() bool {
	return b.recoveryState != bbrRecoveryStateNotInRecovery
}

// GetCongestionWindow implements the SendAlgorithmWithDebugInfos interface.
func (b *bbrSender) GetCongestionWindow() congestion.ByteCount {
	if b.mode == bbrModeProbeRtt {
		return b.probeRttCongestionWindow()
	}

	if b.InRecovery() {
		return min(b.congestionWindow, b.recoveryWindow)
	}

	return b.congestionWindow
}

func (b *bbrSender) OnCongestionEvent(number congestion.PacketNumber, lostBytes, priorInFlight congestion.ByteCount) {
	// Do nothing.
}

func (b *bbrSender) OnCongestionEventEx(priorInFlight congestion.ByteCount, eventTime monotime.Time, ackedPackets []congestion.AckedPacketInfo, lostPackets []congestion.LostPacketInfo) {
	totalBytesAckedBefore := b.sampler.TotalBytesAcked()
	totalBytesLostBefore := b.sampler.TotalBytesLost()

	var isRoundStart, minRttExpired bool
	var excessAcked, bytesLost congestion.ByteCount

	// The send state of the largest packet in acked_packets, unless it is
	// empty. If acked_packets is empty, it's the send state of the largest
	// packet in lost_packets.
	var lastPacketSendState sendTimeState

	b.maybeAppLimited(priorInFlight)

	// Update bytesInFlight
	b.bytesInFlight = priorInFlight
	for _, p := range ackedPackets {
		b.bytesInFlight -= p.BytesAcked
	}
	for _, p := range lostPackets {
		b.bytesInFlight -= p.BytesLost
	}

	if len(ackedPackets) != 0 {
		lastAckedPacket := ackedPackets[len(ackedPackets)-1].PacketNumber
		isRoundStart = b.updateRoundTripCounter(lastAckedPacket)
		b.updateRecoveryState(lastAckedPacket, len(lostPackets) != 0, isRoundStart)
	}

	sample := b.sampler.OnCongestionEvent(eventTime,
		ackedPackets, lostPackets, b.maxBandwidth.GetBest(), infBandwidth, b.roundTripCount)
	if sample.lastPacketSendState.isValid {
		b.lastSampleIsAppLimited = sample.lastPacketSendState.isAppLimited
		b.hasNoAppLimitedSample = b.hasNoAppLimitedSample || !b.lastSampleIsAppLimited
	}
	// Avoid updating |max_bandwidth_| if a) this is a loss-only event, or b) all
	// packets in |acked_packets| did not generate valid samples. (e.g. ack of
	// ack-only packets). In both cases, sampler_.total_bytes_acked() will not
	// change.
	if totalBytesAckedBefore != b.sampler.TotalBytesAcked() {
		if !sample.sampleIsAppLimited || sample.sampleMaxBandwidth > b.maxBandwidth.GetBest() {
			b.maxBandwidth.Update(sample.sampleMaxBandwidth, b.roundTripCount)
		}
	}

	if sample.sampleRtt != infRTT {
		minRttExpired = b.maybeUpdateMinRtt(eventTime, sample.sampleRtt)
	}
	bytesLost = b.sampler.TotalBytesLost() - totalBytesLostBefore

	excessAcked = sample.extraAcked
	lastPacketSendState = sample.lastPacketSendState

	if len(lostPackets) != 0 {
		b.numLossEventsInRound++
		b.bytesLostInRound += bytesLost
	}

	// Handle logic specific to PROBE_BW mode.
	if b.mode == bbrModeProbeBw {
		b.updateGainCyclePhase(eventTime, priorInFlight, len(lostPackets) != 0)
	}

	// Handle logic specific to STARTUP and DRAIN modes.
	if isRoundStart && !b.isAtFullBandwidth {
		b.checkIfFullBandwidthReached(&lastPacketSendState)
	}

	b.maybeExitStartupOrDrain(eventTime)

	// Handle logic specific to PROBE_RTT.
	b.maybeEnterOrExitProbeRtt(eventTime, isRoundStart, minRttExpired)

	// Calculate number of packets acked and lost.
	bytesAcked := b.sampler.TotalBytesAcked() - totalBytesAckedBefore

	// After the model is updated, recalculate the pacing rate and congestion
	// window.
	b.calculatePacingRate(bytesLost)
	b.calculateCongestionWindow(bytesAcked, excessAcked)
	b.calculateRecoveryWindow(bytesAcked, bytesLost)

	// Cleanup internal state.
	// This is where we clean up obsolete (acked or lost) packets from the bandwidth sampler.
	// The "least unacked" should actually be FirstOutstanding, but since we are not passing
	// that through OnCongestionEventEx, we will only do an estimate using acked/lost packets
	// for now. Because of fast retransmission, they should differ by no more than 2 packets.
	// (this is controlled by packetThreshold in quic-go's sentPacketHandler)
	var leastUnacked congestion.PacketNumber
	if len(ackedPackets) != 0 {
		leastUnacked = ackedPackets[len(ackedPackets)-1].PacketNumber - 2
	} else {
		leastUnacked = lostPackets[len(lostPackets)-1].PacketNumber + 1
	}
	b.sampler.RemoveObsoletePackets(leastUnacked)

	if isRoundStart {
		b.numLossEventsInRound = 0
		b.bytesLostInRound = 0
	}
}

func (b *bbrSender) PacingRate() Bandwidth {
	if b.pacingRate == 0 {
		return Bandwidth(b.highGain * float64(
			BandwidthFromDelta(b.initialCongestionWindow, b.getMinRtt())))
	}

	return b.pacingRate
}

// Sets the CWND gain used in STARTUP.  Must be greater than 1.
func (b *bbrSender) setHighCwndGain(highCwndGain float64) {
	b.highCwndGain = highCwndGain
	if b.mode == bbrModeStartup {
		b.congestionWindowGain = highCwndGain
	}
}

// Get the current bandwidth estimate. Note that Bandwidth is in bits per second.
func (b *bbrSender) bandwidthEstimate() Bandwidth {
	return b.maxBandwidth.GetBest()
}

func (b *bbrSender) bandwidthForPacer() congestion.ByteCount {
	bps := congestion.ByteCount(float64(b.PacingRate()) / float64(BytesPerSecond))
	if bps < minBps {
		// We need to make sure that the bandwidth value for pacer is never zero,
		// otherwise it will go into an edge case where HasPacingBudget = false
		// but TimeUntilSend is before, causing the quic-go send loop to go crazy and get stuck.
		return minBps
	}
	return bps
}

// Returns the current estimate of the RTT of the connection.  Outside of the
// edge cases, this is minimum RTT.
func (b *bbrSender) getMinRtt() time.Duration {
	if b.minRtt != 0 {
		return b.minRtt
	}
	// min_rtt could be available if the handshake packet gets neutered then
	// gets acknowledged. This could only happen for QUIC crypto where we do not
	// drop keys.
	minRtt := b.rttStats.MinRTT()
	if minRtt == 0 {
		return 100 * time.Millisecond
	} else {
		return minRtt
	}
}

// Computes the target congestion window using the specified gain.
func (b *bbrSender) getTargetCongestionWindow(gain float64) congestion.ByteCount {
	bdp := bdpFromRttAndBandwidth(b.getMinRtt(), b.bandwidthEstimate())
	congestionWindow := congestion.ByteCount(gain * float64(bdp))

	// BDP estimate will be zero if no bandwidth samples are available yet.
	if congestionWindow == 0 {
		congestionWindow = congestion.ByteCount(gain * float64(b.initialCongestionWindow))
	}

	return max(congestionWindow, b.minCongestionWindow)
}

// The target congestion window during PROBE_RTT.
func (b *bbrSender) probeRttCongestionWindow() congestion.ByteCount {
	return b.minCongestionWindow
}

func (b *bbrSender) maybeUpdateMinRtt(now monotime.Time, sampleMinRtt time.Duration) bool {
	// Do not expire min_rtt if none was ever available.
	minRttExpired := b.minRtt != 0 && now.After(b.minRttTimestamp.Add(minRttExpiry))
	if minRttExpired || sampleMinRtt < b.minRtt || b.minRtt == 0 {
		b.minRtt = sampleMinRtt
		b.minRttTimestamp = now
	}

	return minRttExpired
}

// Enters the STARTUP mode.
func (b *bbrSender) enterStartupMode(now monotime.Time) {
	b.mode = bbrModeStartup
	// b.maybeTraceStateChange(logging.CongestionStateStartup)
	b.pacingGain = b.highGain
	b.congestionWindowGain = b.highCwndGain

	if b.debug {
		b.debugPrint("Phase: STARTUP")
	}
}

// Enters the PROBE_BW mode.
func (b *bbrSender) enterProbeBandwidthMode(now monotime.Time) {
	b.mode = bbrModeProbeBw
	// b.maybeTraceStateChange(logging.CongestionStateProbeBw)
	b.congestionWindowGain = b.congestionWindowGainConstant

	// Pick a random offset for the gain cycle out of {0, 2..7} range. 1 is
	// excluded because in that case increased gain and decreased gain would not
	// follow each other.
	b.cycleCurrentOffset = int(rand.Int31n(congestion.PacketsPerConnectionID)) % (gainCycleLength - 1)
	if b.cycleCurrentOffset >= 1 {
		b.cycleCurrentOffset += 1
	}

	b.lastCycleStart = now
	b.pacingGain = pacingGain[b.cycleCurrentOffset]

	if b.debug {
		b.debugPrint("Phase: PROBE_BW")
	}
}

// Updates the round-trip counter if a round-trip has passed.  Returns true if
// the counter has been advanced.
func (b *bbrSender) updateRoundTripCounter(lastAckedPacket congestion.PacketNumber) bool {
	if b.currentRoundTripEnd == invalidPacketNumber || lastAckedPacket > b.currentRoundTripEnd {
		b.roundTripCount++
		b.currentRoundTripEnd = b.lastSentPacket
		return true
	}
	return false
}

// Updates the current gain used in PROBE_BW mode.
func (b *bbrSender) updateGainCyclePhase(now monotime.Time, priorInFlight congestion.ByteCount, hasLosses bool) {
	// In most cases, the cycle is advanced after an RTT passes.
	shouldAdvanceGainCycling := now.After(b.lastCycleStart.Add(b.getMinRtt()))
	// If the pacing gain is above 1.0, the connection is trying to probe the
	// bandwidth by increasing the number of bytes in flight to at least
	// pacing_gain * BDP.  Make sure that it actually reaches the target, as long
	// as there are no losses suggesting that the buffers are not able to hold
	// that much.
	if b.pacingGain > 1.0 && !hasLosses && priorInFlight < b.getTargetCongestionWindow(b.pacingGain) {
		shouldAdvanceGainCycling = false
	}

	// If pacing gain is below 1.0, the connection is trying to drain the extra
	// queue which could have been incurred by probing prior to it.  If the number
	// of bytes in flight falls down to the estimated BDP value earlier, conclude
	// that the queue has been successfully drained and exit this cycle early.
	if b.pacingGain < 1.0 && b.bytesInFlight <= b.getTargetCongestionWindow(1) {
		shouldAdvanceGainCycling = true
	}

	if shouldAdvanceGainCycling {
		b.cycleCurrentOffset = (b.cycleCurrentOffset + 1) % gainCycleLength
		b.lastCycleStart = now
		// Stay in low gain mode until the target BDP is hit.
		// Low gain mode will be exited immediately when the target BDP is achieved.
		if b.drainToTarget && b.pacingGain < 1 &&
			pacingGain[b.cycleCurrentOffset] == 1 &&
			b.bytesInFlight > b.getTargetCongestionWindow(1) {
			return
		}
		b.pacingGain = pacingGain[b.cycleCurrentOffset]
	}
}

// Tracks for how many round-trips the bandwidth has not increased
// significantly.
func (b *bbrSender) checkIfFullBandwidthReached(lastPacketSendState *sendTimeState) {
	if b.lastSampleIsAppLimited {
		return
	}

	target := Bandwidth(float64(b.bandwidthAtLastRound) * startupGrowthTarget)
	if b.bandwidthEstimate() >= target {
		b.bandwidthAtLastRound = b.bandwidthEstimate()
		b.roundsWithoutBandwidthGain = 0
		if b.expireAckAggregationInStartup {
			// Expire old excess delivery measurements now that bandwidth increased.
			b.sampler.ResetMaxAckHeightTracker(0, b.roundTripCount)
		}
		return
	}

	b.roundsWithoutBandwidthGain++
	if b.roundsWithoutBandwidthGain >= b.numStartupRtts ||
		b.shouldExitStartupDueToLoss(lastPacketSendState) {
		b.isAtFullBandwidth = true
	}
}

func (b *bbrSender) maybeAppLimited(bytesInFlight congestion.ByteCount) {
	if bytesInFlight < b.getTargetCongestionWindow(1) {
		b.sampler.OnAppLimited()
	}
}

// Transitions from STARTUP to DRAIN and from DRAIN to PROBE_BW if
// appropriate.
func (b *bbrSender) maybeExitStartupOrDrain(now monotime.Time) {
	if b.mode == bbrModeStartup && b.isAtFullBandwidth {
		b.mode = bbrModeDrain
		// b.maybeTraceStateChange(logging.CongestionStateDrain)
		b.pacingGain = b.drainGain
		b.congestionWindowGain = b.highCwndGain

		if b.debug {
			b.debugPrint("Phase: DRAIN")
		}
	}
	if b.mode == bbrModeDrain && b.bytesInFlight <= b.getTargetCongestionWindow(1) {
		b.enterProbeBandwidthMode(now)
	}
}

// Decides whether to enter or exit PROBE_RTT.
func (b *bbrSender) maybeEnterOrExitProbeRtt(now monotime.Time, isRoundStart, minRttExpired bool) {
	if minRttExpired && !b.exitingQuiescence && b.mode != bbrModeProbeRtt {
		b.mode = bbrModeProbeRtt
		// b.maybeTraceStateChange(logging.CongestionStateProbRtt)
		b.pacingGain = 1.0
		// Do not decide on the time to exit PROBE_RTT until the |bytes_in_flight|
		// is at the target small value.
		b.exitProbeRttAt = 0

		if b.debug {
			b.debugPrint("BandwidthEstimate: %s, CongestionWindowGain: %.2f, PacingGain: %.2f, PacingRate: %s",
				formatSpeed(b.bandwidthEstimate()), b.congestionWindowGain, b.pacingGain, formatSpeed(b.PacingRate()))
			b.debugPrint("Phase: PROBE_RTT")
		}
	}

	if b.mode == bbrModeProbeRtt {
		b.sampler.OnAppLimited()
		// b.maybeTraceStateChange(logging.CongestionStateApplicationLimited)

		if b.exitProbeRttAt.IsZero() {
			// If the window has reached the appropriate size, schedule exiting
			// PROBE_RTT.  The CWND during PROBE_RTT is kMinimumCongestionWindow, but
			// we allow an extra packet since QUIC checks CWND before sending a
			// packet.
			if b.bytesInFlight < b.probeRttCongestionWindow()+congestion.MaxPacketBufferSize {
				b.exitProbeRttAt = now.Add(probeRttTime)
				b.probeRttRoundPassed = false
			}
		} else {
			if isRoundStart {
				b.probeRttRoundPassed = true
			}
			if now.Sub(b.exitProbeRttAt) >= 0 && b.probeRttRoundPassed {
				b.minRttTimestamp = now
				if b.debug {
					b.debugPrint("MinRTT: %s", b.getMinRtt())
				}
				if !b.isAtFullBandwidth {
					b.enterStartupMode(now)
				} else {
					b.enterProbeBandwidthMode(now)
				}
			}
		}
	}

	b.exitingQuiescence = false
}

// Determines whether BBR needs to enter, exit or advance state of the
// recovery.
func (b *bbrSender) updateRecoveryState(lastAckedPacket congestion.PacketNumber, hasLosses, isRoundStart bool) {
	// Disable recovery in startup, if loss-based exit is enabled.
	if !b.isAtFullBandwidth {
		return
	}

	// Exit recovery when there are no losses for a round.
	if hasLosses {
		b.endRecoveryAt = b.lastSentPacket
	}

	switch b.recoveryState {
	case bbrRecoveryStateNotInRecovery:
		if hasLosses {
			b.recoveryState = bbrRecoveryStateConservation
			// This will cause the |recovery_window_| to be set to the correct
			// value in CalculateRecoveryWindow().
			b.recoveryWindow = 0
			// Since the conservation phase is meant to be lasting for a whole
			// round, extend the current round as if it were started right now.
			b.currentRoundTripEnd = b.lastSentPacket
		}
	case bbrRecoveryStateConservation:
		if isRoundStart {
			b.recoveryState = bbrRecoveryStateGrowth
		}
		fallthrough
	case bbrRecoveryStateGrowth:
		// Exit recovery if appropriate.
		if !hasLosses && lastAckedPacket > b.endRecoveryAt {
			b.recoveryState = bbrRecoveryStateNotInRecovery
		}
	}
}

// Determines the appropriate pacing rate for the connection.
func (b *bbrSender) calculatePacingRate(bytesLost congestion.ByteCount) {
	if b.bandwidthEstimate() == 0 {
		return
	}

	targetRate := Bandwidth(b.pacingGain * float64(b.bandwidthEstimate()))
	if b.isAtFullBandwidth {
		b.pacingRate = targetRate
		return
	}

	// Pace at the rate of initial_window / RTT as soon as RTT measurements are
	// available.
	if b.pacingRate == 0 && b.rttStats.MinRTT() != 0 {
		b.pacingRate = BandwidthFromDelta(b.initialCongestionWindow, b.rttStats.MinRTT())
		return
	}

	if b.detectOvershooting {
		b.bytesLostWhileDetectingOvershooting += bytesLost
		// Check for overshooting with network parameters adjusted when pacing rate
		// > target_rate and loss has been detected.
		if b.pacingRate > targetRate && b.bytesLostWhileDetectingOvershooting > 0 {
			if b.hasNoAppLimitedSample ||
				b.bytesLostWhileDetectingOvershooting*congestion.ByteCount(b.bytesLostMultiplierWhileDetectingOvershooting) > b.initialCongestionWindow {
				// We are fairly sure overshoot happens if 1) there is at least one
				// non app-limited bw sample or 2) half of IW gets lost. Slow pacing
				// rate.
				b.pacingRate = max(targetRate, BandwidthFromDelta(b.cwndToCalculateMinPacingRate, b.rttStats.MinRTT()))
				b.bytesLostWhileDetectingOvershooting = 0
				b.detectOvershooting = false
			}
		}
	}

	// Do not decrease the pacing rate during startup.
	b.pacingRate = max(b.pacingRate, targetRate)
}

// Determines the appropriate congestion window for the connection.
func (b *bbrSender) calculateCongestionWindow(bytesAcked, excessAcked congestion.ByteCount) {
	if b.mode == bbrModeProbeRtt {
		return
	}

	targetWindow := b.getTargetCongestionWindow(b.congestionWindowGain)
	if b.isAtFullBandwidth {
		// Add the max recently measured ack aggregation to CWND.
		targetWindow += b.sampler.MaxAckHeight()
	} else if b.enableAckAggregationDuringStartup {
		// Add the most recent excess acked.  Because CWND never decreases in
		// STARTUP, this will automatically create a very localized max filter.
		targetWindow += excessAcked
	}

	// Instead of immediately setting the target CWND as the new one, BBR grows
	// the CWND towards |target_window| by only increasing it |bytes_acked| at a
	// time.
	if b.isAtFullBandwidth {
		b.congestionWindow = min(targetWindow, b.congestionWindow+bytesAcked)
	} else if b.congestionWindow < targetWindow ||
		b.sampler.TotalBytesAcked() < b.initialCongestionWindow {
		// If the connection is not yet out of startup phase, do not decrease the
		// window.
		b.congestionWindow += bytesAcked
	}

	// Enforce the limits on the congestion window.
	b.congestionWindow = max(b.congestionWindow, b.minCongestionWindow)
	b.congestionWindow = min(b.congestionWindow, b.maxCongestionWindow)
}

// Determines the appropriate window that constrains the in-flight during recovery.
func (b *bbrSender) calculateRecoveryWindow(bytesAcked, bytesLost congestion.ByteCount) {
	if b.recoveryState == bbrRecoveryStateNotInRecovery {
		return
	}

	// Set up the initial recovery window.
	if b.recoveryWindow == 0 {
		b.recoveryWindow = b.bytesInFlight + bytesAcked
		b.recoveryWindow = max(b.minCongestionWindow, b.recoveryWindow)
		return
	}

	// Remove losses from the recovery window, while accounting for a potential
	// integer underflow.
	if b.recoveryWindow >= bytesLost {
		b.recoveryWindow = b.recoveryWindow - bytesLost
	} else {
		b.recoveryWindow = b.maxDatagramSize
	}

	// In CONSERVATION mode, just subtracting losses is sufficient.  In GROWTH,
	// release additional |bytes_acked| to achieve a slow-start-like behavior.
	if b.recoveryState == bbrRecoveryStateGrowth {
		b.recoveryWindow += bytesAcked
	}

	// Always allow sending at least |bytes_acked| in response.
	b.recoveryWindow = max(b.recoveryWindow, b.bytesInFlight+bytesAcked)
	b.recoveryWindow = max(b.minCongestionWindow, b.recoveryWindow)
}

// Return whether we should exit STARTUP due to excessive loss.
func (b *bbrSender) shouldExitStartupDueToLoss(lastPacketSendState *sendTimeState) bool {
	if b.numLossEventsInRound < defaultStartupFullLossCount || !lastPacketSendState.isValid {
		return false
	}

	inflightAtSend := lastPacketSendState.bytesInFlight

	if inflightAtSend > 0 && b.bytesLostInRound > 0 {
		if b.bytesLostInRound > congestion.ByteCount(float64(inflightAtSend)*quicBbr2DefaultLossThreshold) {
			return true
		}
		return false
	}
	return false
}

func (b *bbrSender) debugPrint(format string, a ...any) {
	fmt.Printf("[BBRSender] [%s] %s\n",
		time.Now().Format("15:04:05"),
		fmt.Sprintf(format, a...))
}

func bdpFromRttAndBandwidth(rtt time.Duration, bandwidth Bandwidth) congestion.ByteCount {
	return congestion.ByteCount(rtt) * congestion.ByteCount(bandwidth) / congestion.ByteCount(BytesPerSecond) / congestion.ByteCount(time.Second)
}

func GetInitialPacketSize(addr net.Addr) congestion.ByteCount {
	// If this is not a UDP address, we don't know anything about the MTU.
	// Use the minimum size of an Initial packet as the max packet size.
	if _, ok := addr.(*net.UDPAddr); ok {
		return congestion.InitialPacketSize
	} else {
		return congestion.MinInitialPacketSize
	}
}

func formatSpeed(bw Bandwidth) string {
	bwf := float64(bw)
	units := []string{"bps", "Kbps", "Mbps", "Gbps"}
	unitIndex := 0
	for bwf > 1000 && unitIndex < len(units)-1 {
		bwf /= 1000
		unitIndex++
	}
	return fmt.Sprintf("%.2f %s", bwf, units[unitIndex])
}
