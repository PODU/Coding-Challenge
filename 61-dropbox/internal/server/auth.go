package server

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"sync"
)

// User is a server account. PasswordHash is the hex SHA-256 of the password;
// a real deployment should use bcrypt, but that would add a dependency. Token,
// if set, is a fixed bearer token for the user.
type User struct {
	Name         string `json:"name"`
	PasswordHash string `json:"password_hash"`
	Token        string `json:"token,omitempty"`
}

// Config is the server's user database.
type Config struct {
	Users []User `json:"users"`
}

// Auth maps bearer tokens to users and verifies logins.
type Auth struct {
	mu     sync.RWMutex
	users  map[string]User   // name -> user
	tokens map[string]string // token -> name
}

// NewAuth builds an Auth, registering any fixed tokens from cfg.
func NewAuth(cfg Config) *Auth {
	a := &Auth{users: map[string]User{}, tokens: map[string]string{}}
	for _, u := range cfg.Users {
		a.users[u.Name] = u
		if u.Token != "" {
			a.tokens[u.Token] = u.Name
		}
	}
	return a
}

func HashPassword(pw string) string {
	s := sha256.Sum256([]byte(pw))
	return hex.EncodeToString(s[:])
}

// Login verifies credentials and returns a bearer token on success.
func (a *Auth) Login(name, pw string) (string, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	u, ok := a.users[name]
	if !ok || u.PasswordHash != HashPassword(pw) {
		return "", false
	}
	tok := u.Token
	if tok == "" {
		tok = randomToken()
	}
	a.tokens[tok] = name
	return tok, true
}

// User resolves a bearer token to a user name.
func (a *Auth) User(token string) (string, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	n, ok := a.tokens[token]
	return n, ok
}

func randomToken() string {
	b := make([]byte, 24)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
