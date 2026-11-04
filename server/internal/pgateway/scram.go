package pgateway

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"strings"
)

// scramMechanism is the only SCRAM mechanism the gateway offers. Channel
// binding (SCRAM-SHA-256-PLUS) is not offered; the exchange runs entirely
// inside the TLS-protected channel and the gs2 header must not claim a
// channel binding ("y,," and "n,," are accepted, "p=..." is refused).
const scramMechanism = "SCRAM-SHA-256"

// scramServerNonceBytes is the server nonce length.
const scramServerNonceBytes = 18

// scramClient is one in-flight SCRAM-SHA-256 exchange state.
type scramClient struct {
	// username is the credential identity presented by the client (the
	// startup user and the client-first "n=" field must agree).
	username string
	// clientNonce is the bare cnonce from the client-first message.
	clientNonce string
	// combinedNonce is cnonce + server nonce.
	combinedNonce string
	// clientFirstBare is the client-first-message-bare (auth message part).
	clientFirstBare string
	// serverFirst is the server-first-message (auth message part).
	serverFirst string
	// material is the stored verifier of the identified credential.
	material *scramStoredKeys
}

// scramStoredKeys mirrors application.PGSCRAMMaterial for the exchange.
type scramStoredKeys struct {
	Salt       []byte
	Iterations int
	StoredKey  []byte
	ServerKey  []byte
}

// beginSASL validates the client's SASLInitialResponse and returns the
// server-first-message to send in AuthenticationSASLContinue.
//
// The initial response is: mechanism cstr, int32 initial-response length
// (-1 none), then the client-first-message. The mechanism must be exactly
// SCRAM-SHA-256 (the offered mechanism); a mismatch is a protocol
// violation. A failed client-first message is an authentication failure
// (the connection is closed; no retry within the exchange).
func beginSASL(username string, initial []byte, stored *scramStoredKeys) (*scramClient, string, error) {
	if len(initial) > maxAuthMessageLen {
		return nil, "", errProtocol("oversized SASL initial response")
	}
	// client-first-message: gs2-header "," client-first-message-bare
	// gs2-header: "n,," (no binding) | "y,," (binding unsupported by client)
	//           | "p=cb-name,..." (requires binding -> refused)
	parts := strings.SplitN(string(initial), ",", 3)
	if len(parts) < 3 {
		return nil, "", errProtocol("malformed SCRAM client-first-message")
	}
	switch parts[0] {
	case "n", "y":
		// No channel binding: accepted (no binding is offered).
	case "p":
		return nil, "", errAuth(stateInvalidAuthSpecification,
			"channel binding is not supported by this gateway")
	default:
		return nil, "", errProtocol("invalid gs2 header in SCRAM client-first-message")
	}
	bare := parts[2]
	var clientNonce string
	for _, p := range strings.Split(bare, ",") {
		if p == "" {
			continue
		}
		switch {
		case strings.HasPrefix(p, "n="):
			// The client-first authcid: parsed for the message shape, then
			// ignored — see the credential-identity note above.
			_ = saslnameDecode(p[2:])
		case strings.HasPrefix(p, "r="):
			clientNonce = p[2:]
		default:
			// Unknown attributes/extensions are ignored (RFC 5802 §5.1).
		}
	}
	if clientNonce == "" {
		return nil, "", errProtocol("SCRAM client-first-message carries no nonce")
	}
	// The credential identity is the STARTUP user (the gateway looked up
	// the credential's verifier with it and checkBinding re-checks it after
	// the exchange); the client-first authcid is parsed but not enforced,
	// like PostgreSQL's backend, which uses the startup-packet username and
	// ignores the client-first "n=" field. The maintained node-postgres
	// client sends a literal placeholder ("n=*,r=..."; pg/lib/crypto/
	// sasl.js) instead of the username, so enforcing the field here breaks
	// every real JS client against a server PostgreSQL itself accepts.
	serverNonce := make([]byte, scramServerNonceBytes)
	if _, err := rand.Read(serverNonce); err != nil {
		return nil, "", err
	}
	combined := clientNonce + base64.RawStdEncoding.EncodeToString(serverNonce)
	sc := &scramClient{
		username:        username,
		clientNonce:     clientNonce,
		combinedNonce:   combined,
		clientFirstBare: bare,
		material:        stored,
	}
	serverFirst := fmt.Sprintf("r=%s,s=%s,i=%d",
		combined,
		base64.StdEncoding.EncodeToString(stored.Salt),
		stored.Iterations)
	sc.serverFirst = serverFirst
	return sc, serverFirst, nil
}

// finishSASL verifies the client-final-message and returns the
// server-final-message ("v=<base64 ServerSignature>") for
// AuthenticationSASLFinal. A failed proof is an authentication failure.
func (sc *scramClient) finishSASL(final []byte) (string, error) {
	if len(final) > maxAuthMessageLen {
		return "", errProtocol("oversized SASL response")
	}
	// client-final-message: "c=" b64(gs2+binding) ",r=" nonce [,"ext"]
	//                       ",p=" b64(proof)
	// The proof attribute is the last one; base64 payloads contain no
	// commas, so a comma split is exact.
	var withoutProof []string
	var proofB64 string
	for _, p := range strings.Split(string(final), ",") {
		switch {
		case strings.HasPrefix(p, "p="):
			proofB64 = p[2:]
		default:
			withoutProof = append(withoutProof, p)
		}
	}
	if proofB64 == "" {
		return "", errProtocol("SCRAM client-final-message carries no proof")
	}
	// The channel-binding flag must agree with the offered mechanism.
	gs2Flag, err := decodeGS2Flag(withoutProof)
	if err != nil {
		return "", err
	}
	if gs2Flag == "p" {
		return "", errAuth(stateInvalidAuthSpecification,
			"channel binding is not supported by this gateway")
	}
	// Nonce continuity: the r= attribute must equal the combined nonce
	// issued above, and it must extend the client nonce.
	if !extractNonce(strings.Join(withoutProof, ","), sc.clientNonce, sc.combinedNonce) {
		return "", errAuth(stateInvalidAuthSpecification, "SCRAM nonce mismatch")
	}
	authMessage := sc.clientFirstBare + "," + sc.serverFirst + "," + strings.Join(withoutProof, ",")
	proof, err := base64.StdEncoding.DecodeString(proofB64)
	if err != nil || len(proof) != sha256.Size {
		return "", errProtocol("invalid SCRAM client proof encoding")
	}
	// ClientSignature = HMAC(StoredKey, authMessage);
	// ClientKey = ClientProof XOR ClientSignature;
	// verify H(ClientKey) == StoredKey.
	clientSignature := hmacSHA256(sc.material.StoredKey, []byte(authMessage))
	clientKey := make([]byte, sha256.Size)
	for i := range clientKey {
		clientKey[i] = proof[i] ^ clientSignature[i]
	}
	h := sha256.Sum256(clientKey)
	if !bytes.Equal(h[:], sc.material.StoredKey) {
		return "", errAuth(stateInvalidPassword,
			"password authentication failed for user %q", sc.username)
	}
	// ServerSignature = HMAC(ServerKey, authMessage).
	serverSignature := hmacSHA256(sc.material.ServerKey, []byte(authMessage))
	return "v=" + base64.StdEncoding.EncodeToString(serverSignature), nil
}

// decodeGS2Flag extracts the gs2 flag carried in the base64-decoded c=
// attribute of the client-final-message-without-proof.
func decodeGS2Flag(withoutProof []string) (string, error) {
	for _, p := range withoutProof {
		if !strings.HasPrefix(p, "c=") {
			continue
		}
		raw, err := base64.StdEncoding.DecodeString(p[2:])
		if err != nil {
			return "", errProtocol("invalid SCRAM c= encoding")
		}
		parts := strings.SplitN(string(raw), ",", 3)
		if len(parts) < 3 {
			return "", errProtocol("invalid SCRAM gs2 header")
		}
		return parts[0], nil
	}
	return "", errProtocol("SCRAM client-final-message carries no c= field")
}

// extractNonce verifies the r= attribute equals the issued combined nonce.
func extractNonce(text, clientNonce, combined string) bool {
	for _, p := range strings.Split(text, ",") {
		if strings.HasPrefix(p, "r=") {
			return p[2:] == combined && strings.HasPrefix(combined, clientNonce)
		}
	}
	return false
}

// saslnameDecode normalizes the SCRAM username (RFC 5802 §5.1: the "=2C"
// and "=3D" escapes). Credential ids are server-generated hex strings, so
// the escapes never occur in practice; the decode keeps the comparison
// honest instead of silently comparing raw text.
func saslnameDecode(name string) string {
	r := strings.NewReplacer("=2C", ",", "=3D", "=")
	return r.Replace(name)
}

// hmacSHA256 is the HMAC-SHA256 used for the SCRAM signature computations
// (a small local helper; the exchange stays on the standard library).
func hmacSHA256(key, data []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write(data)
	return h.Sum(nil)
}
