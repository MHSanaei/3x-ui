package job

import (
	"encoding/json"
	"fmt"
	"net"
	"slices"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

type ldapDir struct {
	ln    net.Listener
	mu    sync.Mutex
	users []string
}

func newLDAPDir(t *testing.T, users ...string) *ldapDir {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	d := &ldapDir{ln: ln, users: users}
	go d.serve()
	t.Cleanup(func() { _ = ln.Close() })
	return d
}

func (d *ldapDir) port() int { return d.ln.Addr().(*net.TCPAddr).Port }

func (d *ldapDir) setUsers(users []string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.users = users
}

func (d *ldapDir) serve() {
	for {
		conn, err := d.ln.Accept()
		if err != nil {
			return
		}
		go d.handle(conn)
	}
}

func (d *ldapDir) handle(conn net.Conn) {
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(10 * time.Second))
	buf := make([]byte, 512)
	n, err := conn.Read(buf)
	if err != nil {
		return
	}
	id, ok := ldapMessageID(buf[:n])
	if !ok {
		return
	}
	d.mu.Lock()
	users := append([]string(nil), d.users...)
	d.mu.Unlock()
	for _, user := range users {
		if _, err := conn.Write(ldapSearchEntry(id, user)); err != nil {
			return
		}
	}
	_, _ = conn.Write(ldapSearchDone(id))
}

func berLength(n int) []byte {
	if n < 0x80 {
		return []byte{byte(n)}
	}
	var encoded []byte
	for value := n; value > 0; value >>= 8 {
		encoded = append([]byte{byte(value & 0xff)}, encoded...)
	}
	return append([]byte{byte(0x80 | len(encoded))}, encoded...)
}

func berTLV(tag byte, content []byte) []byte {
	out := []byte{tag}
	out = append(out, berLength(len(content))...)
	return append(out, content...)
}

func berString(tag byte, value string) []byte { return berTLV(tag, []byte(value)) }

func berInt(tag byte, value int) []byte { return berTLV(tag, []byte{byte(value)}) }

func berSeq(items ...[]byte) []byte {
	var content []byte
	for _, item := range items {
		content = append(content, item...)
	}
	return berTLV(0x30, content)
}

func berConcat(parts ...[]byte) []byte {
	var out []byte
	for _, part := range parts {
		out = append(out, part...)
	}
	return out
}

func ldapMessageID(message []byte) (int, bool) {
	if len(message) < 2 || message[0] != 0x30 {
		return 0, false
	}
	index := 1
	if message[index]&0x80 != 0 {
		index += 1 + int(message[index]&0x7f)
	} else {
		index++
	}
	if index+2 > len(message) || message[index] != 0x02 {
		return 0, false
	}
	index++
	length := int(message[index])
	index++
	if length == 0 || length > 4 || index+length > len(message) {
		return 0, false
	}
	id := 0
	for _, value := range message[index : index+length] {
		id = id<<8 | int(value)
	}
	return id, true
}

func ldapSearchEntry(id int, mail string) []byte {
	mailAttr := berSeq(berString(0x04, "mail"), berTLV(0x31, berString(0x04, mail)))
	flagAttr := berSeq(berString(0x04, "vless_enabled"), berTLV(0x31, berString(0x04, "true")))
	entry := berTLV(0x64, berConcat(berString(0x04, "dc=example,dc=com"), berSeq(mailAttr, flagAttr)))
	return berSeq(berInt(0x02, id), entry)
}

func ldapSearchDone(id int) []byte {
	done := berTLV(0x65, berConcat(berInt(0x0a, 0), berString(0x04, ""), berString(0x04, "")))
	return berSeq(berInt(0x02, id), done)
}

func ldapSetSetting(t *testing.T, key, value string) {
	t.Helper()
	if err := database.GetDB().Where("key = ?", key).Delete(&model.Setting{}).Error; err != nil {
		t.Fatalf("clear setting %s: %v", key, err)
	}
	if err := database.GetDB().Create(&model.Setting{Key: key, Value: value}).Error; err != nil {
		t.Fatalf("set %s=%s: %v", key, value, err)
	}
}

func ldapInboundEmails(t *testing.T, inboundID int) []string {
	t.Helper()
	var inbound model.Inbound
	if err := database.GetDB().First(&inbound, inboundID).Error; err != nil {
		t.Fatalf("reload inbound: %v", err)
	}
	var parsed struct {
		Clients []model.Client `json:"clients"`
	}
	if err := json.Unmarshal([]byte(inbound.Settings), &parsed); err != nil {
		t.Fatalf("parse settings: %v", err)
	}
	emails := make([]string, 0, len(parsed.Clients))
	for _, client := range parsed.Clients {
		emails = append(emails, client.Email)
	}
	return emails
}

func assertLDAPInboundEmails(t *testing.T, inboundID int, want []string) {
	t.Helper()
	got := ldapInboundEmails(t, inboundID)
	want = slices.Clone(want)
	slices.Sort(got)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Fatalf("inbound clients = %v, want %v", got, want)
	}
}

func ldapBaselineFixture(t *testing.T) (*LdapSyncJob, *ldapDir, *model.Inbound, []string) {
	t.Helper()
	initLdapJobDB(t)
	db := database.GetDB()

	const tag = "ldap-baseline-in"
	all := make([]string, 0, 10)
	clients := make([]model.Client, 0, 10)
	for i := 0; i < 10; i++ {
		email := fmt.Sprintf("u%d@ldap", i)
		all = append(all, email)
		clients = append(clients, model.Client{Email: email, ID: fmt.Sprintf("00000000-0000-4000-8000-%012d", i), Enable: true})
	}
	settings, _ := json.Marshal(map[string]any{"clients": clients, "decryption": "none"})
	inbound := &model.Inbound{
		UserId: 1, Enable: true, Port: 47300, Protocol: model.VLESS,
		Tag: tag, Settings: string(settings), StreamSettings: `{"network":"tcp"}`,
	}
	if err := db.Create(inbound).Error; err != nil {
		t.Fatalf("create inbound: %v", err)
	}
	for _, client := range clients {
		record := model.ClientRecord{Email: client.Email, UUID: client.ID, Enable: true}
		if err := db.Create(&record).Error; err != nil {
			t.Fatalf("create client record: %v", err)
		}
		if err := db.Create(&model.ClientInbound{ClientId: record.Id, InboundId: inbound.Id}).Error; err != nil {
			t.Fatalf("link client: %v", err)
		}
		if err := db.Create(&xray.ClientTraffic{InboundId: inbound.Id, Email: client.Email, Enable: true}).Error; err != nil {
			t.Fatalf("create client traffic: %v", err)
		}
	}

	dir := newLDAPDir(t, all...)
	ldapSetSetting(t, "ldapEnable", "true")
	ldapSetSetting(t, "ldapHost", "127.0.0.1")
	ldapSetSetting(t, "ldapPort", strconv.Itoa(dir.port()))
	ldapSetSetting(t, "ldapInboundTags", tag)
	ldapSetSetting(t, "ldapAutoDelete", "true")

	return NewLdapSyncJob(), dir, inbound, all
}

func TestLdapAutoDeleteKeepsBaselineAfterRefusedFetch(t *testing.T) {
	job, dir, inbound, all := ldapBaselineFixture(t)

	job.Run()
	assertLDAPInboundEmails(t, inbound.Id, all)

	dir.setUsers(all[:4])
	job.Run()
	assertLDAPInboundEmails(t, inbound.Id, all)

	job.Run()
	assertLDAPInboundEmails(t, inbound.Id, all)
}

func TestLdapAutoDeleteKeepsBaselineAfterEmptyFetch(t *testing.T) {
	job, dir, inbound, all := ldapBaselineFixture(t)

	job.Run()
	assertLDAPInboundEmails(t, inbound.Id, all)

	dir.setUsers(nil)
	job.Run()
	assertLDAPInboundEmails(t, inbound.Id, all)

	dir.setUsers(all[:4])
	job.Run()
	assertLDAPInboundEmails(t, inbound.Id, all)
}
