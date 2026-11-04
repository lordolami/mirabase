package pgateway_test

import (
	"bufio"
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/binary"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"io"
	"log/slog"
	"math/big"
	"net"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"mirabase/apis"
	"mirabase/internal/application"
	"mirabase/internal/mbengine"
	"mirabase/internal/pgateway"
	"mirabase/tests"
)

// requireEngine skips when the pinned payload is not compiled in (the
// house parity harness _build/p02/parity_turso.sh passes the digest pin).
func requireEngine(t *testing.T) {
	t.Helper()
	if mbengine.PinnedDigest() == "" {
		t.Skip("requires the pinned engine payload (run via _build/p02/parity_turso.sh)")
	}
	if err := mbengine.Ensure(); err != nil {
		t.Skipf("engine ensure failed: %v", err)
	}
}

// debugTestLogger returns the session logger: stderr at debug when the
// PGW_DEBUG_LOG environment variable is set, a discarded logger otherwise.
func debugTestLogger() *slog.Logger {
	if os.Getenv("PGW_DEBUG_LOG") == "" {
		return slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
}

// ---------------------------------------------------------------------------
// Fixture: a real runtime (TestApp on the pinned engine), a provisioned
// target database, issued pg credentials and a TLS-only gateway listener.
// ---------------------------------------------------------------------------

type gatewayFixture struct {
	rt      *application.Runtime
	server  *pgateway.Server
	addr    string
	target1 string // database scoped credential 1
	target2 string // database scoped credential 2
	secret1 string // one-time secret of credential 1 (target1)
	secret2 string // one-time secret of credential 2 (target2)
}

func newGatewayFixture(t *testing.T) *gatewayFixture {
	t.Helper()
	requireEngine(t)

	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatalf("NewTestApp: %v", err)
	}
	t.Cleanup(func() { app.Cleanup() })

	rt, err := application.BindRuntime(app)
	if err != nil {
		t.Fatalf("BindRuntime: %v", err)
	}

	ctx := context.Background()
	d1, err := rt.Service.ProvisionManual(ctx, app.DataDir(), "pgw-target-1", "wal", "")
	if err != nil {
		t.Fatalf("provision target 1: %v", err)
	}
	d2, err := rt.Service.ProvisionManual(ctx, app.DataDir(), "pgw-target-2", "wal", "")
	if err != nil {
		t.Fatalf("provision target 2: %v", err)
	}

	cred1, err := rt.IssueCredential(ctx, d1.ID, "pg", []string{"sql.read", "sql.write"}, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatalf("issue credential 1: %v", err)
	}
	cred2, err := rt.IssueCredential(ctx, d2.ID, "pg", []string{"sql.read", "sql.write"}, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatalf("issue credential 2: %v", err)
	}

	server := pgateway.New(rt, pgateway.Config{
		Addr: "127.0.0.1:0",
		TLS:  testTLSConfig(t),
	}, debugTestLogger())
	ln, err := server.Listen()
	if err != nil {
		t.Fatalf("gateway listen: %v", err)
	}
	t.Cleanup(func() {
		_ = ln.Close()
		server.Close()
	})

	return &gatewayFixture{
		rt:      rt,
		server:  server,
		addr:    ln.Addr().String(),
		target1: d1.ID,
		target2: d2.ID,
		secret1: cred1.Secret,
		secret2: cred2.Secret,
	}
}

// testTLSConfig generates a self-signed server certificate for the
// loopback listener (the wiring step supplies the real certificate).
func testTLSConfig(t *testing.T) *tls.Config {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("tls key: %v", err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "127.0.0.1"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:     []string{"localhost"},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("tls cert: %v", err)
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatalf("tls key marshal: %v", err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		t.Fatalf("tls key pair: %v", err)
	}
	return &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS12}
}

// ---------------------------------------------------------------------------
// Minimal standard-library PG wire client (protocol 3, SCRAM-SHA-256)
// ---------------------------------------------------------------------------

type pgClient struct {
	t    *testing.T
	conn net.Conn
	br   *bufio.Reader
}

// pgErrorWire is one ErrorResponse received on the wire.
type pgErrorWire struct {
	Fields map[byte]string
}

func (e *pgErrorWire) code() string  { return e.Fields['C'] }
func (e *pgErrorWire) message() string {
	return e.Fields['M']
}

// dialPG performs the SSLRequest negotiation and the TLS handshake.
func dialPG(t *testing.T, addr string) *pgClient {
	t.Helper()
	raw, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	var sslReq [8]byte
	binary.BigEndian.PutUint32(sslReq[0:4], 8)
	binary.BigEndian.PutUint32(sslReq[4:8], 80877103)
	if _, err := raw.Write(sslReq[:]); err != nil {
		t.Fatalf("ssl request: %v", err)
	}
	var answer [1]byte
	if _, err := io.ReadFull(raw, answer[:]); err != nil {
		t.Fatalf("ssl answer: %v", err)
	}
	if answer[0] != 'S' {
		t.Fatalf("server refused TLS upgrade: %q", answer[0])
	}
	tlsConn := tls.Client(raw, &tls.Config{
		InsecureSkipVerify: true, // the fixture certificate is self-signed
		ServerName:         "127.0.0.1",
		MinVersion:         tls.VersionTLS12,
	})
	if err := tlsConn.Handshake(); err != nil {
		t.Fatalf("tls handshake: %v", err)
	}
	return &pgClient{t: t, conn: tlsConn, br: bufio.NewReader(tlsConn)}
}

// dialPlaintext connects WITHOUT TLS and sends the startup packet in the
// clear (the negative control for the TLS requirement).
func dialPlaintext(t *testing.T, addr, user, database string) *pgClient {
	t.Helper()
	raw, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	c := &pgClient{t: t, conn: raw, br: bufio.NewReader(raw)}
	c.sendStartup(user, database)
	return c
}

// sendStartup sends the StartupMessage.
func (c *pgClient) sendStartup(user, database string) {
	var payload bytes.Buffer
	var ver [4]byte
	binary.BigEndian.PutUint32(ver[:], 196608)
	payload.Write(ver[:])
	payload.WriteString("user\x00" + user + "\x00")
	if database != "" {
		payload.WriteString("database\x00" + database + "\x00")
	}
	payload.WriteString("application_name\x00pgateway-test\x00")
	payload.WriteByte(0)
	c.sendMessage(0, payload.Bytes()) // startup has no type byte
}

// sendMessage writes one framed message (msgType 0 = startup, no type).
func (c *pgClient) sendMessage(msgType byte, payload []byte) {
	c.t.Helper()
	var head []byte
	if msgType != 0 {
		head = append(head, msgType)
	}
	var lenBuf [4]byte
	binary.BigEndian.PutUint32(lenBuf[:], uint32(4+len(payload)))
	head = append(head, lenBuf[:]...)
	if _, err := c.conn.Write(append(head, payload...)); err != nil {
		c.t.Fatalf("write message %q: %v", string(rune(msgType)), err)
	}
}

// readMessage reads one backend message.
func (c *pgClient) readMessage() (byte, []byte) {
	c.t.Helper()
	var typeBuf [1]byte
	if _, err := io.ReadFull(c.br, typeBuf[:]); err != nil {
		c.t.Fatalf("read message type: %v", err)
	}
	var lenBuf [4]byte
	if _, err := io.ReadFull(c.br, lenBuf[:]); err != nil {
		c.t.Fatalf("read message length: %v", err)
	}
	n := int(binary.BigEndian.Uint32(lenBuf[:]))
	if n < 4 {
		c.t.Fatalf("absurd message length %d", n)
	}
	payload := make([]byte, n-4)
	if _, err := io.ReadFull(c.br, payload); err != nil {
		c.t.Fatalf("read message payload: %v", err)
	}
	return typeBuf[0], payload
}

// expectAuthRequest reads one Authentication* request.
func (c *pgClient) expectAuthRequest() (int32, []byte) {
	c.t.Helper()
	mtype, payload := c.readMessage()
	if mtype != 'R' {
		c.t.Fatalf("expected Authentication request, got %q (%q)", string(rune(mtype)), payload)
	}
	if len(payload) < 4 {
		c.t.Fatalf("authentication payload too short")
	}
	code := int32(binary.BigEndian.Uint32(payload[:4]))
	return code, payload[4:]
}

// expectError reads one ErrorResponse and returns it.
func (c *pgClient) expectError(why string) *pgErrorWire {
	c.t.Helper()
	for {
		mtype, payload := c.readMessage()
		if mtype != 'E' {
			c.t.Fatalf("%s: expected ErrorResponse, got %q (%q)", why, string(rune(mtype)), payload)
		}
		e := parseError(payload)
		if e == nil {
			c.t.Fatalf("%s: unparseable ErrorResponse %q", why, payload)
		}
		return e
	}
}

func parseError(payload []byte) *pgErrorWire {
	fields := map[byte]string{}
	i := 0
	for i < len(payload) && payload[i] != 0 {
		code := payload[i]
		i++
		end := bytes.IndexByte(payload[i:], 0)
		if end < 0 {
			return nil
		}
		fields[code] = string(payload[i : i+end])
		i += end + 1
	}
	return &pgErrorWire{Fields: fields}
}

// scramAuthenticate speaks the full SASL flow: AuthenticationSASL,
// SASLInitialResponse, AuthenticationSASLContinue, SASLResponse,
// AuthenticationSASLFinal, AuthenticationOk, parameters, backend key data
// and ReadyForQuery. secret empty simulates a WRONG secret (a valid-shaped
// proof that fails verification).
func (c *pgClient) scramAuthenticate(user, secret string) (serverSig string, status byte) {
	c.t.Helper()

	code, mechPayload := c.expectAuthRequest()
	if code != 10 {
		c.t.Fatalf("expected AuthenticationSASL (10), got %d", code)
	}
	mechs := strings.Split(strings.TrimSuffix(string(mechPayload), "\x00"), "\x00")
	found := false
	for _, m := range mechs {
		if m == "SCRAM-SHA-256" {
			found = true
		}
	}
	if !found {
		c.t.Fatalf("SCRAM-SHA-256 not offered: %q", mechPayload)
	}

	cnonce := make([]byte, 12)
	if _, err := rand.Read(cnonce); err != nil {
		c.t.Fatalf("nonce: %v", err)
	}
	clientFirstBare := "n=" + user + ",r=" + hex.EncodeToString(cnonce)
	initial := "n,," + clientFirstBare

	var initResp bytes.Buffer
	initResp.WriteString("SCRAM-SHA-256\x00")
	var initLen [4]byte
	binary.BigEndian.PutUint32(initLen[:], uint32(len(initial)))
	initResp.Write(initLen[:])
	initResp.WriteString(initial)
	c.sendMessage('p', initResp.Bytes())

	code, contPayload := c.expectAuthRequest()
	if code != 11 {
		c.t.Fatalf("expected AuthenticationSASLContinue (11), got %d (%q)", code, contPayload)
	}
	serverFirst := string(bytes.TrimSuffix(contPayload, []byte{0}))
	var combinedNonce, saltB64 string
	var iterations int
	for _, p := range strings.Split(serverFirst, ",") {
		switch {
		case strings.HasPrefix(p, "r="):
			combinedNonce = p[2:]
		case strings.HasPrefix(p, "s="):
			saltB64 = p[2:]
		case strings.HasPrefix(p, "i="):
			iterations, _ = strconv.Atoi(p[2:])
		}
	}
	if !strings.HasPrefix(combinedNonce, hex.EncodeToString(cnonce)) || iterations == 0 || saltB64 == "" {
		c.t.Fatalf("malformed server-first message %q", serverFirst)
	}
	salt, err := hexOrB64(saltB64)
	if err != nil {
		c.t.Fatalf("salt: %v", err)
	}

	clientFinalWithoutProof := "c=" + b64([]byte("n,,")) + ",r=" + combinedNonce
	authMessage := clientFirstBare + "," + serverFirst + "," + clientFinalWithoutProof
	proof := scramClientProof(secret, salt, iterations, authMessage)
	clientFinal := clientFinalWithoutProof + ",p=" + b64(proof)
	c.sendMessage('p', []byte(clientFinal))

	// AuthenticationSASLFinal carries the server signature; an
	// authentication failure arrives as an ErrorResponse instead.
	mtype, payload := c.readMessage()
	switch mtype {
	case 'R':
		if len(payload) < 4 {
			c.t.Fatalf("SASL final payload too short")
		}
		finalCode := int32(binary.BigEndian.Uint32(payload[:4]))
		if finalCode != 12 {
			c.t.Fatalf("expected AuthenticationSASLFinal (12), got %d", finalCode)
		}
		serverSig = strings.TrimPrefix(string(bytes.TrimSuffix(payload[4:], []byte{0})), "v=")
		// Verify the server signature with the client-side ServerKey.
		want := scramServerSignature(secret, salt, iterations, authMessage)
		if b64(want) != serverSig {
			c.t.Fatalf("server signature mismatch: got %s want %s", serverSig, b64(want))
		}
	case 'E':
		e := parseError(payload)
		c.t.Fatalf("authentication failed: %s %s", e.code(), e.message())
	default:
		c.t.Fatalf("unexpected message %q during SASL final", string(rune(mtype)))
	}

	// AuthenticationOk then parameters, backend key data, ReadyForQuery.
	code, _ = c.expectAuthRequest()
	if code != 0 {
		c.t.Fatalf("expected AuthenticationOk (0), got %d", code)
	}
	for {
		mtype, payload := c.readMessage()
		switch mtype {
		case 'S': // ParameterStatus
		case 'K': // BackendKeyData
		case 'Z':
			return serverSig, payload[0]
		default:
			c.t.Fatalf("unexpected message %q before ReadyForQuery", string(rune(mtype)))
		}
	}
}

func b64(b []byte) string {
	const std = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"
	return stdBase64(b, std)
}

func stdBase64(src []byte, alphabet string) string {
	// Small local base64 (StdEncoding) to keep the client self-contained.
	// Equivalent to encoding/base64 StdEncoding for these inputs.
	var out []byte
	for i := 0; i < len(src); i += 3 {
		var chunk [3]byte
		n := copy(chunk[:], src[i:])
		v := uint32(chunk[0])<<16 | uint32(chunk[1])<<8 | uint32(chunk[2])
		out = append(out, alphabet[(v>>18)&0x3f], alphabet[(v>>12)&0x3f])
		if n > 1 {
			out = append(out, alphabet[(v>>6)&0x3f])
		} else {
			out = append(out, '=')
		}
		if n > 2 {
			out = append(out, alphabet[v&0x3f])
		} else {
			out = append(out, '=')
		}
	}
	return string(out)
}

// hexOrB64 decodes the server salt (standard base64 with padding).
func hexOrB64(s string) ([]byte, error) {
	return stdBase64Decode(s)
}

func stdBase64Decode(s string) ([]byte, error) {
	const std = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"
	var out []byte
	var buf uint32
	var bits int
	for _, ch := range s {
		if ch == '=' {
			break
		}
		idx := strings.IndexRune(std, ch)
		if idx < 0 {
			return nil, fmt.Errorf("invalid base64 character %q", ch)
		}
		buf = buf<<6 | uint32(idx)
		bits += 6
		if bits >= 8 {
			bits -= 8
			out = append(out, byte(buf>>bits))
		}
	}
	return out, nil
}

// scramClientProof computes the RFC 5802 client proof for a secret.
func scramClientProof(secret string, salt []byte, iterations int, authMessage string) []byte {
	salted := pbkdf2SHA256Test([]byte(secret), salt, iterations, 32)
	clientKey := hmacSHA256Test(salted, []byte("Client Key"))
	storedKey := sha256.Sum256(clientKey)
	clientSignature := hmacSHA256Test(storedKey[:], []byte(authMessage))
	proof := make([]byte, 32)
	for i := range proof {
		proof[i] = clientKey[i] ^ clientSignature[i]
	}
	return proof
}

// scramServerSignature computes the expected server signature.
func scramServerSignature(secret string, salt []byte, iterations int, authMessage string) []byte {
	salted := pbkdf2SHA256Test([]byte(secret), salt, iterations, 32)
	serverKey := hmacSHA256Test(salted, []byte("Server Key"))
	return hmacSHA256Test(serverKey, []byte(authMessage))
}

func hmacSHA256Test(key, data []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write(data)
	return h.Sum(nil)
}

// pbkdf2SHA256Test mirrors the credential-side PBKDF2 (HMAC-SHA256).
func pbkdf2SHA256Test(password, salt []byte, iterations, keyLen int) []byte {
	prf := hmac.New(sha256.New, password)
	hashLen := prf.Size()
	numBlocks := (keyLen + hashLen - 1) / hashLen
	var counter [4]byte
	dk := make([]byte, 0, numBlocks*hashLen)
	U := make([]byte, hashLen)
	for block := 1; block <= numBlocks; block++ {
		prf.Reset()
		prf.Write(salt)
		counter[0] = byte(block >> 24)
		counter[1] = byte(block >> 16)
		counter[2] = byte(block >> 8)
		counter[3] = byte(block)
		prf.Write(counter[:])
		dk = prf.Sum(dk)
		T := dk[len(dk)-hashLen:]
		copy(U, T)
		for n := 2; n <= iterations; n++ {
			prf.Reset()
			prf.Write(U)
			U = U[:0]
			U = prf.Sum(U)
			for i := range T {
				T[i] ^= U[i]
			}
		}
	}
	return dk[:keyLen]
}

// ---------------------------------------------------------------------------
// Simple query protocol client
// ---------------------------------------------------------------------------

// queryResult is one simple-query batch result.
type queryResult struct {
	columns []string
	oids    []int32
	rows    [][]*string // nil entry = NULL
	tags    []string
	err     *pgErrorWire
	status  byte // ReadyForQuery status
}

// simpleQuery sends one Query message and collects the responses until
// ReadyForQuery.
func (c *pgClient) simpleQuery(q string) *queryResult {
	c.t.Helper()
	c.sendMessage('Q', append([]byte(q), 0))
	res := &queryResult{}
	for {
		mtype, payload := c.readMessage()
		switch mtype {
		case 'T':
			if len(payload) < 2 {
				c.t.Fatalf("RowDescription too short")
			}
			n := int(int16(binary.BigEndian.Uint16(payload[:2])))
			i := 2
			for k := 0; k < n; k++ {
				end := bytes.IndexByte(payload[i:], 0)
				if end < 0 {
					c.t.Fatalf("RowDescription unterminated name")
				}
				res.columns = append(res.columns, string(payload[i:i+end]))
				i += end + 1
				// table oid(4) attnum(2) type oid(4) typlen(2) typmod(4) format(2)
				oid := int32(binary.BigEndian.Uint32(payload[i+6 : i+10]))
				res.oids = append(res.oids, oid)
				i += 18
			}
		case 'D':
			if len(payload) < 2 {
				c.t.Fatalf("DataRow too short")
			}
			n := int(int16(binary.BigEndian.Uint16(payload[:2])))
			i := 2
			row := make([]*string, 0, n)
			for k := 0; k < n; k++ {
				l := int32(binary.BigEndian.Uint32(payload[i : i+4]))
				i += 4
				if l < 0 {
					row = append(row, nil)
					continue
				}
				v := string(payload[i : i+int(l)])
				i += int(l)
				row = append(row, &v)
			}
			res.rows = append(res.rows, row)
		case 'C':
			res.tags = append(res.tags, strings.TrimSuffix(string(payload), "\x00"))
		case 'I':
			// EmptyQueryResponse
		case 'E':
			res.err = parseError(payload)
			if res.err == nil {
				c.t.Fatalf("unparseable ErrorResponse %q", payload)
			}
		case 'S', 'K', 'N':
			// ParameterStatus / BackendKeyData / NoticeResponse
		case 'Z':
			res.status = payload[0]
			return res
		default:
			c.t.Fatalf("unexpected message %q in query response", string(rune(mtype)))
		}
	}
}

// ---------------------------------------------------------------------------
// Tests (the acceptance scenarios of this wave)
// ---------------------------------------------------------------------------

// TestPGGatewayStartWiring proves the wiring entry point itself: the
// listener the wiring step brings up accepts a TLS+SCRAM session with the
// same flow as the direct server.
func TestPGGatewayStartWiring(t *testing.T) {
	requireEngine(t)
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatalf("NewTestApp: %v", err)
	}
	defer app.Cleanup()
	rt, err := application.BindRuntime(app)
	if err != nil {
		t.Fatalf("BindRuntime: %v", err)
	}
	d, err := rt.Service.ProvisionManual(context.Background(), app.DataDir(), "pgw-wiring", "wal", "")
	if err != nil {
		t.Fatalf("provision: %v", err)
	}
	cred, err := rt.IssueCredential(context.Background(), d.ID, "pg", []string{"sql.read"}, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatalf("issue credential: %v", err)
	}
	ln, err := apis.StartPGGateway(app, "127.0.0.1:0", testTLSConfig(t), slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("StartPGGateway: %v", err)
	}
	defer ln.Close()

	c := dialPG(t, ln.Addr().String())
	c.sendStartup(cred.ID, d.ID)
	if _, status := c.scramAuthenticate(cred.ID, cred.Secret); status != 'I' {
		t.Fatalf("expected idle ReadyForQuery, got %q", status)
	}
	res := c.simpleQuery("SELECT 40+2")
	if res.err != nil || len(res.rows) != 1 || res.rows[0][0] == nil || *res.rows[0][0] != "42" {
		t.Fatalf("wiring query failed: err=%v rows=%v tags=%v", res.err, res.rows, res.tags)
	}
}

// TestPGGatewayAuthenticatesValidCredential proves: valid credentials
// authenticate (TLS + SCRAM-SHA-256 against the issued scoped credential).
func TestPGGatewayAuthenticatesValidCredential(t *testing.T) {
	fx := newGatewayFixture(t)

	c := dialPG(t, fx.addr)
	c.sendStartup(fx.rtCredentialIDForTest(t, fx.target1), fx.target1)
	_, status := c.scramAuthenticate(fx.rtCredentialIDForTest(t, fx.target1), fx.secret1)
	if status != 'I' {
		t.Fatalf("expected idle ReadyForQuery, got %q", status)
	}
}

// rtCredentialIDForTest looks up the credential id issued for a database
// (the fixture keeps only the secrets; the id is read from the registry).
func (fx *gatewayFixture) rtCredentialIDForTest(t *testing.T, databaseID string) string {
	t.Helper()
	items, err := fx.rt.ListCredentials(context.Background(), databaseID)
	if err != nil || len(items) == 0 {
		t.Fatalf("list credentials for %s: %v (%d items)", databaseID, err, len(items))
	}
	id, _ := items[0]["id"].(string)
	if id == "" {
		t.Fatalf("credential id missing for %s", databaseID)
	}
	return id
}

// TestPGGatewaySimpleQueryRoundTripsRows proves: a simple query round-trips
// rows (RowDescription, DataRows, CommandComplete, ReadyForQuery) over the
// authenticated session, including unmanaged-object DDL/DML.
func TestPGGatewaySimpleQueryRoundTripsRows(t *testing.T) {
	fx := newGatewayFixture(t)
	user := fx.rtCredentialIDForTest(t, fx.target1)

	c := dialPG(t, fx.addr)
	c.sendStartup(user, fx.target1)
	if _, status := c.scramAuthenticate(user, fx.secret1); status != 'I' {
		t.Fatalf("expected idle ReadyForQuery, got %q", status)
	}

	// Unmanaged DDL (allowed for sql.write; new objects are unmanaged SQL
	// objects, not invented collections).
	res := c.simpleQuery("CREATE TABLE pgw_items (name TEXT, qty INTEGER)")
	if res.err != nil || len(res.tags) != 1 || res.tags[0] != "CREATE TABLE" {
		t.Fatalf("CREATE TABLE: err=%v tags=%v", res.err, res.tags)
	}
	// DML
	res = c.simpleQuery("INSERT INTO pgw_items VALUES ('alpha', 1), ('beta', 2)")
	if res.err != nil || len(res.tags) != 1 || res.tags[0] != "INSERT 0 2" {
		t.Fatalf("INSERT: err=%v tags=%v", res.err, res.tags)
	}
	// The row round-trip: RowDescription names/types, DataRows, tag.
	res = c.simpleQuery("SELECT name, qty FROM pgw_items ORDER BY name")
	if res.err != nil {
		t.Fatalf("SELECT failed: %s %s", res.err.code(), res.err.message())
	}
	if len(res.columns) != 2 || res.columns[0] != "name" || res.columns[1] != "qty" {
		t.Fatalf("columns: %v", res.columns)
	}
	if len(res.rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(res.rows))
	}
	if res.rows[0][0] == nil || *res.rows[0][0] != "alpha" || res.rows[0][1] == nil || *res.rows[0][1] != "1" {
		t.Fatalf("row 0: %v", res.rows[0])
	}
	if res.rows[1][0] == nil || *res.rows[1][0] != "beta" || res.rows[1][1] == nil || *res.rows[1][1] != "2" {
		t.Fatalf("row 1: %v", res.rows[1])
	}
	if len(res.tags) != 1 || res.tags[0] != "SELECT 2" {
		t.Fatalf("tag: %v", res.tags)
	}
	if res.status != 'I' {
		t.Fatalf("expected idle ReadyForQuery, got %q", res.status)
	}
	if len(res.oids) != 2 || res.oids[0] != 25 /* text */ || res.oids[1] != 23 /* int4 */ {
		t.Fatalf("reported OIDs: %v", res.oids)
	}
}

// TestPGGatewayRejectsWrongCredentials proves: wrong credentials are
// rejected by the SCRAM exchange (a valid-shaped proof for a different
// secret must fail verification with 28P01 and must not authenticate).
func TestPGGatewayRejectsWrongCredentials(t *testing.T) {
	fx := newGatewayFixture(t)
	user := fx.rtCredentialIDForTest(t, fx.target1)

	c := dialPG(t, fx.addr)
	c.sendStartup(user, fx.target1)
	code, mechPayload := c.expectAuthRequest()
	if code != 10 {
		t.Fatalf("expected AuthenticationSASL, got %d", code)
	}
	if !strings.Contains(string(mechPayload), "SCRAM-SHA-256") {
		t.Fatalf("SCRAM mechanism not offered: %q", mechPayload)
	}

	cnonce := "cafecafecafecafecafecafecafecafe"
	clientFirstBare := "n=" + user + ",r=" + cnonce
	initial := "n,," + clientFirstBare
	var initResp bytes.Buffer
	initResp.WriteString("SCRAM-SHA-256\x00")
	var initLen [4]byte
	binary.BigEndian.PutUint32(initLen[:], uint32(len(initial)))
	initResp.Write(initLen[:])
	initResp.WriteString(initial)
	c.sendMessage('p', initResp.Bytes())

	code, contPayload := c.expectAuthRequest()
	if code != 11 {
		t.Fatalf("expected AuthenticationSASLContinue, got %d", code)
	}
	serverFirst := string(bytes.TrimSuffix(contPayload, []byte{0}))
	var combinedNonce, saltB64 string
	var iterations int
	for _, p := range strings.Split(serverFirst, ",") {
		switch {
		case strings.HasPrefix(p, "r="):
			combinedNonce = p[2:]
		case strings.HasPrefix(p, "s="):
			saltB64 = p[2:]
		case strings.HasPrefix(p, "i="):
			iterations, _ = strconv.Atoi(p[2:])
		}
	}
	if !strings.HasPrefix(combinedNonce, cnonce) {
		t.Fatalf("server nonce does not extend the client nonce: %q", combinedNonce)
	}
	salt, err := stdBase64Decode(saltB64)
	if err != nil {
		t.Fatalf("salt: %v", err)
	}

	// A proof computed from the WRONG secret.
	clientFinalWithoutProof := "c=" + b64([]byte("n,,")) + ",r=" + combinedNonce
	authMessage := clientFirstBare + "," + serverFirst + "," + clientFinalWithoutProof
	wrongProof := scramClientProof("definitely-not-the-secret", salt, iterations, authMessage)
	clientFinal := clientFinalWithoutProof + ",p=" + b64(wrongProof)
	c.sendMessage('p', []byte(clientFinal))

	e := c.expectError("wrong credentials")
	if e.code() != "28P01" {
		t.Fatalf("expected 28P01 invalid_password, got %s (%s)", e.code(), e.message())
	}
}

// TestPGGatewayRefusesCredentialScopedToAnotherDatabase proves: a
// credential scoped to another database is refused (post-SCRAM binding
// check, 28000), and an unknown database selector fails before
// authentication (3D000).
func TestPGGatewayRefusesCredentialScopedToAnotherDatabase(t *testing.T) {
	fx := newGatewayFixture(t)
	// credential 2 is scoped to target2; connecting it to target1 must be
	// refused (the exchange itself succeeds — the credential is genuine).
	user2 := fx.rtCredentialIDForTest(t, fx.target2)
	c := dialPG(t, fx.addr)
	c.sendStartup(user2, fx.target1)
	c.scramAuthenticateExpectBindingRefusal(user2, fx.secret2)
}

// scramAuthenticateExpectBindingRefusal runs the exchange and expects the
// connection to be refused AFTER the SASL final (28000, cross-database
// selector).
func (c *pgClient) scramAuthenticateExpectBindingRefusal(user, secret string) (string, byte) {
	c.t.Helper()
	// Reuse the full exchange; it must stop at the binding check.
	code, mechPayload := c.expectAuthRequest()
	if code != 10 {
		c.t.Fatalf("expected AuthenticationSASL, got %d", code)
	}
	if !strings.Contains(string(mechPayload), "SCRAM-SHA-256") {
		c.t.Fatalf("SCRAM mechanism not offered: %q", mechPayload)
	}
	cnonce := make([]byte, 12)
	if _, err := rand.Read(cnonce); err != nil {
		c.t.Fatalf("nonce: %v", err)
	}
	clientFirstBare := "n=" + user + ",r=" + hex.EncodeToString(cnonce)
	initial := "n,," + clientFirstBare
	var initResp bytes.Buffer
	initResp.WriteString("SCRAM-SHA-256\x00")
	var initLen [4]byte
	binary.BigEndian.PutUint32(initLen[:], uint32(len(initial)))
	initResp.Write(initLen[:])
	initResp.WriteString(initial)
	c.sendMessage('p', initResp.Bytes())

	code, contPayload := c.expectAuthRequest()
	if code != 11 {
		c.t.Fatalf("expected AuthenticationSASLContinue, got %d", code)
	}
	serverFirst := string(bytes.TrimSuffix(contPayload, []byte{0}))
	var combinedNonce, saltB64 string
	var iterations int
	for _, p := range strings.Split(serverFirst, ",") {
		switch {
		case strings.HasPrefix(p, "r="):
			combinedNonce = p[2:]
		case strings.HasPrefix(p, "s="):
			saltB64 = p[2:]
		case strings.HasPrefix(p, "i="):
			iterations, _ = strconv.Atoi(p[2:])
		}
	}
	salt, err := stdBase64Decode(saltB64)
	if err != nil {
		c.t.Fatalf("salt: %v", err)
	}
	clientFinalWithoutProof := "c=" + b64([]byte("n,,")) + ",r=" + combinedNonce
	authMessage := clientFirstBare + "," + serverFirst + "," + clientFinalWithoutProof
	proof := scramClientProof(secret, salt, iterations, authMessage)
	c.sendMessage('p', []byte(clientFinalWithoutProof+",p="+b64(proof)))

	mtype, payload := c.readMessage()
	if mtype != 'R' {
		e := parseError(payload)
		if mtype == 'E' && e != nil {
			c.t.Fatalf("unexpected error after client-final: %s %s", e.code(), e.message())
		}
		c.t.Fatalf("expected AuthenticationSASLFinal, got %q", string(rune(mtype)))
	}
	_ = payload

	// The binding refusal (the credential is scoped to another database).
	e := c.expectError("cross-database credential")
	if e.code() != "28000" {
		c.t.Fatalf("expected 28000 invalid_authorization_specification, got %s (%s)", e.code(), e.message())
	}
	return "", 0
}

// TestPGGatewayRefusesUnknownDatabase proves the unknown database
// selector fails before authentication.
func TestPGGatewayRefusesUnknownDatabase(t *testing.T) {
	fx := newGatewayFixture(t)
	user := fx.rtCredentialIDForTest(t, fx.target1)
	c := dialPG(t, fx.addr)
	c.sendStartup(user, "0000000000000000000000000000ff")
	e := c.expectError("unknown database")
	if e.code() != "3D000" {
		t.Fatalf("expected 3D000 invalid_catalog_name, got %s (%s)", e.code(), e.message())
	}
}

// TestPGGatewayRequiresTLSAndRejectsTrustFallback proves the TLS-only
// policy: a plaintext startup packet is refused with an ErrorResponse and
// never reaches an AuthenticationOk (no trust fallback).
func TestPGGatewayRequiresTLSAndRejectsTrustFallback(t *testing.T) {
	fx := newGatewayFixture(t)
	user := fx.rtCredentialIDForTest(t, fx.target1)

	c := dialPlaintext(t, fx.addr, user, fx.target1)
	e := c.expectError("plaintext startup")
	if e.code() != "28000" {
		t.Fatalf("expected 28000 for the plaintext startup, got %s (%s)", e.code(), e.message())
	}
	if !strings.Contains(strings.ToLower(e.message()), "tls") {
		t.Fatalf("expected a TLS-required message, got %q", e.message())
	}
}

// TestPGGatewayEnforcesSQLReadScope proves the parsed scope enforcement:
// a sql.read-only credential refuses writes with 42501 while SELECT works.
func TestPGGatewayEnforcesSQLReadScope(t *testing.T) {
	requireEngine(t)
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatalf("NewTestApp: %v", err)
	}
	defer app.Cleanup()
	rt, err := application.BindRuntime(app)
	if err != nil {
		t.Fatalf("BindRuntime: %v", err)
	}
	d, err := rt.Service.ProvisionManual(context.Background(), app.DataDir(), "pgw-readonly", "wal", "")
	if err != nil {
		t.Fatalf("provision: %v", err)
	}
	cred, err := rt.IssueCredential(context.Background(), d.ID, "pg", []string{"sql.read"}, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatalf("issue credential: %v", err)
	}
	server := pgateway.New(rt, pgateway.Config{Addr: "127.0.0.1:0", TLS: testTLSConfig(t)},
		slog.New(slog.NewTextHandler(io.Discard, nil)))
	ln, err := server.Listen()
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer func() { _ = ln.Close(); server.Close() }()

	c := dialPG(t, ln.Addr().String())
	c.sendStartup(cred.ID, d.ID)
	if _, status := c.scramAuthenticate(cred.ID, cred.Secret); status != 'I' {
		t.Fatalf("expected idle ReadyForQuery, got %q", status)
	}

	res := c.simpleQuery("SELECT 1+1")
	if res.err != nil || len(res.rows) != 1 || res.rows[0][0] == nil || *res.rows[0][0] != "2" {
		t.Fatalf("read-only SELECT: err=%v rows=%v", res.err, res.rows)
	}
	res = c.simpleQuery("CREATE TABLE pgw_denied (id TEXT)")
	if res.err == nil || res.err.code() != "42501" {
		t.Fatalf("expected 42501 for the write denial, got %+v", res.err)
	}
	res = c.simpleQuery("SELECT load_extension('x')")
	if res.err == nil || res.err.code() != "42501" {
		t.Fatalf("expected 42501 for the extension denial, got %+v", res.err)
	}
	res = c.simpleQuery("PRAGMA journal_mode=WAL")
	if res.err == nil || res.err.code() != "42501" {
		t.Fatalf("expected 42501 for the unsafe pragma, got %+v", res.err)
	}
}

// TestPGGatewayProtectedMetadata proves the protected metadata namespace:
// writes to the MiraBase registry/PocketBase system tables are refused on
// every credential (sql.write cannot alter protected application
// metadata).
func TestPGGatewayProtectedMetadata(t *testing.T) {
	fx := newGatewayFixture(t)
	user := fx.rtCredentialIDForTest(t, fx.target1)

	c := dialPG(t, fx.addr)
	c.sendStartup(user, fx.target1)
	if _, status := c.scramAuthenticate(user, fx.secret1); status != 'I' {
		t.Fatalf("expected idle ReadyForQuery, got %q", status)
	}
	res := c.simpleQuery("CREATE TABLE _mb_evil (id TEXT)")
	if res.err == nil || res.err.code() != "42501" {
		t.Fatalf("expected 42501 for the protected namespace, got %+v", res.err)
	}
}

// TestPGGatewayTransactionBoundaries proves the transaction state machine:
// BEGIN/COMMIT report the in-transaction status, an error inside the
// block fails it (25P02 for later statements) and ROLLBACK recovers.
func TestPGGatewayTransactionBoundaries(t *testing.T) {
	fx := newGatewayFixture(t)
	user := fx.rtCredentialIDForTest(t, fx.target1)

	c := dialPG(t, fx.addr)
	c.sendStartup(user, fx.target1)
	if _, status := c.scramAuthenticate(user, fx.secret1); status != 'I' {
		t.Fatalf("expected idle ReadyForQuery, got %q", status)
	}

	res := c.simpleQuery("CREATE TABLE pgw_tx (id TEXT)")
	if res.err != nil {
		t.Fatalf("CREATE TABLE: %v", res.err)
	}
	res = c.simpleQuery("BEGIN")
	if res.err != nil || res.status != 'T' {
		t.Fatalf("BEGIN: err=%v status=%q", res.err, res.status)
	}
	res = c.simpleQuery("INSERT INTO pgw_tx VALUES ('x')")
	if res.err != nil || len(res.tags) != 1 || res.tags[0] != "INSERT 0 1" {
		t.Fatalf("INSERT in tx: err=%v tags=%v", res.err, res.tags)
	}
	// A failed statement inside the block.
	res = c.simpleQuery("INSERT INTO pgw_missing_table VALUES ('x')")
	if res.err == nil {
		t.Fatalf("expected an error for the missing table")
	}
	if res.status != 'E' {
		t.Fatalf("expected failed status 'E', got %q", res.status)
	}
	// Later statements are refused while the block is failed.
	res = c.simpleQuery("SELECT 1")
	if res.err == nil || res.err.code() != "25P02" {
		t.Fatalf("expected 25P02 in the failed block, got %+v", res.err)
	}
	// ROLLBACK recovers.
	res = c.simpleQuery("ROLLBACK")
	if res.err != nil || res.status != 'I' {
		t.Fatalf("ROLLBACK: err=%v status=%q", res.err, res.status)
	}
	// The failed INSERT was rolled back with the block.
	res = c.simpleQuery("SELECT count(*) FROM pgw_tx")
	if res.err != nil || len(res.rows) != 1 || res.rows[0][0] == nil || *res.rows[0][0] != "0" {
		t.Fatalf("count after rollback: err=%v rows=%v", res.err, res.rows)
	}
}

// TestPGGatewayEngineRoundTripThroughStart closes the loop on the
// listener + engine handle path (the runtime handle cache serves the
// gateway connection through the shared *sql.DB).
func TestPGGatewayEngineRoundTripThroughStart(t *testing.T) {
	requireEngine(t)
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatalf("NewTestApp: %v", err)
	}
	defer app.Cleanup()
	rt, err := application.BindRuntime(app)
	if err != nil {
		t.Fatalf("BindRuntime: %v", err)
	}
	d, err := rt.Service.ProvisionManual(context.Background(), app.DataDir(), "pgw-engine", "wal", "")
	if err != nil {
		t.Fatalf("provision: %v", err)
	}
	// Seed data through the record-store handle (independent access path)
	// and read it through the gateway with a sql.write credential.
	handle, err := rt.Handles.Open(context.Background(), d.ID)
	if err != nil {
		t.Fatalf("open handle: %v", err)
	}
	if _, err := handle.ExecContext(context.Background(), "CREATE TABLE seeded (k TEXT, v INTEGER)"); err != nil {
		t.Fatalf("seed create: %v", err)
	}
	if _, err := handle.ExecContext(context.Background(), "INSERT INTO seeded VALUES ('k1', 7)"); err != nil {
		t.Fatalf("seed insert: %v", err)
	}
	cred, err := rt.IssueCredential(context.Background(), d.ID, "pg", []string{"sql.read", "sql.write"}, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatalf("issue credential: %v", err)
	}
	server := pgateway.New(rt, pgateway.Config{Addr: "127.0.0.1:0", TLS: testTLSConfig(t)},
		slog.New(slog.NewTextHandler(io.Discard, nil)))
	ln, err := server.Listen()
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer func() { _ = ln.Close(); server.Close() }()

	c := dialPG(t, ln.Addr().String())
	c.sendStartup(cred.ID, d.ID)
	if _, status := c.scramAuthenticate(cred.ID, cred.Secret); status != 'I' {
		t.Fatalf("expected idle ReadyForQuery, got %q", status)
	}
	res := c.simpleQuery("SELECT k, v FROM seeded")
	if res.err != nil {
		t.Fatalf("SELECT: %s %s", res.err.code(), res.err.message())
	}
	if len(res.rows) != 1 || res.rows[0][0] == nil || *res.rows[0][0] != "k1" || *res.rows[0][1] != "7" {
		t.Fatalf("seeded row: %v", res.rows)
	}
}

// TestPGGatewayPrimaryAliasRefusal proves the primary alias binding: a
// target-scoped credential cannot connect with the reserved primary
// alias, and the primary-scoped credential CAN (whole-database
// administrative SQL, analogous to the superuser SQL console).
func TestPGGatewayPrimaryAliasBinding(t *testing.T) {
	fx := newGatewayFixture(t)
	// A target-scoped credential against the primary alias: the exchange
	// completes (the credential is genuine) and the binding refusal
	// arrives after the SASL final (28000, cross-database selector).
	user1 := fx.rtCredentialIDForTest(t, fx.target1)
	c := dialPG(t, fx.addr)
	c.sendStartup(user1, "primary")
	c.scramAuthenticateExpectBindingRefusal(user1, fx.secret1)

	// A primary-scoped credential connects to the primary database and
	// reads (read-only SELECT; writes to protected metadata stay refused).
	primCred, err := fx.rt.IssueCredential(context.Background(), "", "pg", []string{"sql.read"}, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatalf("issue primary credential: %v", err)
	}
	c2 := dialPG(t, fx.addr)
	c2.sendStartup(primCred.ID, "primary")
	if _, status := c2.scramAuthenticate(primCred.ID, primCred.Secret); status != 'I' {
		t.Fatalf("expected idle ReadyForQuery, got %q", status)
	}
	res := c2.simpleQuery("SELECT 11*11")
	if res.err != nil || len(res.rows) != 1 || res.rows[0][0] == nil || *res.rows[0][0] != "121" {
		t.Fatalf("primary SELECT: err=%v rows=%v", res.err, res.rows)
	}
	res = c2.simpleQuery("UPDATE _collections SET id = id")
	if res.err == nil || res.err.code() != "42501" {
		t.Fatalf("expected 42501 for the protected metadata write, got %+v", res.err)
	}
}
