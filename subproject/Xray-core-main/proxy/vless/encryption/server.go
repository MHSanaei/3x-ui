package encryption

import (
	"bytes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/mlkem"
	"crypto/rand"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/xtls/xray-core/common/crypto"
	"github.com/xtls/xray-core/common/errors"
	"lukechampine.com/blake3"
)

type ServerSession struct {
	PfsKey  []byte
	NfsKeys sync.Map
}

type ServerInstance struct {
	NfsSKeys      []any
	NfsPKeysBytes [][]byte
	Hash32s       [][32]byte
	RelaysLength  int
	XorMode       uint32
	SecondsFrom   int64
	SecondsTo     int64
	PaddingLens   [][3]int
	PaddingGaps   [][3]int

	RWLock   sync.RWMutex
	Closed   bool
	Lasts    map[int64][16]byte
	Tickets  [][16]byte
	Sessions map[[16]byte]*ServerSession
}

func (i *ServerInstance) Init(nfsSKeysBytes [][]byte, xorMode uint32, secondsFrom, secondsTo int64, padding string) (err error) {
	if i.NfsSKeys != nil {
		return errors.New("already initialized")
	}
	l := len(nfsSKeysBytes)
	if l == 0 {
		return errors.New("empty nfsSKeysBytes")
	}
	i.NfsSKeys = make([]any, l)
	i.NfsPKeysBytes = make([][]byte, l)
	i.Hash32s = make([][32]byte, l)
	for j, k := range nfsSKeysBytes {
		if len(k) == 32 {
			if i.NfsSKeys[j], err = ecdh.X25519().NewPrivateKey(k); err != nil {
				return
			}
			i.NfsPKeysBytes[j] = i.NfsSKeys[j].(*ecdh.PrivateKey).PublicKey().Bytes()
			i.RelaysLength += 32 + 32
		} else {
			if i.NfsSKeys[j], err = mlkem.NewDecapsulationKey768(k); err != nil {
				return
			}
			i.NfsPKeysBytes[j] = i.NfsSKeys[j].(*mlkem.DecapsulationKey768).EncapsulationKey().Bytes()
			i.RelaysLength += 1088 + 32
		}
		i.Hash32s[j] = blake3.Sum256(i.NfsPKeysBytes[j])
	}
	i.RelaysLength -= 32
	i.XorMode = xorMode
	i.SecondsFrom = secondsFrom
	i.SecondsTo = secondsTo
	err = ParsePadding(padding, &i.PaddingLens, &i.PaddingGaps)
	if err != nil {
		return
	}
	if i.SecondsFrom > 0 || i.SecondsTo > 0 {
		i.Lasts = make(map[int64][16]byte)
		i.Tickets = make([][16]byte, 0, 1024)
		i.Sessions = make(map[[16]byte]*ServerSession)
		go func() {
			for {
				time.Sleep(time.Minute)
				i.RWLock.Lock()
				if i.Closed {
					i.RWLock.Unlock()
					return
				}
				minute := time.Now().Unix() / 60
				last := i.Lasts[minute]
				delete(i.Lasts, minute)
				delete(i.Lasts, minute-1) // for insurance
				if last != [16]byte{} {
					for j, ticket := range i.Tickets {
						delete(i.Sessions, ticket)
						if ticket == last {
							i.Tickets = i.Tickets[j+1:]
							break
						}
					}
				}
				i.RWLock.Unlock()
			}
		}()
	}
	return
}

func (i *ServerInstance) Close() (err error) {
	i.RWLock.Lock()
	i.Closed = true
	i.RWLock.Unlock()
	return
}

func (i *ServerInstance) Handshake(conn net.Conn, fallback *[]byte) (*CommonConn, error) {
	if i.NfsSKeys == nil {
		return nil, errors.New("uninitialized")
	}
	c := NewCommonConn(conn, true)

	ivAndRelays := make([]byte, 16+i.RelaysLength)
	if _, err := io.ReadFull(conn, ivAndRelays); err != nil {
		return nil, err
	}
	if fallback != nil {
		*fallback = append(*fallback, ivAndRelays...)
	}
	iv := ivAndRelays[:16]
	relays := ivAndRelays[16:]
	var nfsKey []byte
	var lastCTR cipher.Stream
	for j, k := range i.NfsSKeys {
		if lastCTR != nil {
			lastCTR.XORKeyStream(relays, relays[:32]) // recover this relay
		}
		var index = 32
		if _, ok := k.(*mlkem.DecapsulationKey768); ok {
			index = 1088
		}
		if i.XorMode > 0 {
			NewCTR(i.NfsPKeysBytes[j], iv).XORKeyStream(relays, relays[:index]) // we don't use buggy elligator2, because we have PSK :)
		}
		if k, ok := k.(*ecdh.PrivateKey); ok {
			publicKey, err := ecdh.X25519().NewPublicKey(relays[:index])
			if err != nil {
				return nil, err
			}
			if publicKey.Bytes()[31] > 127 { // we just don't want the observer can change even one bit without breaking the connection, though it has nothing to do with security
				return nil, errors.New("the highest bit of the last byte of the peer-sent X25519 public key is not 0")
			}
			nfsKey, err = k.ECDH(publicKey)
			if err != nil {
				return nil, err
			}
		}
		if k, ok := k.(*mlkem.DecapsulationKey768); ok {
			var err error
			nfsKey, err = k.Decapsulate(relays[:index])
			if err != nil {
				return nil, err
			}
		}
		if j == len(i.NfsSKeys)-1 {
			break
		}
		relays = relays[index:]
		lastCTR = NewCTR(nfsKey, iv)
		lastCTR.XORKeyStream(relays, relays[:32])
		if !bytes.Equal(relays[:32], i.Hash32s[j+1][:]) {
			return nil, errors.New("unexpected hash32: ", fmt.Sprintf("%v", relays[:32]))
		}
		relays = relays[32:]
	}
	nfsAEAD := NewAEAD(iv, nfsKey, c.UseAES)

	encryptedLength := make([]byte, 18)
	if _, err := io.ReadFull(conn, encryptedLength); err != nil {
		return nil, err
	}
	if fallback != nil {
		*fallback = append(*fallback, encryptedLength...)
	}
	decryptedLength := make([]byte, 2)
	if _, err := nfsAEAD.Open(decryptedLength[:0], nil, encryptedLength, nil); err != nil {
		c.UseAES = !c.UseAES
		nfsAEAD = NewAEAD(iv, nfsKey, c.UseAES)
		if _, err := nfsAEAD.Open(decryptedLength[:0], nil, encryptedLength, nil); err != nil {
			return nil, err
		}
	}
	if fallback != nil {
		*fallback = nil
	}
	length := DecodeLength(decryptedLength)

	if length == 32 {
		if i.SecondsFrom == 0 && i.SecondsTo == 0 {
			return nil, errors.New("0-RTT is not allowed")
		}
		encryptedTicket := make([]byte, 32)
		if _, err := io.ReadFull(conn, encryptedTicket); err != nil {
			return nil, err
		}
		ticket, err := nfsAEAD.Open(nil, nil, encryptedTicket, nil)
		if err != nil {
			return nil, err
		}
		i.RWLock.RLock()
		s := i.Sessions[[16]byte(ticket)]
		i.RWLock.RUnlock()
		if s == nil {
			noises := make([]byte, crypto.RandBetween(1279, 2279)) // matches 1-RTT's server hello length for "random", though it is not important, just for example
			var err error
			for err == nil {
				rand.Read(noises)
				_, err = DecodeHeader(noises)
			}
			conn.Write(noises) // make client do new handshake
			return nil, errors.New("expired ticket")
		}
		if _, loaded := s.NfsKeys.LoadOrStore([32]byte(nfsKey), true); loaded { // prevents bad client also
			return nil, errors.New("replay detected")
		}
		c.UnitedKey = append(s.PfsKey, nfsKey...) // the same nfsKey links the upload & download (prevents server -> client's another request)
		c.PreWrite = make([]byte, 16)
		rand.Read(c.PreWrite) // always trust yourself, not the client (also prevents being parsed as TLS thus causing false interruption for "native" and "xorpub")
		c.AEAD = NewAEAD(c.PreWrite, c.UnitedKey, c.UseAES)
		c.PeerAEAD = NewAEAD(encryptedTicket, c.UnitedKey, c.UseAES) // unchangeable ctx (prevents server -> server), and different ctx length for upload / download (prevents client -> client)
		if i.XorMode == 2 {
			c.Conn = NewXorConn(conn, NewCTR(c.UnitedKey, c.PreWrite), NewCTR(c.UnitedKey, iv), 16, 0) // it doesn't matter if the attacker sends client's iv back to the client
		}
		return c, nil
	}

	if length < 1184+32+16 { // client may send more public keys in the future's version
		return nil, errors.New("too short length")
	}
	encryptedPfsPublicKey := make([]byte, length)
	if _, err := io.ReadFull(conn, encryptedPfsPublicKey); err != nil {
		return nil, err
	}
	if _, err := nfsAEAD.Open(encryptedPfsPublicKey[:0], nil, encryptedPfsPublicKey, nil); err != nil {
		return nil, err
	}
	mlkem768EKey, err := mlkem.NewEncapsulationKey768(encryptedPfsPublicKey[:1184])
	if err != nil {
		return nil, err
	}
	mlkem768Key, encapsulatedPfsKey := mlkem768EKey.Encapsulate()
	peerX25519PKey, err := ecdh.X25519().NewPublicKey(encryptedPfsPublicKey[1184 : 1184+32])
	if err != nil {
		return nil, err
	}
	x25519SKey, _ := ecdh.X25519().GenerateKey(rand.Reader)
	x25519Key, err := x25519SKey.ECDH(peerX25519PKey)
	if err != nil {
		return nil, err
	}
	pfsKey := make([]byte, 32+32) // no more capacity
	copy(pfsKey, mlkem768Key)
	copy(pfsKey[32:], x25519Key)
	pfsPublicKey := append(encapsulatedPfsKey, x25519SKey.PublicKey().Bytes()...)
	c.UnitedKey = append(pfsKey, nfsKey...)
	c.AEAD = NewAEAD(pfsPublicKey, c.UnitedKey, c.UseAES)
	c.PeerAEAD = NewAEAD(encryptedPfsPublicKey[:1184+32], c.UnitedKey, c.UseAES)

	ticket := [16]byte{}
	rand.Read(ticket[:])
	var seconds int64
	if i.SecondsTo == 0 {
		seconds = i.SecondsFrom * crypto.RandBetween(50, 100) / 100
	} else {
		seconds = crypto.RandBetween(i.SecondsFrom, i.SecondsTo)
	}
	copy(ticket[:], EncodeLength(int(seconds)))
	if seconds > 0 {
		i.RWLock.Lock()
		i.Lasts[(time.Now().Unix()+max(i.SecondsFrom, i.SecondsTo))/60+2] = ticket
		i.Tickets = append(i.Tickets, ticket)
		i.Sessions[ticket] = &ServerSession{PfsKey: pfsKey}
		i.RWLock.Unlock()
	}

	pfsKeyExchangeLength := 1088 + 32 + 16
	encryptedTicketLength := 32
	paddingLength, paddingLens, paddingGaps := CreatPadding(i.PaddingLens, i.PaddingGaps)
	serverHello := make([]byte, pfsKeyExchangeLength+encryptedTicketLength+paddingLength)
	nfsAEAD.Seal(serverHello[:0], MaxNonce, pfsPublicKey, nil)
	c.AEAD.Seal(serverHello[:pfsKeyExchangeLength], nil, ticket[:], nil)
	padding := serverHello[pfsKeyExchangeLength+encryptedTicketLength:]
	c.AEAD.Seal(padding[:0], nil, EncodeLength(paddingLength-18), nil)
	c.AEAD.Seal(padding[:18], nil, padding[18:paddingLength-16], nil)

	paddingLens[0] = pfsKeyExchangeLength + encryptedTicketLength + paddingLens[0]
	for i, l := range paddingLens { // sends padding in a fragmented way, to create variable traffic pattern, before inner VLESS flow takes control
		if l > 0 {
			if _, err := conn.Write(serverHello[:l]); err != nil {
				return nil, err
			}
			serverHello = serverHello[l:]
		}
		if len(paddingGaps) > i {
			time.Sleep(paddingGaps[i])
		}
	}

	// important: allows client sends padding slowly, eliminating 1-RTT's traffic pattern
	if _, err := io.ReadFull(conn, encryptedLength); err != nil {
		return nil, err
	}
	if _, err := nfsAEAD.Open(encryptedLength[:0], nil, encryptedLength, nil); err != nil {
		return nil, err
	}
	encryptedPadding := make([]byte, DecodeLength(encryptedLength[:2]))
	if _, err := io.ReadFull(conn, encryptedPadding); err != nil {
		return nil, err
	}
	if _, err := nfsAEAD.Open(encryptedPadding[:0], nil, encryptedPadding, nil); err != nil {
		return nil, err
	}

	if i.XorMode == 2 {
		c.Conn = NewXorConn(conn, NewCTR(c.UnitedKey, ticket[:]), NewCTR(c.UnitedKey, iv), 0, 0)
	}
	return c, nil
}
