// Package ha fails the master panel over with Patroni (docs/master-failover.md).
// Ported from github.com/AndreyOsipuk/telemux (GPL-3.0): internal/role, peers, patroni.
package ha

import (
	"slices"
	"time"
)

// Role is the panel's failover role.
type Role string

const (
	// Master may write the database and drive the nodes.
	Master Role = "master"
	// Replica runs on a PostgreSQL standby.
	Replica Role = "replica"
	// Fenced sits on the primary without a proven right to write (no quorum,
	// Patroni not leader, or master_hold not yet served).
	Fenced Role = "fenced"
	// Unknown means the local PostgreSQL could not be asked; never master.
	Unknown Role = "unknown"
)

// Config holds the role automaton parameters.
type Config struct {
	// Self is this machine's code, equal to its Patroni member name.
	Self string
	// Peers are the other voting machines; empty means a single-member cluster.
	Peers []string
	// RequirePatroni makes "local Patroni is the leader" a master condition.
	RequirePatroni bool
	// FenceAfter forgives a quorum loss this long; must stay below Patroni ttl.
	FenceAfter time.Duration
	// MasterHold is how long the master conditions must hold before promotion.
	MasterHold time.Duration
}

// Quorum is the vote count, own vote included, that makes a majority.
func (c Config) Quorum() int { return (len(c.Peers)+1)/2 + 1 }

// Sample is one observation of the node.
type Sample struct {
	// At must come from time.Now() so comparisons use the monotonic clock.
	At time.Time

	PGErr      error
	InRecovery bool

	PatroniErr    error
	PatroniLeader bool

	// Seen are peers visible over any channel (HTTP ping or a live streaming
	// standby); codes outside Config.Peers are ignored.
	Seen []string
	// MasterHint is the master code reported by the peers.
	MasterHint string
}

// PGPrimary reports whether the local PostgreSQL answered as a primary.
func (s Sample) PGPrimary() bool { return s.PGErr == nil && !s.InRecovery }

// State is the automaton's current decision.
type State struct {
	Role   Role      `json:"role"`
	Reason string    `json:"reason,omitempty"`
	Quorum bool      `json:"quorum"`
	Seen   []string  `json:"seen"`
	Since  time.Time `json:"since"`
	// Master is our own code when master, else the peers' hint.
	Master string `json:"master,omitempty"`

	initial        bool
	holding        bool
	lastQuorum     time.Time
	everQuorum     bool
	candidateSince time.Time
}

// Initial is the state before the first observation. A fresh process skips
// master_hold so restarting the panel on a healthy primary does not stop it.
func Initial() State {
	return State{Role: Unknown, Reason: "no observation yet", initial: true}
}

// IsMaster reports whether the node may write.
func (s State) IsMaster() bool { return s.Role == Master }

// Holding reports that the master conditions hold and master_hold is running.
func (s State) Holding() bool { return s.holding }

// Next is the pure transition: master = PG primary, Patroni leader (if
// required), quorum, and master_hold served. Every error leans to "not master".
func Next(cfg Config, prev State, s Sample) State {
	st := State{
		Seen:       memberPeers(cfg, s.Seen),
		lastQuorum: prev.lastQuorum,
		everQuorum: prev.everQuorum,
	}
	if 1+len(st.Seen) >= cfg.Quorum() {
		st.lastQuorum = s.At
		st.everQuorum = true
	}
	st.Quorum = st.everQuorum && s.At.Sub(st.lastQuorum) <= cfg.FenceAfter

	switch {
	case s.PGErr != nil:
		st.Role, st.Reason = Unknown, "local PostgreSQL unreachable"
	case s.InRecovery:
		st.Role = Replica
	case cfg.RequirePatroni && s.PatroniErr != nil:
		st.Role, st.Reason = Fenced, "Patroni unreachable"
	case cfg.RequirePatroni && !s.PatroniLeader:
		st.Role, st.Reason = Fenced, "Patroni does not consider this node the leader"
	case !st.Quorum:
		st.Role, st.Reason = Fenced, "no quorum of voting machines"
	case prev.Role == Master:
		st.Role = Master
	default:
		cand := prev.candidateSince
		if cand.IsZero() {
			cand = s.At
		}
		st.candidateSince = cand
		if prev.initial || s.At.Sub(cand) >= cfg.MasterHold {
			st.Role = Master
		} else {
			st.Role, st.Reason = Fenced, "master conditions hold, serving master_hold"
			st.holding = true
		}
	}
	if st.Role == Master {
		st.candidateSince = time.Time{}
		st.Master = cfg.Self
	} else {
		st.Master = s.MasterHint
	}

	st.Since = prev.Since
	if st.Role != prev.Role || st.Since.IsZero() {
		st.Since = s.At
	}
	return st
}

func memberPeers(cfg Config, seen []string) []string {
	out := make([]string, 0, len(seen))
	for _, c := range seen {
		if c != cfg.Self && slices.Contains(cfg.Peers, c) && !slices.Contains(out, c) {
			out = append(out, c)
		}
	}
	slices.Sort(out)
	return out
}
