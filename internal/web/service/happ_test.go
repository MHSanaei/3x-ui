package service

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"

	"golang.org/x/crypto/chacha20poly1305"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
)

func initHappTestDB(t *testing.T) {
	t.Helper()
	dbDir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", dbDir)
	t.Setenv("XUI_BIN_FOLDER", dbDir)
	if err := os.WriteFile(filepath.Join(dbDir, "config.json"), []byte(`{"log":{}}`), 0o600); err != nil {
		t.Fatalf("write Xray config: %v", err)
	}
	if err := database.InitDB(filepath.Join(dbDir, "x-ui.db")); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = database.CloseDB() })
}

func seedHappClient(t *testing.T, subID string) *model.ClientRecord {
	t.Helper()
	client := &model.ClientRecord{Email: "happ@test", SubID: subID, Enable: true}
	if err := database.GetDB().Create(client).Error; err != nil {
		t.Fatalf("seed client: %v", err)
	}
	return client
}

func configureHappSubscription(t *testing.T, enabled bool, subURI string) {
	t.Helper()
	settings := &SettingService{}
	for key, value := range map[string]string{
		"subEnable": "false",
		"subURI":    subURI,
		"subPath":   "/sub/",
		"subPort":   "80",
		"subDomain": "",
	} {
		if key == "subEnable" && enabled {
			value = "true"
		}
		if err := settings.saveSetting(key, value); err != nil {
			t.Fatalf("save %s: %v", key, err)
		}
	}
}

func configureHappLinkGate(t *testing.T, enabled bool) {
	t.Helper()
	if err := (&SettingService{}).saveSetting("happLinkEnable", strconv.FormatBool(enabled)); err != nil {
		t.Fatalf("save happLinkEnable: %v", err)
	}
}

var syntheticHappKey = sync.OnceValues(func() (*rsa.PrivateKey, error) {
	return rsa.GenerateKey(rand.Reader, 4096)
})

func newLocalHappTestService(t *testing.T) (*HappService, *rsa.PrivateKey) {
	t.Helper()
	key, err := syntheticHappKey()
	if err != nil {
		t.Fatal(err)
	}
	svc := NewHappService(&ClientService{}, &SettingService{})
	svc.encrypt = func(source string) (string, error) { return encryptHappSource(source, &key.PublicKey) }
	return svc, key
}

func decryptHappTestLink(t *testing.T, link string, key *rsa.PrivateKey) string {
	t.Helper()
	return decodeHappTestLink(t, link, key).source
}

type happTestDecoded struct {
	source string
	key    []byte
	nonce  []byte
}

func decodeHappTestLink(t *testing.T, link string, key *rsa.PrivateKey) happTestDecoded {
	t.Helper()
	const prefix = "happ://crypt5/"
	if !strings.HasPrefix(link, prefix) {
		t.Fatal("unexpected Happ protocol")
	}
	payload := []byte(link[len(prefix):])
	// Independent inverse indexing catches encoder swap errors without sharing its helpers.
	frame := append([]byte{}, payload...)
	for i := 0; i+4 <= len(payload); i += 4 {
		copy(frame[i:i+2], payload[i+2:i+4])
		copy(frame[i+2:i+4], payload[i:i+2])
	}
	if len(frame) < 38 || string(frame[:4])+string(frame[len(frame)-4:]) != "vdfzfoff" {
		t.Fatal("invalid marker or short Crypt5 frame")
	}
	body := frame[4 : len(frame)-4]
	nonce, tag, salt := body[:12], body[12:14], body[14:22]
	if !regexp.MustCompile(`^[a-zA-Z0-9]{12}$`).Match(nonce) ||
		!regexp.MustCompile(`^[a-zA-Z]{2}$`).Match(tag) ||
		!regexp.MustCompile(`^[a-zA-Z0-9]{8}$`).Match(salt) {
		t.Fatal("incorrect salted field shape")
	}
	separatorIndex := 22
	for separatorIndex < len(body) && body[separatorIndex] >= '0' && body[separatorIndex] <= '9' {
		separatorIndex++
	}
	if separatorIndex == 22 || separatorIndex >= len(body) || body[separatorIndex] != 'V' {
		t.Fatal("missing length or wrong tested separator")
	}
	segmentLength, err := strconv.Atoi(string(body[22:separatorIndex]))
	if err != nil || segmentLength < 24 || segmentLength > len(body)-separatorIndex-1 {
		t.Fatal("invalid ciphertext segment length")
	}
	cipherB64 := body[separatorIndex+1 : separatorIndex+1+segmentLength]
	rsaB64 := body[separatorIndex+1+segmentLength:]
	rsaCipher, err := base64.StdEncoding.Strict().DecodeString(string(rsaB64))
	if err != nil || len(rsaCipher) != 512 || len(rsaB64) != 684 {
		t.Fatalf("expected standard padded Base64 of a 512-byte RSA block: %v", err)
	}
	//nolint:staticcheck // Only an ephemeral test key decodes Happ's required PKCS#1 v1.5 wrapping.
	rsaPlain, err := rsa.DecryptPKCS1v15(nil, key, rsaCipher)
	if err != nil || len(rsaPlain) != 44 {
		t.Fatalf("RSA wrapped key should contain 44 encoded bytes: %v", err)
	}
	keyB64 := make([]byte, len(rsaPlain))
	for i := range rsaPlain {
		keyB64[i] = rsaPlain[i^1]
	}
	wrappedKey, err := base64.StdEncoding.Strict().DecodeString(string(keyB64))
	if err != nil || len(wrappedKey) != 32 {
		t.Fatalf("wrapped key should decode to 32 bytes: %v", err)
	}
	sessionKey := make([]byte, 32)
	for i := range sessionKey {
		sessionKey[i] = wrappedKey[i] ^ salt[i%8]
	}
	ciphertext, err := base64.StdEncoding.Strict().DecodeString(string(cipherB64))
	if err != nil || !bytes.Equal([]byte(base64.StdEncoding.EncodeToString(ciphertext)), cipherB64) {
		t.Fatalf("noncanonical ciphertext Base64: %v", err)
	}
	aead, err := chacha20poly1305.New(sessionKey)
	if err != nil {
		t.Fatal(err)
	}
	swappedSource, err := aead.Open(nil, nonce, ciphertext, nil)
	if err != nil || len(swappedSource)%4 != 0 {
		t.Fatalf("AEAD authentication or source framing failed: %v", err)
	}
	sourceB64 := make([]byte, len(swappedSource))
	for i := range swappedSource {
		sourceB64[i] = swappedSource[i^1]
	}
	source, err := base64.StdEncoding.Strict().DecodeString(string(sourceB64))
	if err != nil {
		t.Fatal(err)
	}
	return happTestDecoded{string(source), sessionKey, append([]byte{}, nonce...)}
}

func TestHappGenerateRejectsDisabledGateBeforeEncryption(t *testing.T) {
	for _, value := range []string{"", "false", "not-a-bool"} {
		t.Run("setting="+value, func(t *testing.T) {
			initHappTestDB(t)
			client := seedHappClient(t, "current-sub-id")
			configureHappSubscription(t, true, "https://sub.example/sub/")
			if value != "" {
				if err := (&SettingService{}).saveSetting("happLinkEnable", value); err != nil {
					t.Fatal(err)
				}
			}
			svc := NewHappService(&ClientService{}, &SettingService{})
			svc.encrypt = func(string) (string, error) {
				t.Fatal("disabled feature attempted encryption")
				return "", nil
			}
			result, err := svc.Generate(context.Background(), client.Id, "panel.example")
			if !errors.Is(err, ErrHappLinkUnavailable) || result != (HappLinkResult{}) {
				t.Fatalf("disabled generation = %#v, %v", result, err)
			}
		})
	}
}

func TestHappGenerateUsesCurrentSourceAndFreshCiphertext(t *testing.T) {
	initHappTestDB(t)
	client := seedHappClient(t, "before")
	configureHappSubscription(t, true, "https://sub.example/sub/")
	configureHappLinkGate(t, true)
	svc, key := newLocalHappTestService(t)
	var previous string
	for range 2 {
		result, err := svc.Generate(context.Background(), client.Id, "panel.example")
		if err != nil {
			t.Fatal(err)
		}
		if got := decryptHappTestLink(t, result.EncryptedLink, key); got != "https://sub.example/sub/before" {
			t.Fatalf("source = %q", got)
		}
		if result.EncryptedLink == previous {
			t.Fatal("generation reused cached ciphertext")
		}
		previous = result.EncryptedLink
	}
	if err := database.GetDB().Model(client).Update("sub_id", "after").Error; err != nil {
		t.Fatal(err)
	}
	configureHappSubscription(t, true, "https://next.example/中文?literal=%2F&token=")
	result, err := svc.Generate(context.Background(), client.Id, "panel.example")
	if err != nil {
		t.Fatal(err)
	}
	if got := decryptHappTestLink(t, result.EncryptedLink, key); got != "https://next.example/中文?literal=%2F&token=after" {
		t.Fatalf("updated source = %q", got)
	}
	configureHappSubscription(t, true, "")
	result, err = svc.Generate(context.Background(), client.Id, "panel.example")
	if err != nil {
		t.Fatal(err)
	}
	if got := decryptHappTestLink(t, result.EncryptedLink, key); got != "http://panel.example/sub/after" {
		t.Fatalf("default source = %q", got)
	}
}

func TestHappGenerateDiscardsChangedSourceOrGate(t *testing.T) {
	for _, tc := range []struct {
		name   string
		reason string
		change func(*testing.T, *model.ClientRecord)
	}{
		{"subscription ID", "source_changed", func(t *testing.T, c *model.ClientRecord) {
			if err := database.GetDB().Model(c).Update("sub_id", "after").Error; err != nil {
				t.Fatal(err)
			}
		}},
		{"subscription URL", "source_changed", func(t *testing.T, _ *model.ClientRecord) {
			configureHappSubscription(t, true, "https://next.example/sub/")
		}},
		{"subscription disabled", "source_changed", func(t *testing.T, _ *model.ClientRecord) {
			configureHappSubscription(t, false, "https://sub.example/sub/")
		}},
		{"gate disabled", "integration_disabled", func(t *testing.T, _ *model.ClientRecord) {
			configureHappLinkGate(t, false)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			initHappTestDB(t)
			client := seedHappClient(t, "before")
			configureHappSubscription(t, true, "https://sub.example/sub/")
			configureHappLinkGate(t, true)
			svc, _ := newLocalHappTestService(t)
			encrypt := svc.encrypt
			svc.encrypt = func(source string) (string, error) {
				link, err := encrypt(source)
				tc.change(t, client)
				return link, err
			}
			result, err := svc.Generate(context.Background(), client.Id, "panel.example")
			if !errors.Is(err, ErrHappLinkUnavailable) || result != (HappLinkResult{}) {
				t.Fatalf("stale result = %#v, %v", result, err)
			}
			logs := logger.GetLogs(1, "WARNING")
			if len(logs) != 1 || !strings.Contains(logs[0], "reason="+tc.reason) {
				t.Fatalf("wrong stale-result diagnostic: %v", logs)
			}
		})
	}
}

func TestHappGenerateSkipsUnavailableSources(t *testing.T) {
	for _, tc := range []struct {
		name    string
		enabled bool
		subID   string
		missing bool
	}{
		{"disabled subscription", false, "current", false},
		{"missing client", true, "current", true},
		{"empty subscription ID", true, "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			initHappTestDB(t)
			client := seedHappClient(t, tc.subID)
			configureHappSubscription(t, tc.enabled, "https://sub.example/sub/")
			configureHappLinkGate(t, true)
			svc := NewHappService(&ClientService{}, &SettingService{})
			svc.encrypt = func(string) (string, error) { t.Fatal("unavailable source was encrypted"); return "", nil }
			id := client.Id
			if tc.missing {
				id++
			}
			result, err := svc.Generate(context.Background(), id, "panel.example")
			if !errors.Is(err, ErrHappLinkUnavailable) || result != (HappLinkResult{}) {
				t.Fatalf("unavailable result = %#v, %v", result, err)
			}
		})
	}
}

func TestHappGenerateDiscardsCancelledRequests(t *testing.T) {
	for _, before := range []bool{true, false} {
		t.Run(strconv.FormatBool(before), func(t *testing.T) {
			initHappTestDB(t)
			client := seedHappClient(t, "current")
			configureHappSubscription(t, true, "https://sub.example/sub/")
			configureHappLinkGate(t, true)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			svc, _ := newLocalHappTestService(t)
			encrypt := svc.encrypt
			svc.encrypt = func(source string) (string, error) {
				if before {
					t.Fatal("cancelled request attempted encryption")
				}
				link, err := encrypt(source)
				cancel()
				return link, err
			}
			if before {
				cancel()
			}
			result, err := svc.Generate(ctx, client.Id, "panel.example")
			if !errors.Is(err, ErrHappLinkUnavailable) || result != (HappLinkResult{}) {
				t.Fatalf("cancelled result = %#v, %v", result, err)
			}
			logs := logger.GetLogs(1, "WARNING")
			if len(logs) != 1 || !strings.Contains(logs[0], "reason=request_cancelled") {
				t.Fatalf("wrong cancellation diagnostic: %v", logs)
			}
		})
	}
}

func TestHappGeneratePropagatesLengthErrorWithoutSecrets(t *testing.T) {
	initHappTestDB(t)
	client := seedHappClient(t, strings.Repeat("s", 8173))
	configureHappSubscription(t, true, "https://example.com/")
	configureHappLinkGate(t, true)
	result, err := NewHappService(&ClientService{}, &SettingService{}).Generate(context.Background(), client.Id, "panel.example")
	if !errors.Is(err, ErrHappSourceTooLong) || result != (HappLinkResult{}) {
		t.Fatalf("length result = %#v, %v", result, err)
	}
	logs := logger.GetLogs(1, "WARNING")
	if len(logs) != 1 || !strings.Contains(logs[0], "reason=source_too_long") {
		t.Fatalf("length diagnostic = %v", logs)
	}
	if strings.Contains(logs[0], client.SubID) || strings.Contains(logs[0], "example.com") {
		t.Fatal("length diagnostic leaked source")
	}
}

func TestHappGenerateLogsSanitizedEncryptionFailure(t *testing.T) {
	initHappTestDB(t)
	client := seedHappClient(t, "secret-sub-id")
	configureHappSubscription(t, true, "https://sub.example/secret-source/")
	configureHappLinkGate(t, true)
	svc := NewHappService(&ClientService{}, &SettingService{})
	svc.encrypt = func(source string) (string, error) {
		return "", errors.New("encryption failed " + source + " token=secret cookie=session authorization=Bearer-secret happ://crypt5/leak")
	}
	result, err := svc.Generate(context.Background(), client.Id, "panel.example")
	if !errors.Is(err, ErrHappLinkUnavailable) || err.Error() != "happ link unavailable" || result != (HappLinkResult{}) {
		t.Fatalf("failure = %#v, %v", result, err)
	}
	logs := logger.GetLogs(1, "WARNING")
	if len(logs) != 1 {
		t.Fatalf("logs = %v", logs)
	}
	for _, want := range []string{"component=happ_link", "client_id=" + strconv.Itoa(client.Id), "reason=encryption", "elapsed_ms=", "correlation_id=", "encryption failed"} {
		if !strings.Contains(logs[0], want) {
			t.Fatalf("diagnostic missing %q: %s", want, logs[0])
		}
	}
	for _, secret := range []string{"secret-sub-id", "secret-source", "token=secret", "cookie=session", "Bearer-secret", "happ://"} {
		if strings.Contains(logs[0], secret) {
			t.Fatalf("diagnostic leaked %q", secret)
		}
	}
}

func TestSanitizeHappDetailRedactsSensitiveTokens(t *testing.T) {
	detail := sanitizeHappDetail("provider said https://provider.example/path?token=secret password=hunter2\nsource=https://sub.example/sub/current-sub-id", "https://sub.example/sub/current-sub-id", "current-sub-id")
	for _, secret := range []string{"provider.example", "token=secret", "hunter2", "current-sub-id", "\n"} {
		if strings.Contains(detail, secret) {
			t.Fatalf("sanitized detail leaked %q: %q", secret, detail)
		}
	}
}
