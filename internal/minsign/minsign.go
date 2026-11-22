// Package minsign signs and verifies evidence with a minisign Ed25519 signature.
package minsign

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"strings"
)

var randRead = rand.Read

// Key is an Ed25519 key pair. The id is the first eight bytes of the public key.
type Key struct {
	ID      [8]byte
	Public  ed25519.PublicKey
	Private ed25519.PrivateKey
}

// Generate creates a new key pair.
func Generate() (Key, error) {
	pub, priv, err := generate()
	if err != nil {
		return Key{}, err
	}
	var id [8]byte
	copy(id[:], pub[:8])
	return Key{ID: id, Public: pub, Private: priv}, nil
}

func generate() (ed25519.PublicKey, ed25519.PrivateKey, error) {
	seed := make([]byte, ed25519.SeedSize)
	if _, err := randRead(seed); err != nil {
		return nil, nil, err
	}
	priv := ed25519.NewKeyFromSeed(seed)
	return priv.Public().(ed25519.PublicKey), priv, nil
}

// PublicText is a minisign public key file.
func (k Key) PublicText() string {
	blob := append([]byte{'E', 'd'}, k.ID[:]...)
	blob = append(blob, k.Public...)
	return "untrusted comment: minisign public key\n" + base64.StdEncoding.EncodeToString(blob) + "\n"
}

// SecretText is the secret key file used by proofsieve keygen.
func (k Key) SecretText() string {
	blob := append([]byte{'E', 'd'}, k.ID[:]...)
	blob = append(blob, k.Private.Seed()...)
	blob = append(blob, k.Public...)
	return "untrusted comment: proofsieve minisign secret key\n" + base64.StdEncoding.EncodeToString(blob) + "\n"
}

// ParseSecret reads a secret key file from SecretText.
func ParseSecret(text string) (Key, error) {
	raw, err := decodeBody(text)
	if err != nil {
		return Key{}, err
	}
	if len(raw) != 2+8+ed25519.SeedSize+ed25519.PublicKeySize || string(raw[:2]) != "Ed" {
		return Key{}, fmt.Errorf("secret key is not a proofsieve minisign key")
	}
	var id [8]byte
	copy(id[:], raw[2:10])
	priv := ed25519.NewKeyFromSeed(raw[10 : 10+ed25519.SeedSize])
	pub := ed25519.PublicKey(raw[10+ed25519.SeedSize:])
	if !pub.Equal(priv.Public()) {
		return Key{}, fmt.Errorf("secret key does not match its public key")
	}
	return Key{ID: id, Public: pub, Private: priv}, nil
}

// Sign writes a minisign signature file for message.
func Sign(k Key, message []byte, trusted string) (string, error) {
	if strings.TrimSpace(trusted) == "" {
		return "", fmt.Errorf("trusted comment is required")
	}
	sig := ed25519.Sign(k.Private, message)
	blob := append([]byte{'E', 'd'}, k.ID[:]...)
	blob = append(blob, sig...)
	global := ed25519.Sign(k.Private, append(append([]byte{}, sig...), []byte(trusted)...))
	var b strings.Builder
	b.WriteString("untrusted comment: signature from minisign secret key\n")
	b.WriteString(base64.StdEncoding.EncodeToString(blob))
	b.WriteString("\ntrusted comment: ")
	b.WriteString(trusted)
	b.WriteString("\n")
	b.WriteString(base64.StdEncoding.EncodeToString(global))
	b.WriteString("\n")
	return b.String(), nil
}

// Verify checks a minisign signature file against a public key file and message.
func Verify(publicText, sigText string, message []byte) error {
	pub, id, err := parsePublic(publicText)
	if err != nil {
		return err
	}
	lines := nonEmpty(sigText)
	if len(lines) < 4 {
		return fmt.Errorf("signature file is incomplete")
	}
	raw, err := base64.StdEncoding.DecodeString(lines[1])
	if err != nil {
		return fmt.Errorf("signature: %w", err)
	}
	if len(raw) != 2+8+ed25519.SignatureSize || string(raw[:2]) != "Ed" {
		return fmt.Errorf("signature algorithm is not Ed")
	}
	var sigID [8]byte
	copy(sigID[:], raw[2:10])
	if sigID != id {
		return fmt.Errorf("signature key id does not match the public key")
	}
	sig := raw[10:]
	if !ed25519.Verify(pub, message, sig) {
		return fmt.Errorf("signature does not match the evidence")
	}
	trusted, ok := strings.CutPrefix(lines[2], "trusted comment: ")
	if !ok {
		return fmt.Errorf("signature is missing a trusted comment")
	}
	global, err := base64.StdEncoding.DecodeString(lines[3])
	if err != nil {
		return fmt.Errorf("global signature: %w", err)
	}
	if !ed25519.Verify(pub, append(append([]byte{}, sig...), []byte(trusted)...), global) {
		return fmt.Errorf("trusted comment was altered")
	}
	return nil
}

func parsePublic(text string) (ed25519.PublicKey, [8]byte, error) {
	raw, err := decodeBody(text)
	if err != nil {
		return nil, [8]byte{}, err
	}
	if len(raw) != 2+8+ed25519.PublicKeySize || string(raw[:2]) != "Ed" {
		return nil, [8]byte{}, fmt.Errorf("public key is not a minisign key")
	}
	var id [8]byte
	copy(id[:], raw[2:10])
	return ed25519.PublicKey(raw[10:]), id, nil
}

func decodeBody(text string) ([]byte, error) {
	for _, line := range nonEmpty(text) {
		if strings.HasPrefix(line, "untrusted comment:") {
			continue
		}
		raw, err := base64.StdEncoding.DecodeString(line)
		if err != nil {
			return nil, err
		}
		return raw, nil
	}
	return nil, fmt.Errorf("key file has no key data")
}

func nonEmpty(text string) []string {
	var out []string
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}
