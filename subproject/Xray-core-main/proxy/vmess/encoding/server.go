package encoding

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"hash/fnv"
	"io"
	"sync"
	"time"

	"github.com/xtls/xray-core/common"
	"github.com/xtls/xray-core/common/bitmask"
	"github.com/xtls/xray-core/common/buf"
	"github.com/xtls/xray-core/common/crypto"
	"github.com/xtls/xray-core/common/drain"
	"github.com/xtls/xray-core/common/errors"
	"github.com/xtls/xray-core/common/net"
	"github.com/xtls/xray-core/common/protocol"
	"github.com/xtls/xray-core/common/task"
	"github.com/xtls/xray-core/proxy/vmess"
	vmessaead "github.com/xtls/xray-core/proxy/vmess/aead"
	"golang.org/x/crypto/chacha20poly1305"
)

type sessionID struct {
	user  [16]byte
	key   [16]byte
	nonce [16]byte
}

// SessionHistory keeps track of historical session ids, to prevent replay attacks.
type SessionHistory struct {
	sync.RWMutex
	cache map[sessionID]time.Time
	task  *task.Periodic
}

// NewSessionHistory creates a new SessionHistory object.
func NewSessionHistory() *SessionHistory {
	h := &SessionHistory{
		cache: make(map[sessionID]time.Time, 128),
	}
	h.task = &task.Periodic{
		Interval: time.Second * 30,
		Execute:  h.removeExpiredEntries,
	}
	return h
}

// Close implements common.Closable.
func (h *SessionHistory) Close() error {
	return h.task.Close()
}

func (h *SessionHistory) addIfNotExits(session sessionID) bool {
	h.Lock()

	if expire, found := h.cache[session]; found && expire.After(time.Now()) {
		h.Unlock()
		return false
	}

	h.cache[session] = time.Now().Add(time.Minute * 3)
	h.Unlock()
	common.Must(h.task.Start())
	return true
}

func (h *SessionHistory) removeExpiredEntries() error {
	now := time.Now()

	h.Lock()
	defer h.Unlock()

	if len(h.cache) == 0 {
		return errors.New("nothing to do")
	}

	for session, expire := range h.cache {
		if expire.Before(now) {
			delete(h.cache, session)
		}
	}

	if len(h.cache) == 0 {
		h.cache = make(map[sessionID]time.Time, 128)
	}

	return nil
}

// ServerSession keeps information for a session in VMess server.
type ServerSession struct {
	userValidator   *vmess.TimedUserValidator
	sessionHistory  *SessionHistory
	requestBodyKey  [16]byte
	requestBodyIV   [16]byte
	responseBodyKey [16]byte
	responseBodyIV  [16]byte
	responseWriter  io.Writer
	responseHeader  byte
}

// NewServerSession creates a new ServerSession, using the given UserValidator.
// The ServerSession instance doesn't take ownership of the validator.
func NewServerSession(validator *vmess.TimedUserValidator, sessionHistory *SessionHistory) *ServerSession {
	return &ServerSession{
		userValidator:  validator,
		sessionHistory: sessionHistory,
	}
}

func parseSecurityType(b byte) protocol.SecurityType {
	if _, f := protocol.SecurityType_name[int32(b)]; f {
		st := protocol.SecurityType(b)
		// For backward compatibility.
		if st == protocol.SecurityType_UNKNOWN {
			st = protocol.SecurityType_AUTO
		}
		return st
	}
	return protocol.SecurityType_UNKNOWN
}

// DecodeRequestHeader decodes and returns (if successful) a RequestHeader from an input stream.
func (s *ServerSession) DecodeRequestHeader(reader io.Reader, isDrain bool) (*protocol.RequestHeader, error) {
	buffer := buf.New()

	drainer, err := drain.NewBehaviorSeedLimitedDrainer(int64(s.userValidator.GetBehaviorSeed()), 16+38, 3266, 64)
	if err != nil {
		return nil, errors.New("failed to initialize drainer").Base(err)
	}

	drainConnection := func(e error) error {
		// We read a deterministic generated length of data before closing the connection to offset padding read pattern
		drainer.AcknowledgeReceive(int(buffer.Len()))
		if isDrain {
			return drain.WithError(drainer, reader, e)
		}
		return e
	}

	defer func() {
		buffer.Release()
	}()

	if _, err := buffer.ReadFullFrom(reader, protocol.IDBytesLen); err != nil {
		return nil, errors.New("failed to read request header").Base(err)
	}

	var decryptor io.Reader
	var vmessAccount *vmess.MemoryAccount

	user, foundAEAD, errorAEAD := s.userValidator.GetAEAD(buffer.Bytes())

	var fixedSizeAuthID [16]byte
	copy(fixedSizeAuthID[:], buffer.Bytes())

	switch {
	case foundAEAD:
		vmessAccount = user.Account.(*vmess.MemoryAccount)
		var fixedSizeCmdKey [16]byte
		copy(fixedSizeCmdKey[:], vmessAccount.ID.CmdKey())
		aeadData, shouldDrain, bytesRead, errorReason := vmessaead.OpenVMessAEADHeader(fixedSizeCmdKey, fixedSizeAuthID, reader)
		if errorReason != nil {
			if shouldDrain {
				drainer.AcknowledgeReceive(bytesRead)
				return nil, drainConnection(errors.New("AEAD read failed").Base(errorReason))
			} else {
				return nil, drainConnection(errors.New("AEAD read failed, drain skipped").Base(errorReason))
			}
		}
		decryptor = bytes.NewReader(aeadData)
	default:
		return nil, drainConnection(errors.New("invalid user").Base(errorAEAD))
	}

	drainer.AcknowledgeReceive(int(buffer.Len()))
	buffer.Clear()
	if _, err := buffer.ReadFullFrom(decryptor, 38); err != nil {
		return nil, errors.New("failed to read request header").Base(err)
	}

	request := &protocol.RequestHeader{
		User:    user,
		Version: buffer.Byte(0),
	}

	copy(s.requestBodyIV[:], buffer.BytesRange(1, 17))   // 16 bytes
	copy(s.requestBodyKey[:], buffer.BytesRange(17, 33)) // 16 bytes
	var sid sessionID
	copy(sid.user[:], vmessAccount.ID.Bytes())
	sid.key = s.requestBodyKey
	sid.nonce = s.requestBodyIV
	if !s.sessionHistory.addIfNotExits(sid) {
		return nil, errors.New("duplicated session id, possibly under replay attack, but this is a AEAD request")
	}

	s.responseHeader = buffer.Byte(33)             // 1 byte
	request.Option = bitmask.Byte(buffer.Byte(34)) // 1 byte
	paddingLen := int(buffer.Byte(35) >> 4)
	request.Security = parseSecurityType(buffer.Byte(35) & 0x0F)
	// 1 bytes reserved
	request.Command = protocol.RequestCommand(buffer.Byte(37))

	switch request.Command {
	case protocol.RequestCommandMux:
		request.Address = net.DomainAddress("v1.mux.cool")
		request.Port = 0

	case protocol.RequestCommandTCP, protocol.RequestCommandUDP:
		if addr, port, err := addrParser.ReadAddressPort(buffer, decryptor); err == nil {
			request.Address = addr
			request.Port = port
		}
	}

	if paddingLen > 0 {
		if _, err := buffer.ReadFullFrom(decryptor, int32(paddingLen)); err != nil {
			return nil, errors.New("failed to read padding").Base(err)
		}
	}

	if _, err := buffer.ReadFullFrom(decryptor, 4); err != nil {
		return nil, errors.New("failed to read checksum").Base(err)
	}

	fnv1a := fnv.New32a()
	common.Must2(fnv1a.Write(buffer.BytesTo(-4)))
	actualHash := fnv1a.Sum32()
	expectedHash := binary.BigEndian.Uint32(buffer.BytesFrom(-4))

	if actualHash != expectedHash {
		return nil, errors.New("invalid auth, but this is a AEAD request")
	}

	if request.Address == nil {
		return nil, errors.New("invalid remote address")
	}

	if request.Security == protocol.SecurityType_UNKNOWN || request.Security == protocol.SecurityType_AUTO {
		return nil, errors.New("unknown security type: ", request.Security)
	}

	return request, nil
}

// DecodeRequestBody returns Reader from which caller can fetch decrypted body.
func (s *ServerSession) DecodeRequestBody(request *protocol.RequestHeader, reader io.Reader) (buf.Reader, error) {
	var sizeParser crypto.ChunkSizeDecoder = crypto.PlainChunkSizeParser{}
	if request.Option.Has(protocol.RequestOptionChunkMasking) {
		sizeParser = NewShakeSizeParser(s.requestBodyIV[:])
	}
	var padding crypto.PaddingLengthGenerator
	if request.Option.Has(protocol.RequestOptionGlobalPadding) {
		var ok bool
		padding, ok = sizeParser.(crypto.PaddingLengthGenerator)
		if !ok {
			return nil, errors.New("invalid option: RequestOptionGlobalPadding")
		}
	}

	switch request.Security {
	case protocol.SecurityType_NONE:
		if request.Option.Has(protocol.RequestOptionChunkStream) {
			if request.Command.TransferType() == protocol.TransferTypeStream {
				return crypto.NewChunkStreamReader(sizeParser, reader), nil
			}

			auth := &crypto.AEADAuthenticator{
				AEAD:                    new(NoOpAuthenticator),
				NonceGenerator:          crypto.GenerateEmptyBytes(),
				AdditionalDataGenerator: crypto.GenerateEmptyBytes(),
			}
			return crypto.NewAuthenticationReader(auth, sizeParser, reader, protocol.TransferTypePacket, padding), nil
		}
		return buf.NewReader(reader), nil

	case protocol.SecurityType_AES128_GCM:
		aead := crypto.NewAesGcm(s.requestBodyKey[:])
		auth := &crypto.AEADAuthenticator{
			AEAD:                    aead,
			NonceGenerator:          GenerateChunkNonce(s.requestBodyIV[:], uint32(aead.NonceSize())),
			AdditionalDataGenerator: crypto.GenerateEmptyBytes(),
		}
		if request.Option.Has(protocol.RequestOptionAuthenticatedLength) {
			AuthenticatedLengthKey := vmessaead.KDF16(s.requestBodyKey[:], "auth_len")
			AuthenticatedLengthKeyAEAD := crypto.NewAesGcm(AuthenticatedLengthKey)

			lengthAuth := &crypto.AEADAuthenticator{
				AEAD:                    AuthenticatedLengthKeyAEAD,
				NonceGenerator:          GenerateChunkNonce(s.requestBodyIV[:], uint32(aead.NonceSize())),
				AdditionalDataGenerator: crypto.GenerateEmptyBytes(),
			}
			sizeParser = NewAEADSizeParser(lengthAuth)
		}
		return crypto.NewAuthenticationReader(auth, sizeParser, reader, request.Command.TransferType(), padding), nil

	case protocol.SecurityType_CHACHA20_POLY1305:
		aead, _ := chacha20poly1305.New(GenerateChacha20Poly1305Key(s.requestBodyKey[:]))

		auth := &crypto.AEADAuthenticator{
			AEAD:                    aead,
			NonceGenerator:          GenerateChunkNonce(s.requestBodyIV[:], uint32(aead.NonceSize())),
			AdditionalDataGenerator: crypto.GenerateEmptyBytes(),
		}
		if request.Option.Has(protocol.RequestOptionAuthenticatedLength) {
			AuthenticatedLengthKey := vmessaead.KDF16(s.requestBodyKey[:], "auth_len")
			AuthenticatedLengthKeyAEAD, err := chacha20poly1305.New(GenerateChacha20Poly1305Key(AuthenticatedLengthKey))
			common.Must(err)

			lengthAuth := &crypto.AEADAuthenticator{
				AEAD:                    AuthenticatedLengthKeyAEAD,
				NonceGenerator:          GenerateChunkNonce(s.requestBodyIV[:], uint32(aead.NonceSize())),
				AdditionalDataGenerator: crypto.GenerateEmptyBytes(),
			}
			sizeParser = NewAEADSizeParser(lengthAuth)
		}
		return crypto.NewAuthenticationReader(auth, sizeParser, reader, request.Command.TransferType(), padding), nil

	default:
		return nil, errors.New("invalid option: Security")
	}
}

// EncodeResponseHeader writes encoded response header into the given writer.
func (s *ServerSession) EncodeResponseHeader(header *protocol.ResponseHeader, writer io.Writer) {
	var encryptionWriter io.Writer
	BodyKey := sha256.Sum256(s.requestBodyKey[:])
	copy(s.responseBodyKey[:], BodyKey[:16])
	BodyIV := sha256.Sum256(s.requestBodyIV[:])
	copy(s.responseBodyIV[:], BodyIV[:16])

	aesStream := crypto.NewAesEncryptionStream(s.responseBodyKey[:], s.responseBodyIV[:])
	encryptionWriter = crypto.NewCryptionWriter(aesStream, writer)
	s.responseWriter = encryptionWriter

	aeadEncryptedHeaderBuffer := bytes.NewBuffer(nil)
	encryptionWriter = aeadEncryptedHeaderBuffer

	common.Must2(encryptionWriter.Write([]byte{s.responseHeader, byte(header.Option)}))
	err := MarshalCommand(header.Command, encryptionWriter)
	if err != nil {
		common.Must2(encryptionWriter.Write([]byte{0x00, 0x00}))
	}

	aeadResponseHeaderLengthEncryptionKey := vmessaead.KDF16(s.responseBodyKey[:], vmessaead.KDFSaltConstAEADRespHeaderLenKey)
	aeadResponseHeaderLengthEncryptionIV := vmessaead.KDF(s.responseBodyIV[:], vmessaead.KDFSaltConstAEADRespHeaderLenIV)[:12]

	aeadResponseHeaderLengthEncryptionAEAD := crypto.NewAesGcm(aeadResponseHeaderLengthEncryptionKey)

	aeadResponseHeaderLengthEncryptionBuffer := bytes.NewBuffer(nil)

	decryptedResponseHeaderLengthBinaryDeserializeBuffer := uint16(aeadEncryptedHeaderBuffer.Len())

	common.Must(binary.Write(aeadResponseHeaderLengthEncryptionBuffer, binary.BigEndian, decryptedResponseHeaderLengthBinaryDeserializeBuffer))

	AEADEncryptedLength := aeadResponseHeaderLengthEncryptionAEAD.Seal(nil, aeadResponseHeaderLengthEncryptionIV, aeadResponseHeaderLengthEncryptionBuffer.Bytes(), nil)
	common.Must2(io.Copy(writer, bytes.NewReader(AEADEncryptedLength)))

	aeadResponseHeaderPayloadEncryptionKey := vmessaead.KDF16(s.responseBodyKey[:], vmessaead.KDFSaltConstAEADRespHeaderPayloadKey)
	aeadResponseHeaderPayloadEncryptionIV := vmessaead.KDF(s.responseBodyIV[:], vmessaead.KDFSaltConstAEADRespHeaderPayloadIV)[:12]

	aeadResponseHeaderPayloadEncryptionAEAD := crypto.NewAesGcm(aeadResponseHeaderPayloadEncryptionKey)

	aeadEncryptedHeaderPayload := aeadResponseHeaderPayloadEncryptionAEAD.Seal(nil, aeadResponseHeaderPayloadEncryptionIV, aeadEncryptedHeaderBuffer.Bytes(), nil)
	common.Must2(io.Copy(writer, bytes.NewReader(aeadEncryptedHeaderPayload)))
}

// EncodeResponseBody returns a Writer that auto-encrypt content written by caller.
func (s *ServerSession) EncodeResponseBody(request *protocol.RequestHeader, writer io.Writer) (buf.Writer, error) {
	var sizeParser crypto.ChunkSizeEncoder = crypto.PlainChunkSizeParser{}
	if request.Option.Has(protocol.RequestOptionChunkMasking) {
		sizeParser = NewShakeSizeParser(s.responseBodyIV[:])
	}
	var padding crypto.PaddingLengthGenerator
	if request.Option.Has(protocol.RequestOptionGlobalPadding) {
		var ok bool
		padding, ok = sizeParser.(crypto.PaddingLengthGenerator)
		if !ok {
			return nil, errors.New("invalid option: RequestOptionGlobalPadding")
		}
	}

	switch request.Security {
	case protocol.SecurityType_NONE:
		if request.Option.Has(protocol.RequestOptionChunkStream) {
			if request.Command.TransferType() == protocol.TransferTypeStream {
				return crypto.NewChunkStreamWriter(sizeParser, writer), nil
			}

			auth := &crypto.AEADAuthenticator{
				AEAD:                    new(NoOpAuthenticator),
				NonceGenerator:          crypto.GenerateEmptyBytes(),
				AdditionalDataGenerator: crypto.GenerateEmptyBytes(),
			}
			return crypto.NewAuthenticationWriter(auth, sizeParser, writer, protocol.TransferTypePacket, padding), nil
		}
		return buf.NewWriter(writer), nil

	case protocol.SecurityType_AES128_GCM:
		aead := crypto.NewAesGcm(s.responseBodyKey[:])
		auth := &crypto.AEADAuthenticator{
			AEAD:                    aead,
			NonceGenerator:          GenerateChunkNonce(s.responseBodyIV[:], uint32(aead.NonceSize())),
			AdditionalDataGenerator: crypto.GenerateEmptyBytes(),
		}
		if request.Option.Has(protocol.RequestOptionAuthenticatedLength) {
			AuthenticatedLengthKey := vmessaead.KDF16(s.requestBodyKey[:], "auth_len")
			AuthenticatedLengthKeyAEAD := crypto.NewAesGcm(AuthenticatedLengthKey)

			lengthAuth := &crypto.AEADAuthenticator{
				AEAD:                    AuthenticatedLengthKeyAEAD,
				NonceGenerator:          GenerateChunkNonce(s.requestBodyIV[:], uint32(aead.NonceSize())),
				AdditionalDataGenerator: crypto.GenerateEmptyBytes(),
			}
			sizeParser = NewAEADSizeParser(lengthAuth)
		}
		return crypto.NewAuthenticationWriter(auth, sizeParser, writer, request.Command.TransferType(), padding), nil

	case protocol.SecurityType_CHACHA20_POLY1305:
		aead, _ := chacha20poly1305.New(GenerateChacha20Poly1305Key(s.responseBodyKey[:]))

		auth := &crypto.AEADAuthenticator{
			AEAD:                    aead,
			NonceGenerator:          GenerateChunkNonce(s.responseBodyIV[:], uint32(aead.NonceSize())),
			AdditionalDataGenerator: crypto.GenerateEmptyBytes(),
		}
		if request.Option.Has(protocol.RequestOptionAuthenticatedLength) {
			AuthenticatedLengthKey := vmessaead.KDF16(s.requestBodyKey[:], "auth_len")
			AuthenticatedLengthKeyAEAD, err := chacha20poly1305.New(GenerateChacha20Poly1305Key(AuthenticatedLengthKey))
			common.Must(err)

			lengthAuth := &crypto.AEADAuthenticator{
				AEAD:                    AuthenticatedLengthKeyAEAD,
				NonceGenerator:          GenerateChunkNonce(s.requestBodyIV[:], uint32(aead.NonceSize())),
				AdditionalDataGenerator: crypto.GenerateEmptyBytes(),
			}
			sizeParser = NewAEADSizeParser(lengthAuth)
		}
		return crypto.NewAuthenticationWriter(auth, sizeParser, writer, request.Command.TransferType(), padding), nil

	default:
		return nil, errors.New("invalid option: Security")
	}
}
