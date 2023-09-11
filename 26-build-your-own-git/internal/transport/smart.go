package transport

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// ZeroOID is the all-zero OID used for creating a new ref.
const ZeroOID = "0000000000000000000000000000000000000000"

// Auth carries HTTP basic-auth credentials. Tokens are never logged.
type Auth struct {
	User  string
	Token string
}

// RemoteRef is an advertised remote reference.
type RemoteRef struct {
	Name string
	OID  string
}

// RefUpdate is a single ref change in a receive-pack command.
type RefUpdate struct {
	Old string
	New string
	Ref string
}

// pktLine encodes a single pkt-line (with its 4-byte hex length prefix).
func pktLine(s string) []byte {
	n := len(s) + 4
	return []byte(fmt.Sprintf("%04x%s", n, s))
}

const flushPkt = "0000"

// readPktLines returns the payload of every pkt-line in r, with trailing
// newlines kept. A flush-pkt comes back as "".
func readPktLines(r *bufio.Reader) ([]string, error) {
	var lines []string
	for {
		var lenBuf [4]byte
		if _, err := io.ReadFull(r, lenBuf[:]); err != nil {
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				break
			}
			return nil, err
		}
		var length int
		if _, err := fmt.Sscanf(string(lenBuf[:]), "%04x", &length); err != nil {
			return nil, fmt.Errorf("bad pkt-line length %q", lenBuf)
		}
		if length == 0 {
			lines = append(lines, "")
			continue
		}
		if length < 4 {
			return nil, fmt.Errorf("invalid pkt-line length %d", length)
		}
		payload := make([]byte, length-4)
		if _, err := io.ReadFull(r, payload); err != nil {
			return nil, err
		}
		lines = append(lines, string(payload))
	}
	return lines, nil
}

func applyAuth(req *http.Request, auth Auth) {
	if auth.Token != "" || auth.User != "" {
		user := auth.User
		if user == "" {
			user = "x-access-token"
		}
		req.SetBasicAuth(user, auth.Token)
	}
}

// DiscoverReceivePack performs smart-HTTP ref discovery for git-receive-pack.
func DiscoverReceivePack(client *http.Client, remoteURL string, auth Auth) ([]RemoteRef, error) {
	if client == nil {
		client = http.DefaultClient
	}
	u := strings.TrimRight(remoteURL, "/") + "/info/refs?service=git-receive-pack"
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "gg/0.1")
	applyAuth(req, auth)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ref discovery: server returned %s", resp.Status)
	}

	br := bufio.NewReader(resp.Body)
	lines, err := readPktLines(br)
	if err != nil {
		return nil, err
	}
	return parseRefAdvertisement(lines), nil
}

// parseRefAdvertisement extracts refs from a service ref advertisement.
func parseRefAdvertisement(lines []string) []RemoteRef {
	var refs []RemoteRef
	for _, line := range lines {
		if line == "" {
			continue
		}
		// Skip the service announcement line.
		if strings.HasPrefix(line, "# service=") {
			continue
		}
		line = strings.TrimRight(line, "\n")
		// First ref line carries capabilities after a NUL.
		if nul := strings.IndexByte(line, 0); nul >= 0 {
			line = line[:nul]
		}
		fields := strings.SplitN(line, " ", 2)
		if len(fields) != 2 || len(fields[0]) != 40 {
			continue
		}
		if fields[1] == "capabilities^{}" {
			continue
		}
		refs = append(refs, RemoteRef{OID: fields[0], Name: fields[1]})
	}
	return refs
}

// ReceivePack POSTs ref-update commands followed by a packfile, returning the
// server's report-status text.
func ReceivePack(client *http.Client, remoteURL string, auth Auth, updates []RefUpdate, pack []byte) (string, error) {
	if client == nil {
		client = http.DefaultClient
	}
	if len(updates) == 0 {
		return "", fmt.Errorf("no ref updates to send")
	}

	var body bytes.Buffer
	for i, up := range updates {
		line := fmt.Sprintf("%s %s %s", up.Old, up.New, up.Ref)
		if i == 0 {
			// Advertise the capability we understand on the first command.
			line += "\x00 report-status"
		}
		line += "\n"
		body.Write(pktLine(line))
	}
	body.WriteString(flushPkt)
	body.Write(pack)

	u := strings.TrimRight(remoteURL, "/") + "/git-receive-pack"
	req, err := http.NewRequest(http.MethodPost, u, bytes.NewReader(body.Bytes()))
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "gg/0.1")
	req.Header.Set("Content-Type", "application/x-git-receive-pack-request")
	req.Header.Set("Accept", "application/x-git-receive-pack-result")
	applyAuth(req, auth)

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return "", fmt.Errorf("receive-pack: server returned %s: %s", resp.Status, strings.TrimSpace(string(data)))
	}

	br := bufio.NewReader(resp.Body)
	lines, err := readPktLines(br)
	if err != nil {
		return "", err
	}

	var report strings.Builder
	ok := true
	for _, line := range lines {
		if line == "" {
			continue
		}
		report.WriteString(line)
		trimmed := strings.TrimRight(line, "\n")
		if strings.HasPrefix(trimmed, "unpack ") && trimmed != "unpack ok" {
			ok = false
		}
		if strings.HasPrefix(trimmed, "ng ") {
			ok = false
		}
	}
	if !ok {
		return report.String(), fmt.Errorf("remote rejected push: %s", strings.TrimSpace(report.String()))
	}
	return report.String(), nil
}
