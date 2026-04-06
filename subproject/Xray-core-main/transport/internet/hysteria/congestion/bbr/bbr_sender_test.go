package bbr

import (
	"testing"

	"github.com/apernet/quic-go/congestion"
	"github.com/stretchr/testify/require"
)

func TestSetMaxDatagramSizeRescalesPacketSizedWindows(t *testing.T) {
	const oldMaxDatagramSize = congestion.ByteCount(1000)
	const newMaxDatagramSize = congestion.ByteCount(1400)
	const initialCongestionWindowPackets = congestion.ByteCount(20)
	const maxCongestionWindowPackets = congestion.ByteCount(80)

	b := newBbrSender(
		DefaultClock{},
		oldMaxDatagramSize,
		initialCongestionWindowPackets*oldMaxDatagramSize,
		maxCongestionWindowPackets*oldMaxDatagramSize,
		ProfileStandard,
	)
	b.congestionWindow = b.initialCongestionWindow

	b.SetMaxDatagramSize(newMaxDatagramSize)

	require.Equal(t, initialCongestionWindowPackets*newMaxDatagramSize, b.initialCongestionWindow)
	require.Equal(t, maxCongestionWindowPackets*newMaxDatagramSize, b.maxCongestionWindow)
	require.Equal(t, minCongestionWindowPackets*newMaxDatagramSize, b.minCongestionWindow)
	require.Equal(t, initialCongestionWindowPackets*newMaxDatagramSize, b.congestionWindow)
}

func TestSetMaxDatagramSizeClampsCongestionWindow(t *testing.T) {
	const oldMaxDatagramSize = congestion.ByteCount(1000)
	const newMaxDatagramSize = congestion.ByteCount(1400)

	b := NewBbrSender(DefaultClock{}, oldMaxDatagramSize, ProfileStandard)
	b.congestionWindow = b.minCongestionWindow + oldMaxDatagramSize
	b.recoveryWindow = b.minCongestionWindow + oldMaxDatagramSize

	b.SetMaxDatagramSize(newMaxDatagramSize)

	require.Equal(t, b.minCongestionWindow, b.congestionWindow)
	require.Equal(t, b.minCongestionWindow, b.recoveryWindow)
}

func TestNewBbrSenderAppliesProfiles(t *testing.T) {
	testCases := []struct {
		name                                string
		profile                             Profile
		highGain                            float64
		highCwndGain                        float64
		congestionWindowGainConstant        float64
		numStartupRtts                      int64
		drainToTarget                       bool
		detectOvershooting                  bool
		bytesLostMultiplier                 uint8
		enableAckAggregationDuringStartup   bool
		expireAckAggregationInStartup       bool
		enableOverestimateAvoidance         bool
		reduceExtraAckedOnBandwidthIncrease bool
	}{
		{
			name:                         "standard",
			profile:                      ProfileStandard,
			highGain:                     defaultHighGain,
			highCwndGain:                 derivedHighCWNDGain,
			congestionWindowGainConstant: 2.0,
			numStartupRtts:               roundTripsWithoutGrowthBeforeExitingStartup,
			bytesLostMultiplier:          2,
		},
		{
			name:                                "conservative",
			profile:                             ProfileConservative,
			highGain:                            2.25,
			highCwndGain:                        1.75,
			congestionWindowGainConstant:        1.75,
			numStartupRtts:                      2,
			drainToTarget:                       true,
			detectOvershooting:                  true,
			bytesLostMultiplier:                 1,
			enableOverestimateAvoidance:         true,
			reduceExtraAckedOnBandwidthIncrease: true,
		},
		{
			name:                              "aggressive",
			profile:                           ProfileAggressive,
			highGain:                          3.0,
			highCwndGain:                      2.25,
			congestionWindowGainConstant:      2.5,
			numStartupRtts:                    4,
			bytesLostMultiplier:               2,
			enableAckAggregationDuringStartup: true,
			expireAckAggregationInStartup:     true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			b := NewBbrSender(DefaultClock{}, congestion.InitialPacketSize, tc.profile)
			require.Equal(t, tc.profile, b.profile)
			require.Equal(t, tc.highGain, b.highGain)
			require.Equal(t, tc.highCwndGain, b.highCwndGain)
			require.Equal(t, tc.congestionWindowGainConstant, b.congestionWindowGainConstant)
			require.Equal(t, tc.numStartupRtts, b.numStartupRtts)
			require.Equal(t, tc.drainToTarget, b.drainToTarget)
			require.Equal(t, tc.detectOvershooting, b.detectOvershooting)
			require.Equal(t, tc.bytesLostMultiplier, b.bytesLostMultiplierWhileDetectingOvershooting)
			require.Equal(t, tc.enableAckAggregationDuringStartup, b.enableAckAggregationDuringStartup)
			require.Equal(t, tc.expireAckAggregationInStartup, b.expireAckAggregationInStartup)
			require.Equal(t, tc.enableOverestimateAvoidance, b.sampler.IsOverestimateAvoidanceEnabled())
			require.Equal(t, tc.reduceExtraAckedOnBandwidthIncrease, b.sampler.maxAckHeightTracker.reduceExtraAckedOnBandwidthIncrease)
			require.Equal(t, b.highGain, b.pacingGain)
			require.Equal(t, b.highCwndGain, b.congestionWindowGain)
		})
	}
}

func TestParseProfile(t *testing.T) {
	profile, err := ParseProfile("")
	require.NoError(t, err)
	require.Equal(t, ProfileStandard, profile)

	profile, err = ParseProfile("Aggressive")
	require.NoError(t, err)
	require.Equal(t, ProfileAggressive, profile)

	_, err = ParseProfile("turbo")
	require.EqualError(t, err, `unsupported BBR profile "turbo"`)
}
