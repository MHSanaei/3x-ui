package service

import (
	"math"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

// localSessionObservation reports activity this panel saw at now: a traffic delta,
// or a live connection that moved nothing (up and down 0).
func localSessionObservation(now, up, down int64) database.SessionObservation {
	return database.SessionObservation{
		QuietBefore:     now - onlineGracePeriodMs,
		UntrackedBefore: math.MaxInt64,
		Start:           now,
		OpenUp:          up,
		OpenDown:        down,
		Up:              up,
		Down:            down,
	}
}

// nodeSessionObservation reports a node's row for a client. A node that tracks
// sessions decides the boundary itself, since merges can lag its online window.
func nodeSessionObservation(node xray.ClientTraffic, up, down int64) database.SessionObservation {
	if node.SessionStart > 0 {
		return database.SessionObservation{
			QuietBefore:     node.SessionStart - onlineGracePeriodMs,
			UntrackedBefore: node.LastOnline + onlineGracePeriodMs,
			Start:           node.SessionStart,
			OpenUp:          clampTrafficCounter(node.SessionUp),
			OpenDown:        clampTrafficCounter(node.SessionDown),
			Up:              up,
			Down:            down,
		}
	}
	// Older nodes send no session: only a gap in lastOnline marks one, and an
	// untracked row opens only on activity newer than what it already holds.
	return database.SessionObservation{
		QuietBefore:     node.LastOnline - onlineGracePeriodMs,
		UntrackedBefore: node.LastOnline,
		Start:           node.LastOnline,
		OpenUp:          up,
		OpenDown:        down,
		Up:              up,
		Down:            down,
	}
}

// fresherNodeCopy orders a node's divergent copies of one email (#5274): the
// latest activity wins, then the latest session, then the larger session.
func fresherNodeCopy(a, b xray.ClientTraffic) bool {
	if a.LastOnline != b.LastOnline {
		return a.LastOnline > b.LastOnline
	}
	if a.SessionStart != b.SessionStart {
		return a.SessionStart > b.SessionStart
	}
	return a.SessionUp+a.SessionDown > b.SessionUp+b.SessionDown
}
