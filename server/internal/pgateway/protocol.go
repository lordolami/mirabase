package pgateway

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
)

// PostgreSQL wire protocol constants (protocol version 3).
const (
	protocolVersion3 = 196608 // 3 << 16

	sslRequestCode  = 80877103
	gssEncRequest   = 80877104
	cancelRequest   = 80877102
	cancelRequestSz = 16
)

// Frontend message types.
const (
	msgBind           = 'B'
	msgCancelSASL     = 'f' // SASL exchange abort
	msgCloseStmt      = 'C'
	msgDescribe       = 'D'
	msgExecute        = 'E'
	msgFlush          = 'H'
	msgParse          = 'P'
	msgPassword       = 'p'
	msgQuery          = 'Q'
	msgSync           = 'S'
	msgTerminate      = 'X'
	msgSASLInitial    = 'p'
	msgSASLResponse   = 'p'
	msgGSSEncResponse = 'p'
)

// Backend message types.
const (
	respAuthRequest           = 'R'
	respBackendKeyData        = 'K'
	respCommandComplete       = 'C'
	respDataRow               = 'D'
	respEmptyQuery            = 'I'
	respError                 = 'E'
	respNotice                = 'N'
	respParameterStatus       = 'S'
	respReadyForQuery         = 'Z'
	respRowDescription        = 'T'
	respNegotiateProtocolVer  = 'v'
	respParseComplete         = '1'
	respBindComplete          = '2'
	respCloseComplete         = '3'
	respParameterDescription  = 't'
	respNoData                = 'n'
	respPortalSuspended       = 's'
)

// Authentication request codes.
const (
	authOK              = 0
	authCleartext       = 3
	authMD5             = 5
	authSASL            = 10
	authSASLContinue    = 11
	authSASLFinal       = 12
	authGSS             = 7
	authSSPI            = 9
	authGSSContinue     = 8
	authSCMCreds        = 6
	authKerberosV5      = 2
	authPasswordMessage = 3
)

// Message size bounds (fail-safe limits; a message beyond them is a
// protocol violation, never a memory commitment).
const (
	maxStartupMessageLen = 10000
	maxCancelMessageLen  = 32
	maxRegularMessageLen = 16 << 20 // 16 MiB
	maxAuthMessageLen    = 4096
)

// readMessageLength reads one length-prefixed frame header from br. The
// returned length INCLUDES the 4 length bytes but excludes nothing else;
// the type byte is consumed by the caller for regular messages.
func readMessageLength(br *bufio.Reader, max int) (int, error) {
	var lenBuf [4]byte
	if _, err := io.ReadFull(br, lenBuf[:]); err != nil {
		return 0, err
	}
	n := int(binary.BigEndian.Uint32(lenBuf[:]))
	if n < 4 || n > max {
		return 0, errProtocol("invalid message length %d", n)
	}
	return n, nil
}

// writeMessage writes one regular backend message (type byte + length +
// payload).
func writeMessage(w io.Writer, msgType byte, payload []byte) (int, error) {
	buf := make([]byte, 0, 5+len(payload))
	buf = append(buf, msgType)
	var lenBuf [4]byte
	binary.BigEndian.PutUint32(lenBuf[:], uint32(4+len(payload)))
	buf = append(buf, lenBuf[:]...)
	buf = append(buf, payload...)
	return w.Write(buf)
}

// writeReadyForQuery writes ReadyForQuery with the given transaction
// status byte ('I' idle, 'T' in transaction, 'E' failed).
func writeReadyForQuery(w io.Writer, status byte) error {
	_, err := writeMessage(w, respReadyForQuery, []byte{status})
	return err
}

// writeParameterStatus writes one ParameterStatus message.
func writeParameterStatus(w io.Writer, key, value string) error {
	payload := make([]byte, 0, len(key)+len(value)+2)
	payload = append(payload, key...)
	payload = append(payload, 0)
	payload = append(payload, value...)
	payload = append(payload, 0)
	_, err := writeMessage(w, respParameterStatus, payload)
	return err
}

// writeAuthRequest writes an Authentication* request with a payload of
// int32 values and optional string data.
func writeAuthRequest(w io.Writer, code int32, extra []byte) error {
	payload := make([]byte, 4, 4+len(extra))
	binary.BigEndian.PutUint32(payload, uint32(code))
	payload = append(payload, extra...)
	_, err := writeMessage(w, respAuthRequest, payload)
	return err
}

// readCString reads one NUL-terminated string from br.
func readCString(br *bufio.Reader, max int) (string, error) {
	var out []byte
	for {
		c, err := br.ReadByte()
		if err != nil {
			return "", err
		}
		if c == 0 {
			return string(out), nil
		}
		out = append(out, c)
		if len(out) > max {
			return "", errProtocol("overlong string field")
		}
	}
}

// startupParams is the parsed StartupMessage parameter set.
type startupParams struct {
	params []startupParam
	values map[string]string
}

type startupParam struct {
	key   string
	value string
}

// get returns the value of one startup parameter ("", absent).
func (s *startupParams) get(key string) string {
	if s.values == nil {
		return ""
	}
	return s.values[key]
}

// parseStartupMessage parses the StartupMessage payload (after the 4
// length bytes): int32 protocol version + NUL-terminated key/value pairs.
func parseStartupMessage(payload []byte) (*startupParams, error) {
	if len(payload) < 4 {
		return nil, errProtocol("startup message too short")
	}
	version := binary.BigEndian.Uint32(payload[:4])
	if version != protocolVersion3 {
		return nil, newPGError(stateFeatureNotSupported,
			fmt.Sprintf("unsupported protocol version %d.%d; this gateway speaks protocol 3.0",
				version>>16, version&0xffff))
	}
	s := &startupParams{values: map[string]string{}}
	rest := payload[4:]
	for len(rest) > 0 && rest[0] != 0 {
		key, err := readCStrBytes(&rest)
		if err != nil {
			return nil, err
		}
		value, err := readCStrBytes(&rest)
		if err != nil {
			return nil, err
		}
		if key == "" {
			return nil, errProtocol("empty startup parameter name")
		}
		s.params = append(s.params, startupParam{key: key, value: value})
		s.values[key] = value
	}
	return s, nil
}

// readCStrBytes reads one NUL-terminated string from a byte slice and
// ADVANCES past the NUL (every caller gets the remainder after the
// terminator).
func readCStrBytes(b *[]byte) (string, error) {
	end := -1
	for i, c := range *b {
		if c == 0 {
			end = i
			break
		}
	}
	if end < 0 {
		return "", errProtocol("unterminated startup parameter")
	}
	v := string((*b)[:end])
	*b = (*b)[end+1:]
	return v, nil
}
