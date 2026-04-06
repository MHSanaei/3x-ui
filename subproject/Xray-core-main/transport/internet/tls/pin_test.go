package tls

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/xtls/xray-core/common"
	"github.com/xtls/xray-core/common/protocol/tls/cert"
)

func TestCalculateCertHash(t *testing.T) {
	const Single = `-----BEGIN CERTIFICATE-----
MIINWzCCC0OgAwIBAgITMwK6ajqdrV0tahuIrQAAArpqOjANBgkqhkiG9w0BAQwF
ADBdMQswCQYDVQQGEwJVUzEeMBwGA1UEChMVTWljcm9zb2Z0IENvcnBvcmF0aW9u
MS4wLAYDVQQDEyVNaWNyb3NvZnQgQXp1cmUgUlNBIFRMUyBJc3N1aW5nIENBIDA0
MB4XDTI1MDkwOTEwMzE1NloXDTI2MDMwODEwMzE1NlowYzELMAkGA1UEBhMCVVMx
CzAJBgNVBAgTAldBMRAwDgYDVQQHEwdSZWRtb25kMR4wHAYDVQQKExVNaWNyb3Nv
ZnQgQ29ycG9yYXRpb24xFTATBgNVBAMTDHd3dy5iaW5nLmNvbTCCASIwDQYJKoZI
hvcNAQEBBQADggEPADCCAQoCggEBAMBflymLifrVkjp8K4/XrHSt+/xDrrZIJyTI
JOhIGZJZ88sNjo4OChQWV8O3CTQwrbKJDd6KjZFFc6BPKpEJZ891w2zkymMbE7wh
vQVviSCIVCO+49pLrEvfh5ZvdbXhtNzm/ZRvkoI8h4ZKPBRNmX5sGpSQ9p0loJBj
Jk1HbzLv0vRk5bLb/J6x7YexaAu86C9TjqnC4irO+AZZNI/0S70ZHxX+ETZVV0EX
QU8UmqV68e4YhAQwiLYdAQw125n2hGWoLokQSZTyEiIIoubB00pE5zf0Qaq6Q4s8
Go5Ukw1A4HjWMisHVKq369pgI8VDZtMzOhS+O0DEQZLwOFETZxECAwEAAaOCCQww
ggkIMIIBgAYKKwYBBAHWeQIEAgSCAXAEggFsAWoAdgCWl2S/VViXrfdDh2g3CEJ3
6fA61fak8zZuRqQ/D8qpxgAAAZkuEXLdAAAEAwBHMEUCIBLzX4AJgVJdQshSMBLS
hBMQX8zgRm2U3IXjLk37JM3QAiEAkVrmCFx0+BM3NOoCAXBU1WzVuniPxJP3Ysbd
OO3dkEAAdwBkEcRspBLsp4kcogIuALyrTygH1B41J6vq/tUDyX3N8AAAAZkuEXKd
AAAEAwBIMEYCIQCCO1ys+tlI8Fhp4J/Dqk3VVtSi408Nuw8T6YciDL6LPgIhAPjp
fm/gMkASgNimNuMFH8oiJbqeQ/yo2zQfub894iMuAHcAVmzVo3a+g9/jQrZ1xJwj
JJinabrDgsurSaOHfZqzLQEAAAGZLhFy2QAABAMASDBGAiEA/93O6XiiYhfeANHh
0n2nJyVvFAc6sBNT2S7WOR28vR0CIQC7i+leDRRIeY2BYJwaRlAqHlSyU4DZu5IG
caxiWFeavzAnBgkrBgEEAYI3FQoEGjAYMAoGCCsGAQUFBwMCMAoGCCsGAQUFBwMB
MDwGCSsGAQQBgjcVBwQvMC0GJSsGAQQBgjcVCIe91xuB5+tGgoGdLo7QDIfw2h1d
gqvnMIft8R8CAWQCAS0wgbQGCCsGAQUFBwEBBIGnMIGkMHMGCCsGAQUFBzAChmdo
dHRwOi8vd3d3Lm1pY3Jvc29mdC5jb20vcGtpb3BzL2NlcnRzL01pY3Jvc29mdCUy
MEF6dXJlJTIwUlNBJTIwVExTJTIwSXNzdWluZyUyMENBJTIwMDQlMjAtJTIweHNp
Z24uY3J0MC0GCCsGAQUFBzABhiFodHRwOi8vb25lb2NzcC5taWNyb3NvZnQuY29t
L29jc3AwHQYDVR0OBBYEFAsWImxddBew8yEv3yGDsmy90FzPMA4GA1UdDwEB/wQE
AwIFoDCCBREGA1UdEQSCBQgwggUEghMqLnBsYXRmb3JtLmJpbmcuY29tggoqLmJp
bmcuY29tgghiaW5nLmNvbYIWaWVvbmxpbmUubWljcm9zb2Z0LmNvbYITKi53aW5k
b3dzc2VhcmNoLmNvbYIZY24uaWVvbmxpbmUubWljcm9zb2Z0LmNvbYIRKi5vcmln
aW4uYmluZy5jb22CDSoubW0uYmluZy5uZXSCDiouYXBpLmJpbmcuY29tgg0qLmNu
LmJpbmcubmV0gg0qLmNuLmJpbmcuY29tghBzc2wtYXBpLmJpbmcuY29tghBzc2wt
YXBpLmJpbmcubmV0gg4qLmFwaS5iaW5nLm5ldIIOKi5iaW5nYXBpcy5jb22CD2Jp
bmdzYW5kYm94LmNvbYIWZmVlZGJhY2subWljcm9zb2Z0LmNvbYIbaW5zZXJ0bWVk
aWEuYmluZy5vZmZpY2UubmV0gg5yLmJhdC5iaW5nLmNvbYIQKi5yLmJhdC5iaW5n
LmNvbYIPKi5kaWN0LmJpbmcuY29tgg4qLnNzbC5iaW5nLmNvbYIQKi5hcHBleC5i
aW5nLmNvbYIWKi5wbGF0Zm9ybS5jbi5iaW5nLmNvbYINd3AubS5iaW5nLmNvbYIM
Ki5tLmJpbmcuY29tgg9nbG9iYWwuYmluZy5jb22CEXdpbmRvd3NzZWFyY2guY29t
gg5zZWFyY2gubXNuLmNvbYIRKi5iaW5nc2FuZGJveC5jb22CGSouYXBpLnRpbGVz
LmRpdHUubGl2ZS5jb22CGCoudDAudGlsZXMuZGl0dS5saXZlLmNvbYIYKi50MS50
aWxlcy5kaXR1LmxpdmUuY29tghgqLnQyLnRpbGVzLmRpdHUubGl2ZS5jb22CGCou
dDMudGlsZXMuZGl0dS5saXZlLmNvbYILM2QubGl2ZS5jb22CE2FwaS5zZWFyY2gu
bGl2ZS5jb22CFGJldGEuc2VhcmNoLmxpdmUuY29tghVjbndlYi5zZWFyY2gubGl2
ZS5jb22CDWRpdHUubGl2ZS5jb22CEWZhcmVjYXN0LmxpdmUuY29tgg5pbWFnZS5s
aXZlLmNvbYIPaW1hZ2VzLmxpdmUuY29tghFsb2NhbC5saXZlLmNvbS5hdYIUbG9j
YWxzZWFyY2gubGl2ZS5jb22CFGxzNGQuc2VhcmNoLmxpdmUuY29tgg1tYWlsLmxp
dmUuY29tghFtYXBpbmRpYS5saXZlLmNvbYIObG9jYWwubGl2ZS5jb22CDW1hcHMu
bGl2ZS5jb22CEG1hcHMubGl2ZS5jb20uYXWCD21pbmRpYS5saXZlLmNvbYINbmV3
cy5saXZlLmNvbYIcb3JpZ2luLmNud2ViLnNlYXJjaC5saXZlLmNvbYIWcHJldmll
dy5sb2NhbC5saXZlLmNvbYIPc2VhcmNoLmxpdmUuY29tghJ0ZXN0Lm1hcHMubGl2
ZS5jb22CDnZpZGVvLmxpdmUuY29tgg92aWRlb3MubGl2ZS5jb22CFXZpcnR1YWxl
YXJ0aC5saXZlLmNvbYIMd2FwLmxpdmUuY29tghJ3ZWJtYXN0ZXIubGl2ZS5jb22C
FXd3dy5sb2NhbC5saXZlLmNvbS5hdYIUd3d3Lm1hcHMubGl2ZS5jb20uYXWCE3dl
Ym1hc3RlcnMubGl2ZS5jb22CGGVjbi5kZXYudmlydHVhbGVhcnRoLm5ldIIMd3d3
LmJpbmcuY29tMAwGA1UdEwEB/wQCMAAwagYDVR0fBGMwYTBfoF2gW4ZZaHR0cDov
L3d3dy5taWNyb3NvZnQuY29tL3BraW9wcy9jcmwvTWljcm9zb2Z0JTIwQXp1cmUl
MjBSU0ElMjBUTFMlMjBJc3N1aW5nJTIwQ0ElMjAwNC5jcmwwZgYDVR0gBF8wXTBR
BgwrBgEEAYI3TIN9AQEwQTA/BggrBgEFBQcCARYzaHR0cDovL3d3dy5taWNyb3Nv
ZnQuY29tL3BraW9wcy9Eb2NzL1JlcG9zaXRvcnkuaHRtMAgGBmeBDAECAjAfBgNV
HSMEGDAWgBQ7cNFT6XYlnWCoymYPxpuub1QWajAdBgNVHSUEFjAUBggrBgEFBQcD
AgYIKwYBBQUHAwEwDQYJKoZIhvcNAQEMBQADggIBAEQCoppNllgoHtfLJt2m7cVL
AILYFxJdi9qc4LUBfaQEdUwAfsC1pSk5YFB0aGcmVFKMvMMOeENOrWgNJVTLYI05
8mu6XmbiqUeIu1Rlye/yNirYm33Js2f3VXYp6HSzisF5cWq4QwYqA6XIMfDl61/y
IXVb5l5eTfproM2grn3RcVVbk5DuEUfyDPzYYNm8elxzac4RrbkDif/b+tVFxmrJ
CUx1o3VLiVVzbIFCDc5r6pPArm1EdgseJ7pRdXzg6flwA0INRpeLCpjtvkHeZCh7
GS2JUBhFv7M+lneJljNU/trTkYiho+ZRW9AgLcN73c4+1wHttPHk+w19m5Ge182V
HzCQdO27IGovKN8jkprGafGxYhyCn4KdSYbRrG7fjkckzpJrjCpF2/bJJ+o4Zi9P
rJIKHzY5lIMXcD7wwwT2WwlKXoTDrgm4QKN18V+kZaoOILdKyMlEww4jPFUqk6j1
0Qeod55F5h4tCq2lmwDIa/jyWTGgqTr4UESqj46NB5+JkGYl0O1PPbS1nUm9sN1l
hkY45iskXVXqLl6AVVcXyxMTefD43M81tFVuJJgpdD/BaMaXAuBdNDfTQcJwhP99
uI6HqHFD3iEct8fBkYfQiwH2e1eu9OwgujiWHsutyK8VvzVB3/YnhQ/TzciRjPqz
7ykUutQNUALq8dQwoTnK
-----END CERTIFICATE-----

`
	t.Run("singlepublickey", func(t *testing.T) {
		block, _ := pem.Decode([]byte(Single))
		cert, err := x509.ParseCertificate(block.Bytes)
		assert.Equal(t, err, nil)
		hash := GenerateCertHash(cert)
		fingerprint, _ := hex.DecodeString("ae243d668ec9c7f74a0dcd1ad21c6676b4efe30c39728934b362093af886bf77")
		assert.Equal(t, fingerprint, hash)
	})
}

func TestVerifyPeerLeafCert(t *testing.T) {
	leafCert, leafHash := cert.MustGenerate(nil, cert.DNSNames("example.com"))
	leaf := common.Must2(x509.ParseCertificate(leafCert.Certificate))

	r := &RandCarrier{
		Config: &tls.Config{
			ServerName: "example.com",
		},
		PinnedPeerCertSha256: [][]byte{leafHash[:]},
	}

	rawCerts := [][]byte{leaf.Raw}
	err := r.verifyPeerCert(rawCerts, nil)
	if err != nil {
		t.Fatal("expected to verify leaf cert signed by pinned CA, but got error:", err)
	}

	// make the pinned hash incorrect
	r.PinnedPeerCertSha256[0][0] += 1
	err = r.verifyPeerCert(rawCerts, nil)
	if err == nil {
		t.Fatal("expected to fail verifying leaf cert with incorrect pinned CA hash, but got no error")
	}
}

func TestVerifyPeerCACert(t *testing.T) {
	caCert, caHash := cert.MustGenerate(nil, cert.Authority(true), cert.KeyUsage(x509.KeyUsageCertSign))
	ca := common.Must2(x509.ParseCertificate(caCert.Certificate))

	leafCert, _ := cert.MustGenerate(caCert, cert.DNSNames("example.com"))
	leaf := common.Must2(x509.ParseCertificate(leafCert.Certificate))

	r := &RandCarrier{
		Config: &tls.Config{
			ServerName: "example.com",
		},
		PinnedPeerCertSha256: [][]byte{caHash[:]},
	}

	rawCerts := [][]byte{leaf.Raw, ca.Raw}
	err := r.verifyPeerCert(rawCerts, nil)
	if err != nil {
		t.Fatal("expected to verify leaf cert signed by pinned CA, but got error:", err)
	}

	// make the pinned hash incorrect
	r.PinnedPeerCertSha256[0][0] += 1
	err = r.verifyPeerCert(rawCerts, nil)
	if err == nil {
		t.Fatal("expected to fail verifying leaf cert with incorrect pinned CA hash, but got no error")
	}
}
