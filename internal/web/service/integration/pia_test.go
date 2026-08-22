package integration

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/netip"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/crypto/nodetoken"
	"github.com/mhsanaei/3x-ui/v3/internal/database"
	piaprotocol "github.com/mhsanaei/3x-ui/v3/internal/pia"
)

type fakePiaAuth struct{ token string }

func (f fakePiaAuth) Authenticate(context.Context, string, []byte) (piaprotocol.Token, error) {
	return piaprotocol.Token{Value: []byte(f.token), ExpiresAt: time.Now().Add(24 * time.Hour)}, nil
}

type fakePiaCatalog struct{ payload []byte }

func (f fakePiaCatalog) Fetch(context.Context) (piaprotocol.ServerListSnapshot, error) {
	return piaprotocol.ServerListSnapshot{Payload: f.payload, SchemaHint: "6", SignatureVerified: true}, nil
}

type fakePiaRegistrar struct {
	n     int
	token string
}

func (f *fakePiaRegistrar) RegisterKey(_ context.Context, server piaprotocol.WireGuardServer, token string, _ string) (piaprotocol.Registration, error) {
	f.n++
	f.token = token
	key := make([]byte, 32)
	key[0] = byte(f.n)
	return piaprotocol.Registration{
		PeerIP:     netip.MustParsePrefix("10.8.0." + strconv.Itoa(f.n) + "/32"),
		ServerKey:  base64.StdEncoding.EncodeToString(key),
		ServerIP:   server.IP,
		ServerPort: 1337,
	}, nil
}

func setupPiaService(t *testing.T) *PiaService {
	t.Helper()
	if err := database.InitDB(filepath.Join(t.TempDir(), "x-ui.db")); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.CloseDB() })
	payload := []byte(`{"version":6,"groups":{"wg":[{"name":"wireguard","ports":[1337]}]},"regions":[{"id":"us-east","name":"US East","country":"US","geo":false,"offline":false,"port_forward":true,"servers":{"wg":[{"ip":"198.51.100.10","cn":"useast1"},{"ip":"198.51.100.20","cn":"useast2"}]}},{"id":"de-berlin","name":"Berlin","country":"DE","geo":false,"offline":false,"port_forward":false,"servers":{"wg":[{"ip":"203.0.113.10","cn":"berlin1"}]}}]}`)
	svc := NewPiaService()
	svc.Auth = fakePiaAuth{token: "tokentokentokentoken12"}
	svc.Catalog = piaprotocol.NewCatalog(fakePiaCatalog{payload: payload})
	svc.Registrar = &fakePiaRegistrar{}
	return svc
}

func TestPiaLoginStoresTokenAndHidesItFromData(t *testing.T) {
	svc := setupPiaService(t)
	view, err := svc.Login("p1234567", "TEST-PIA-PASSWORD-MUST-NOT-LEAK")
	if err != nil {
		t.Fatal(err)
	}
	if view.Username != "p1234567" || view.AccountHint != "p1****67" {
		t.Fatalf("account view: %+v", view)
	}
	data, err := svc.GetPiaData()
	if err != nil || data == nil || data.AccountHint != "p1****67" {
		t.Fatalf("data: %+v err=%v", data, err)
	}
	raw, _ := json.Marshal(data)
	if strings.Contains(string(raw), "TEST-PIA-PASSWORD-MUST-NOT-LEAK") || strings.Contains(string(raw), "tokentokentokentoken12") {
		t.Fatalf("secret leaked in data: %s", raw)
	}
	stored, err := svc.GetPia()
	if err != nil || !strings.Contains(stored, "tokentokentokentoken12") {
		t.Fatalf("token must be stored in settings: %q err=%v", stored, err)
	}
}

func TestPiaCountriesAndServers(t *testing.T) {
	svc := setupPiaService(t)
	countries, err := svc.GetCountries()
	if err != nil {
		t.Fatal(err)
	}
	if len(countries) != 2 || countries[0].Code != "DE" || countries[1].Code != "US" {
		t.Fatalf("countries: %+v", countries)
	}
	servers, err := svc.GetServers("US")
	if err != nil {
		t.Fatal(err)
	}
	if len(servers.Regions) != 1 || servers.Regions[0].ID != "us-east" || len(servers.Servers) != 2 {
		t.Fatalf("us servers: %+v", servers)
	}
	if servers.Servers[0].Hostname != "useast1" || servers.Servers[0].RegionID != "us-east" {
		t.Fatalf("first server: %+v", servers.Servers[0])
	}
}

func TestPiaAddKeyRegistersWireGuardPeer(t *testing.T) {
	svc := setupPiaService(t)
	if _, err := svc.AddKey("useast1"); err == nil || piaprotocol.CodeOf(err) != piaprotocol.CodeTokenRejected {
		t.Fatalf("addKey before login: %v", err)
	}
	if _, err := svc.Login("p1234567", "password-long-enough"); err != nil {
		t.Fatal(err)
	}
	key, err := svc.AddKey("useast1")
	if err != nil {
		t.Fatal(err)
	}
	if key.Tag != "pia-us-east-useast1" || key.Hostname != "useast1" || key.SecretKey == "" || key.PublicKey == "" {
		t.Fatalf("key: %+v", key)
	}
	if key.Address != "10.8.0.1/32" || key.Endpoint != "198.51.100.10:1337" {
		t.Fatalf("peer: %+v", key)
	}
	byTag, err := svc.AddKey("pia-us-east-useast1")
	if err != nil || byTag.Hostname != "useast1" || byTag.Tag != "pia-us-east-useast1" {
		t.Fatalf("addKey by tag: %+v err=%v", byTag, err)
	}
	if _, err := svc.AddKey("1a"); err == nil || piaprotocol.CodeOf(err) != piaprotocol.CodeServerNotFound {
		t.Fatalf("truncated hostname must not match: %v", err)
	}
}

func TestPiaExpiredTokenNeverReachesRegistrar(t *testing.T) {
	svc := setupPiaService(t)
	if _, err := svc.Login("p1234567", "password-long-enough"); err != nil {
		t.Fatal(err)
	}
	raw, err := svc.GetPia()
	if err != nil {
		t.Fatal(err)
	}
	var stored piaStored
	if err := json.Unmarshal([]byte(raw), &stored); err != nil {
		t.Fatal(err)
	}
	stored.TokenExpiresAt = time.Now().Add(-time.Minute).Unix()
	rewritten, err := json.Marshal(stored)
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.SetPia(string(rewritten)); err != nil {
		t.Fatal(err)
	}
	reg := svc.Registrar.(*fakePiaRegistrar)
	before := reg.n
	_, err = svc.AddKey("useast1")
	if err == nil || piaprotocol.CodeOf(err) != piaprotocol.CodeTokenRejected {
		t.Fatalf("expired token: %v", err)
	}
	if piaprotocol.MessageOf(err) != "The PIA token has expired. Sign in again." {
		t.Fatalf("expired token message: %v", err)
	}
	if reg.n != before {
		t.Fatalf("expired token reached registrar: calls=%d", reg.n)
	}
}

func TestPiaDelClearsAccount(t *testing.T) {
	svc := setupPiaService(t)
	if _, err := svc.Login("p1234567", "password-long-enough"); err != nil {
		t.Fatal(err)
	}
	if err := svc.DelPiaData(); err != nil {
		t.Fatal(err)
	}
	data, err := svc.GetPiaData()
	if err != nil || data != nil {
		t.Fatalf("want nil data after logout, got %+v err=%v", data, err)
	}
}

func TestPiaOutboundTag(t *testing.T) {
	tests := []struct {
		region, host, want string
	}{
		{"us-east", "useast1", "pia-us-east-useast1"},
		{"US-East", "useast401.privacy.network", "pia-us-east-useast401"},
		{"us_california", "silicon_valley", "pia-us-california-silicon-valley"},
		{"", "berlin1", "pia-berlin1"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := piaOutboundTag(tt.region, tt.host); got != tt.want {
				t.Fatalf("piaOutboundTag(%q, %q) = %q, want %q", tt.region, tt.host, got, tt.want)
			}
		})
	}
}

func TestPiaCorruptSettingIsNotTreatedAsLoggedOut(t *testing.T) {
	svc := setupPiaService(t)
	if err := svc.SetPia(`{"username":`); err != nil {
		t.Fatal(err)
	}
	data, err := svc.GetPiaData()
	if err == nil || data != nil {
		t.Fatalf("corrupt pia setting must not look logged-out: data=%+v err=%v", data, err)
	}
}

func enablePiaTokenEncryption(t *testing.T) {
	t.Helper()
	var k [32]byte
	for i := range k {
		k[i] = byte(i + 1)
	}
	ring := &nodetoken.Keyring{ActiveID: "t1", Keys: map[string][32]byte{"t1": k}}
	codec, err := nodetoken.NewCodec(nodetoken.ModeRequired, ring)
	if err != nil {
		t.Fatalf("new codec: %v", err)
	}
	nodetoken.Init(codec)
	t.Cleanup(func() {
		off, _ := nodetoken.NewCodec(nodetoken.ModeOff, nil)
		nodetoken.Init(off)
	})
}

func TestPiaLoginEncryptsTokenWhenRequired(t *testing.T) {
	svc := setupPiaService(t)
	enablePiaTokenEncryption(t)
	if _, err := svc.Login("p1234567", "password-long-enough"); err != nil {
		t.Fatal(err)
	}
	stored, err := svc.GetPia()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(stored, "tokentokentokentoken12") {
		t.Fatalf("plaintext token at rest: %s", stored)
	}
	var parsed piaStored
	if err := json.Unmarshal([]byte(stored), &parsed); err != nil {
		t.Fatal(err)
	}
	if !nodetoken.IsEncrypted(parsed.Token) {
		t.Fatalf("token at rest is not encrypted: %q", parsed.Token)
	}
	data, err := svc.GetPiaData()
	if err != nil || data == nil || data.AccountHint != "p1****67" {
		t.Fatalf("data: %+v err=%v", data, err)
	}
	raw, _ := json.Marshal(data)
	if strings.Contains(string(raw), "tokentokentokentoken12") {
		t.Fatalf("secret leaked in data: %s", raw)
	}
}

func TestPiaAddKeyDecryptsEncryptedToken(t *testing.T) {
	svc := setupPiaService(t)
	enablePiaTokenEncryption(t)
	if _, err := svc.Login("p1234567", "password-long-enough"); err != nil {
		t.Fatal(err)
	}
	key, err := svc.AddKey("useast1")
	if err != nil {
		t.Fatal(err)
	}
	if key.Tag != "pia-us-east-useast1" {
		t.Fatalf("key: %+v", key)
	}
	reg := svc.Registrar.(*fakePiaRegistrar)
	if reg.token != "tokentokentokentoken12" {
		t.Fatalf("addKey must decrypt the stored token, got %q", reg.token)
	}
}

func TestPiaEncryptedTokenRejectedWhenEncryptionOff(t *testing.T) {
	svc := setupPiaService(t)
	enablePiaTokenEncryption(t)
	if _, err := svc.Login("p1234567", "password-long-enough"); err != nil {
		t.Fatal(err)
	}
	off, _ := nodetoken.NewCodec(nodetoken.ModeOff, nil)
	nodetoken.Init(off)
	if _, err := svc.AddKey("useast1"); err == nil || piaprotocol.CodeOf(err) != piaprotocol.CodeTokenRejected {
		t.Fatalf("addKey with encrypted token and encryption off: %v", err)
	}
}

func TestPiaWrongAADCiphertextRejected(t *testing.T) {
	svc := setupPiaService(t)
	enablePiaTokenEncryption(t)
	enc, err := nodetoken.Encrypt(1, "tokentokentokentoken12")
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(piaStored{Username: "p1234567", Token: enc, TokenExpiresAt: time.Now().Add(time.Hour).Unix()})
	if err := svc.SetPia(string(raw)); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.AddKey("useast1"); err == nil {
		t.Fatal("node-bound ciphertext must not decrypt as a PIA token")
	} else if !strings.Contains(err.Error(), "authentication failed") {
		t.Fatalf("wrong-AAD error: %v", err)
	}
}

func TestPiaPlaintextMigratesWhenEncryptionEnabled(t *testing.T) {
	svc := setupPiaService(t)
	if _, err := svc.Login("p1234567", "password-long-enough"); err != nil {
		t.Fatal(err)
	}
	before, err := svc.GetPia()
	if err != nil || !strings.Contains(before, "tokentokentokentoken12") {
		t.Fatalf("want plaintext before migrate: %q err=%v", before, err)
	}
	enablePiaTokenEncryption(t)
	if _, err := svc.GetPiaData(); err != nil {
		t.Fatal(err)
	}
	after, err := svc.GetPia()
	if err != nil || strings.Contains(after, "tokentokentokentoken12") {
		t.Fatalf("want ciphertext after migrate: %q err=%v", after, err)
	}
	var parsed piaStored
	if err := json.Unmarshal([]byte(after), &parsed); err != nil || !nodetoken.IsEncrypted(parsed.Token) {
		t.Fatalf("migrated token: %+v err=%v", parsed, err)
	}
}

func TestPiaReencryptsTokenToActiveKey(t *testing.T) {
	svc := setupPiaService(t)
	var k1, k2 [32]byte
	for i := range k1 {
		k1[i] = byte(i + 1)
		k2[i] = byte(i + 2)
	}
	c1, err := nodetoken.NewCodec(nodetoken.ModeRequired, &nodetoken.Keyring{
		ActiveID: "k1", Keys: map[string][32]byte{"k1": k1, "k2": k2},
	})
	if err != nil {
		t.Fatal(err)
	}
	nodetoken.Init(c1)
	t.Cleanup(func() {
		off, _ := nodetoken.NewCodec(nodetoken.ModeOff, nil)
		nodetoken.Init(off)
	})
	if _, err := svc.Login("p1234567", "password-long-enough"); err != nil {
		t.Fatal(err)
	}
	before, err := svc.GetPia()
	if err != nil || !strings.Contains(before, "enc:v1:k1:") {
		t.Fatalf("want k1 ciphertext: %q err=%v", before, err)
	}
	c2, err := nodetoken.NewCodec(nodetoken.ModeRequired, &nodetoken.Keyring{
		ActiveID: "k2", Keys: map[string][32]byte{"k1": k1, "k2": k2},
	})
	if err != nil {
		t.Fatal(err)
	}
	nodetoken.Init(c2)
	if _, err := svc.GetPiaData(); err != nil {
		t.Fatal(err)
	}
	after, err := svc.GetPia()
	if err != nil || !strings.Contains(after, "enc:v1:k2:") {
		t.Fatalf("want k2 ciphertext: %q err=%v", after, err)
	}
	if strings.Contains(after, "tokentokentokentoken12") {
		t.Fatalf("plaintext leaked after rotation: %s", after)
	}
}
