// Package vault holds the password records and reads and writes the vault
// file:
//
//	MAGIC "CCPM" | VERSION u8 | HEADER_LEN u16 LE | HEADER(JSON) |
//	NONCE(12) | CIPHERTEXT+TAG
//
// The header (KDF params and salt) isn't encrypted but is passed as GCM AAD,
// so weakening the parameters or swapping the salt makes decryption fail. All
// records are encrypted together as one JSON blob.
package vault

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"ccpm/internal/crypto"
)

var (
	magic   = []byte("CCPM")
	version = byte(0x01)
)

// ErrCorrupt means the file isn't a well-formed vault. Tampering with the
// encrypted part shows up as crypto.ErrAuth instead.
var ErrCorrupt = errors.New("vault is corrupt or has been tampered with")

var (
	ErrNotFound = errors.New("no such record")
	ErrExists   = errors.New("a record with that name already exists")
)

type Record struct {
	Name      string    `json:"name"`
	Username  string    `json:"username"`
	Password  string    `json:"password"`
	Notes     string    `json:"notes,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Vault is the decrypted records plus the KDF parameters to re-encrypt them.
type Vault struct {
	params  crypto.KDFParams
	Records []Record `json:"records"`
}

// New creates an empty vault with fresh default KDF parameters.
func New() (*Vault, error) {
	p, err := crypto.DefaultParams()
	if err != nil {
		return nil, err
	}
	return &Vault{params: p}, nil
}

func (v *Vault) Params() crypto.KDFParams { return v.params }

// Add inserts r, or returns ErrExists if the name is taken.
func (v *Vault) Add(r Record) error {
	if r.Name == "" {
		return errors.New("record name must not be empty")
	}
	if _, _, found := v.find(r.Name); found {
		return ErrExists
	}
	now := time.Now().UTC()
	r.CreatedAt = now
	r.UpdatedAt = now
	v.Records = append(v.Records, r)
	return nil
}

func (v *Vault) Get(name string) (Record, error) {
	if r, _, found := v.find(name); found {
		return r, nil
	}
	return Record{}, ErrNotFound
}

// List returns the record names in sorted order.
func (v *Vault) List() []string {
	names := make([]string, 0, len(v.Records))
	for _, r := range v.Records {
		names = append(names, r.Name)
	}
	sort.Strings(names)
	return names
}

// Update lets apply edit the named record. Renaming isn't allowed, so any
// change to Name is undone.
func (v *Vault) Update(name string, apply func(*Record)) error {
	_, idx, found := v.find(name)
	if !found {
		return ErrNotFound
	}
	apply(&v.Records[idx])
	v.Records[idx].Name = name
	v.Records[idx].UpdatedAt = time.Now().UTC()
	return nil
}

func (v *Vault) Delete(name string) error {
	_, idx, found := v.find(name)
	if !found {
		return ErrNotFound
	}
	v.Records = append(v.Records[:idx], v.Records[idx+1:]...)
	return nil
}

func (v *Vault) find(name string) (Record, int, bool) {
	for i, r := range v.Records {
		if r.Name == name {
			return r, i, true
		}
	}
	return Record{}, -1, false
}

func buildHeader(p crypto.KDFParams) ([]byte, error) {
	hdr, err := crypto.MarshalParams(p)
	if err != nil {
		return nil, fmt.Errorf("marshaling header: %w", err)
	}
	if len(hdr) > 0xFFFF {
		return nil, errors.New("header too large")
	}
	return hdr, nil
}

// Encode returns the encrypted vault file contents.
func (v *Vault) Encode(key []byte) ([]byte, error) {
	hdr, err := buildHeader(v.params)
	if err != nil {
		return nil, err
	}

	plaintext, err := json.Marshal(struct {
		Records []Record `json:"records"`
	}{Records: v.Records})
	if err != nil {
		return nil, fmt.Errorf("marshaling records: %w", err)
	}

	nonce, ciphertext, err := crypto.Seal(key, plaintext, hdr)
	if err != nil {
		return nil, err
	}
	crypto.Zero(plaintext)

	var buf bytes.Buffer
	buf.Write(magic)
	buf.WriteByte(version)
	buf.Write(binary.LittleEndian.AppendUint16(nil, uint16(len(hdr))))
	buf.Write(hdr)
	buf.Write(nonce)
	buf.Write(ciphertext)
	return buf.Bytes(), nil
}

// ParseHeader returns the KDF parameters, which are needed to derive the key
// before the rest can be decrypted.
func ParseHeader(data []byte) (crypto.KDFParams, error) {
	_, p, _, _, err := parse(data)
	return p, err
}

func parse(data []byte) (hdr []byte, p crypto.KDFParams, nonce, ciphertext []byte, err error) {
	const minLen = 4 + 1 + 2
	if len(data) < minLen {
		return nil, p, nil, nil, ErrCorrupt
	}
	if !bytes.Equal(data[:4], magic) {
		return nil, p, nil, nil, ErrCorrupt
	}
	if data[4] != version {
		return nil, p, nil, nil, fmt.Errorf("%w: unsupported version %d", ErrCorrupt, data[4])
	}
	hlen := int(binary.LittleEndian.Uint16(data[5:7]))
	off := 7
	if len(data) < off+hlen+crypto.NonceLen {
		return nil, p, nil, nil, ErrCorrupt
	}
	hdr = data[off : off+hlen]
	off += hlen

	p, err = crypto.UnmarshalParams(hdr)
	if err != nil {
		return nil, p, nil, nil, fmt.Errorf("%w: %v", ErrCorrupt, err)
	}
	if err = p.Validate(); err != nil {
		return nil, p, nil, nil, fmt.Errorf("%w: %v", ErrCorrupt, err)
	}

	nonce = data[off : off+crypto.NonceLen]
	off += crypto.NonceLen
	ciphertext = data[off:]
	if len(ciphertext) == 0 {
		return nil, p, nil, nil, ErrCorrupt
	}
	return hdr, p, nonce, ciphertext, nil
}

// Decode decrypts a vault file. A wrong key or tampering gives crypto.ErrAuth;
// a malformed file gives ErrCorrupt.
func Decode(data, key []byte) (*Vault, error) {
	hdr, p, nonce, ciphertext, err := parse(data)
	if err != nil {
		return nil, err
	}
	plaintext, err := crypto.Open(key, nonce, ciphertext, hdr)
	if err != nil {
		return nil, err
	}
	defer crypto.Zero(plaintext)

	var payload struct {
		Records []Record `json:"records"`
	}
	if err := json.Unmarshal(plaintext, &payload); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrCorrupt, err)
	}
	return &Vault{params: p, Records: payload.Records}, nil
}

// Save writes the vault with mode 0600 via a temp file and rename, so a crash
// part way through can't destroy the existing vault.
func (v *Vault) Save(path string, key []byte) error {
	data, err := v.Encode(key)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".ccpm-*.tmp")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // fails harmlessly once renamed

	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return fmt.Errorf("setting permissions: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("writing vault: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("syncing vault: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("closing vault: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("finalizing vault: %w", err)
	}
	return nil
}

func Load(path string, key []byte) (*Vault, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return Decode(data, key)
}
