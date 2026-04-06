package aead

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"io"
	"time"

	"github.com/xtls/xray-core/common"
	"github.com/xtls/xray-core/common/crypto"
)

func SealVMessAEADHeader(key [16]byte, data []byte) []byte {
	generatedAuthID := CreateAuthID(key[:], time.Now().Unix())

	connectionNonce := make([]byte, 8)
	if _, err := io.ReadFull(rand.Reader, connectionNonce); err != nil {
		panic(err.Error())
	}

	aeadPayloadLengthSerializeBuffer := bytes.NewBuffer(nil)

	headerPayloadDataLen := uint16(len(data))

	common.Must(binary.Write(aeadPayloadLengthSerializeBuffer, binary.BigEndian, headerPayloadDataLen))

	aeadPayloadLengthSerializedByte := aeadPayloadLengthSerializeBuffer.Bytes()
	var payloadHeaderLengthAEADEncrypted []byte

	{
		payloadHeaderLengthAEADKey := KDF16(key[:], KDFSaltConstVMessHeaderPayloadLengthAEADKey, string(generatedAuthID[:]), string(connectionNonce))

		payloadHeaderLengthAEADNonce := KDF(key[:], KDFSaltConstVMessHeaderPayloadLengthAEADIV, string(generatedAuthID[:]), string(connectionNonce))[:12]

		payloadHeaderAEAD := crypto.NewAesGcm(payloadHeaderLengthAEADKey)

		payloadHeaderLengthAEADEncrypted = payloadHeaderAEAD.Seal(nil, payloadHeaderLengthAEADNonce, aeadPayloadLengthSerializedByte, generatedAuthID[:])
	}

	var payloadHeaderAEADEncrypted []byte

	{
		payloadHeaderAEADKey := KDF16(key[:], KDFSaltConstVMessHeaderPayloadAEADKey, string(generatedAuthID[:]), string(connectionNonce))

		payloadHeaderAEADNonce := KDF(key[:], KDFSaltConstVMessHeaderPayloadAEADIV, string(generatedAuthID[:]), string(connectionNonce))[:12]

		payloadHeaderAEAD := crypto.NewAesGcm(payloadHeaderAEADKey)

		payloadHeaderAEADEncrypted = payloadHeaderAEAD.Seal(nil, payloadHeaderAEADNonce, data, generatedAuthID[:])
	}

	outputBuffer := bytes.NewBuffer(nil)

	common.Must2(outputBuffer.Write(generatedAuthID[:]))               // 16
	common.Must2(outputBuffer.Write(payloadHeaderLengthAEADEncrypted)) // 2+16
	common.Must2(outputBuffer.Write(connectionNonce))                  // 8
	common.Must2(outputBuffer.Write(payloadHeaderAEADEncrypted))

	return outputBuffer.Bytes()
}

func OpenVMessAEADHeader(key [16]byte, authid [16]byte, data io.Reader) ([]byte, bool, int, error) {
	var payloadHeaderLengthAEADEncrypted [18]byte
	var nonce [8]byte

	var bytesRead int

	authidCheckValueReadBytesCounts, err := io.ReadFull(data, payloadHeaderLengthAEADEncrypted[:])
	bytesRead += authidCheckValueReadBytesCounts
	if err != nil {
		return nil, false, bytesRead, err
	}

	nonceReadBytesCounts, err := io.ReadFull(data, nonce[:])
	bytesRead += nonceReadBytesCounts
	if err != nil {
		return nil, false, bytesRead, err
	}

	// Decrypt Length

	var decryptedAEADHeaderLengthPayloadResult []byte

	{
		payloadHeaderLengthAEADKey := KDF16(key[:], KDFSaltConstVMessHeaderPayloadLengthAEADKey, string(authid[:]), string(nonce[:]))

		payloadHeaderLengthAEADNonce := KDF(key[:], KDFSaltConstVMessHeaderPayloadLengthAEADIV, string(authid[:]), string(nonce[:]))[:12]

		payloadHeaderLengthAEAD := crypto.NewAesGcm(payloadHeaderLengthAEADKey)

		decryptedAEADHeaderLengthPayload, erropenAEAD := payloadHeaderLengthAEAD.Open(nil, payloadHeaderLengthAEADNonce, payloadHeaderLengthAEADEncrypted[:], authid[:])

		if erropenAEAD != nil {
			return nil, true, bytesRead, erropenAEAD
		}

		decryptedAEADHeaderLengthPayloadResult = decryptedAEADHeaderLengthPayload
	}

	var length uint16

	common.Must(binary.Read(bytes.NewReader(decryptedAEADHeaderLengthPayloadResult), binary.BigEndian, &length))

	var decryptedAEADHeaderPayloadR []byte

	var payloadHeaderAEADEncryptedReadedBytesCounts int

	{
		payloadHeaderAEADKey := KDF16(key[:], KDFSaltConstVMessHeaderPayloadAEADKey, string(authid[:]), string(nonce[:]))

		payloadHeaderAEADNonce := KDF(key[:], KDFSaltConstVMessHeaderPayloadAEADIV, string(authid[:]), string(nonce[:]))[:12]

		// 16 == AEAD Tag size
		payloadHeaderAEADEncrypted := make([]byte, length+16)

		payloadHeaderAEADEncryptedReadedBytesCounts, err = io.ReadFull(data, payloadHeaderAEADEncrypted)
		bytesRead += payloadHeaderAEADEncryptedReadedBytesCounts
		if err != nil {
			return nil, false, bytesRead, err
		}

		payloadHeaderAEAD := crypto.NewAesGcm(payloadHeaderAEADKey)

		decryptedAEADHeaderPayload, erropenAEAD := payloadHeaderAEAD.Open(nil, payloadHeaderAEADNonce, payloadHeaderAEADEncrypted, authid[:])

		if erropenAEAD != nil {
			return nil, true, bytesRead, erropenAEAD
		}

		decryptedAEADHeaderPayloadR = decryptedAEADHeaderPayload
	}

	return decryptedAEADHeaderPayloadR, false, bytesRead, nil
}
