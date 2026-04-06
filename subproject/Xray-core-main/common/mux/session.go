package mux

import (
	"context"
	"io"
	"runtime"
	"sync"
	"time"

	"github.com/xtls/xray-core/common"
	"github.com/xtls/xray-core/common/buf"
	"github.com/xtls/xray-core/common/errors"
	"github.com/xtls/xray-core/common/net"
	"github.com/xtls/xray-core/common/protocol"
	"github.com/xtls/xray-core/common/signal/done"
	"github.com/xtls/xray-core/transport/pipe"
)

type SessionManager struct {
	sync.RWMutex
	sessions map[uint16]*Session
	count    uint16
	closed   bool
}

func NewSessionManager() *SessionManager {
	return &SessionManager{
		count:    0,
		sessions: make(map[uint16]*Session, 16),
	}
}

func (m *SessionManager) Closed() bool {
	m.RLock()
	defer m.RUnlock()

	return m.closed
}

func (m *SessionManager) Size() int {
	m.RLock()
	defer m.RUnlock()

	return len(m.sessions)
}

func (m *SessionManager) Count() int {
	m.RLock()
	defer m.RUnlock()

	return int(m.count)
}

func (m *SessionManager) Allocate(Strategy *ClientStrategy) *Session {
	m.Lock()
	defer m.Unlock()

	MaxConcurrency := int(Strategy.MaxConcurrency)
	MaxConnection := uint16(Strategy.MaxConnection)

	if m.closed || (MaxConcurrency > 0 && len(m.sessions) >= MaxConcurrency) || (MaxConnection > 0 && m.count >= MaxConnection) {
		return nil
	}

	m.count++
	s := &Session{
		ID:     m.count,
		parent: m,
		done:   done.New(),
	}
	m.sessions[s.ID] = s
	return s
}

func (m *SessionManager) Add(s *Session) bool {
	m.Lock()
	defer m.Unlock()

	if m.closed {
		return false
	}

	m.count++
	m.sessions[s.ID] = s
	return true
}

func (m *SessionManager) Remove(locked bool, id uint16) {
	if !locked {
		m.Lock()
		defer m.Unlock()
	}
	locked = true

	if m.closed {
		return
	}

	delete(m.sessions, id)

	/*
		if len(m.sessions) == 0 {
			m.sessions = make(map[uint16]*Session, 16)
		}
	*/
}

func (m *SessionManager) Get(id uint16) (*Session, bool) {
	m.RLock()
	defer m.RUnlock()

	if m.closed {
		return nil, false
	}

	s, found := m.sessions[id]
	return s, found
}

func (m *SessionManager) CloseIfNoSessionAndIdle(checkSize int, checkCount int) bool {
	m.Lock()
	defer m.Unlock()

	if m.closed {
		return true
	}

	if len(m.sessions) != 0 || checkSize != 0 || checkCount != int(m.count) {
		return false
	}

	m.closed = true

	m.sessions = nil
	return true
}

func (m *SessionManager) Close() error {
	m.Lock()
	defer m.Unlock()

	if m.closed {
		return nil
	}

	m.closed = true

	for _, s := range m.sessions {
		s.Close(true)
	}

	m.sessions = nil
	return nil
}

// Session represents a client connection in a Mux connection.
type Session struct {
	input        buf.Reader
	output       buf.Writer
	parent       *SessionManager
	ID           uint16
	transferType protocol.TransferType
	closed       bool
	done         *done.Instance
	XUDP         *XUDP
}

// Close closes all resources associated with this session.
func (s *Session) Close(locked bool) error {
	if !locked {
		s.parent.Lock()
		defer s.parent.Unlock()
	}
	locked = true
	if s.closed {
		return nil
	}
	s.closed = true
	if s.done != nil {
		s.done.Close()
	}
	if s.XUDP == nil {
		common.Interrupt(s.input)
		common.Close(s.output)
	} else {
		// Stop existing handle(), then trigger writer.Close().
		// Note that s.output may be dispatcher.SizeStatWriter.
		s.input.(*pipe.Reader).ReturnAnError(io.EOF)
		runtime.Gosched()
		// If the error set by ReturnAnError still exists, clear it.
		s.input.(*pipe.Reader).Recover()
		XUDPManager.Lock()
		if s.XUDP.Status == Active {
			s.XUDP.Expire = time.Now().Add(time.Minute)
			s.XUDP.Status = Expiring
			errors.LogDebug(context.Background(), "XUDP put ", s.XUDP.GlobalID)
		}
		XUDPManager.Unlock()
	}
	s.parent.Remove(locked, s.ID)
	return nil
}

// NewReader creates a buf.Reader based on the transfer type of this Session.
func (s *Session) NewReader(reader *buf.BufferedReader, dest *net.Destination) buf.Reader {
	if s.transferType == protocol.TransferTypeStream {
		return NewStreamReader(reader)
	}
	return NewPacketReader(reader, dest)
}

const (
	Initializing = 0
	Active       = 1
	Expiring     = 2
)

type XUDP struct {
	GlobalID [8]byte
	Status   uint64
	Expire   time.Time
	Mux      *Session
}

func (x *XUDP) Interrupt() {
	common.Interrupt(x.Mux.input)
	common.Close(x.Mux.output)
}

var XUDPManager struct {
	sync.Mutex
	Map map[[8]byte]*XUDP
}

func init() {
	XUDPManager.Map = make(map[[8]byte]*XUDP)
	go func() {
		for {
			time.Sleep(time.Minute)
			now := time.Now()
			XUDPManager.Lock()
			for id, x := range XUDPManager.Map {
				if x.Status == Expiring && now.After(x.Expire) {
					x.Interrupt()
					delete(XUDPManager.Map, id)
					errors.LogDebug(context.Background(), "XUDP del ", id)
				}
			}
			XUDPManager.Unlock()
		}
	}()
}
