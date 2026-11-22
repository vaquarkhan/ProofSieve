package minsign

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"errors"
	"strings"
	"testing"
)

func TestRoundTrip(t *testing.T) {
	k, err := Generate()
	if err != nil {
		t.Fatal(err)
	}
	again, err := ParseSecret(k.SecretText())
	if err != nil || !bytes.Equal(again.Public, k.Public) {
		t.Fatal(err)
	}
	msg := []byte("{\"sha\":\"abc\"}\n")
	sig, err := Sign(k, msg, "timestamp:1\tfile:evidence.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := Verify(k.PublicText(), sig, msg); err != nil {
		t.Fatal(err)
	}
	if err := Verify(k.PublicText(), sig, []byte("nope")); err == nil {
		t.Fatal("tampered evidence")
	}
	other, err := Generate()
	if err != nil {
		t.Fatal(err)
	}
	if err := Verify(other.PublicText(), sig, msg); err == nil {
		t.Fatal("wrong key")
	}
	lines := strings.Split(strings.TrimSpace(sig), "\n")
	lines[2] = "trusted comment: changed"
	if err := Verify(k.PublicText(), strings.Join(lines, "\n"), msg); err == nil {
		t.Fatal("tampered comment")
	}
	if _, err := Sign(k, msg, "  "); err == nil {
		t.Fatal("empty comment")
	}
}

func TestParseErrors(t *testing.T) {
	if _, err := ParseSecret("untrusted comment: x\n"); err == nil {
		t.Fatal("empty")
	}
	if _, err := ParseSecret("not-base64!!!"); err == nil {
		t.Fatal("b64")
	}
	short := "untrusted comment: x\n" + "RWU=" + "\n"
	if _, err := ParseSecret(short); err == nil {
		t.Fatal("short")
	}
	if err := Verify("!!!!", "a\nb\nc\nd\n", []byte("m")); err == nil {
		t.Fatal("public")
	}
	if err := Verify("untrusted comment: x\nRWU=\n", "a\nb\nc\nd\n", []byte("m")); err == nil {
		t.Fatal("short public")
	}
	k, err := Generate()
	if err != nil {
		t.Fatal(err)
	}
	if err := Verify(k.PublicText(), "only-one-line\n", []byte("m")); err == nil {
		t.Fatal("short sig")
	}
	blob := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{'Z'}, 2+8+ed25519.SignatureSize))
	if err := Verify(k.PublicText(), "untrusted comment: x\n"+blob+"\ntrusted comment: ok\nAAAA\n", []byte("m")); err == nil {
		t.Fatal("alg")
	}
	sig, err := Sign(k, []byte("m"), "ok")
	if err != nil {
		t.Fatal(err)
	}
	bad := strings.Replace(sig, "\ntrusted comment:", "\ncomment:", 1)
	if err := Verify(k.PublicText(), bad, []byte("m")); err == nil {
		t.Fatal("trusted prefix")
	}
	broken := strings.Replace(sig, linesOf(sig)[1], "!!!!", 1)
	if err := Verify(k.PublicText(), broken, []byte("m")); err == nil {
		t.Fatal("sig b64")
	}
	glob := linesOf(sig)
	glob[3] = "!!!!"
	if err := Verify(k.PublicText(), strings.Join(glob, "\n"), []byte("m")); err == nil {
		t.Fatal("global b64")
	}
	raw, err := decodeBody(k.SecretText())
	if err != nil {
		t.Fatal(err)
	}
	raw[len(raw)-1] ^= 0xff
	flipped := "untrusted comment: x\n" + base64.StdEncoding.EncodeToString(raw) + "\n"
	if _, err := ParseSecret(flipped); err == nil {
		t.Fatal("mismatch")
	}
	orig := randRead
	randRead = func([]byte) (int, error) { return 0, errors.New("rand") }
	t.Cleanup(func() { randRead = orig })
	if _, err := Generate(); err == nil {
		t.Fatal("rand")
	}
}

func linesOf(text string) []string {
	return strings.Split(strings.TrimSpace(text), "\n")
}
