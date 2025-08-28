package cryptoutils

import (
	"crypto/sha512"
	"encoding/asn1"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Using very low amount of entropy to avoid timing out
const testBits = 32

func TestPrivatePublicKeyConsistency(t *testing.T) {
	r, err := New(testBits)
	assert.Nil(t, err)

	// Get the private key
	privKey := r.PrivKey()
	assert.Nil(t, err)
	assert.Equalf(t, privKey.Size(), testBits/8, "private key size should be %d bytes", testBits/8)

	// Get the public key
	pubKey := r.PubKey()
	assert.Nil(t, err)
	assert.Equalf(t, pubKey.Size(), testBits/8, "private key size should be %d bytes", testBits/8)
	assert.True(t, pubKey.Equal(&privKey.PublicKey), "extracted and computed public keys should be equal")
}

func TestInvalid(t *testing.T) {
	_, err := New(0)
	if assert.Error(t, err) {
		assert.Equal(t, errors.New("rsa: key too small"), err)
	}
	_, err = NewFromPEM(nil)
	if assert.Error(t, err) {
		assert.Equal(t, errors.New("invalid (nil) pem block provided"), err)
	}
	_, err = NewFromPEM(&pem.Block{})
	if assert.Error(t, err) {
		assert.Equal(t, asn1.SyntaxError{Msg: "sequence truncated"}, err)
	}
	_, err = NewFromString("")
	if assert.Error(t, err) {
		assert.Equal(t, asn1.SyntaxError{Msg: "sequence truncated"}, err)
	}
	_, err = NewFromString("jkhgxdfkjhsgd")
	if assert.Error(t, err) {
		assert.Equal(t, base64.CorruptInputError(12), err)
	}
	_, err = NewFromString("bm9wZQ==")
	if assert.Error(t, err) {
		assert.Equal(t, asn1.StructuralError{Msg: "tags don't match (16 vs {class:1 tag:14 length:111 isCompound:true}) {optional:false explicit:false application:false private:false defaultValue:<nil> tag:<nil> stringType:0 timeType:0 set:false omitEmpty:false} pkcs1PrivateKey @2"}, err)
	}
}

func TestPEMConversion(t *testing.T) {
	r1, err := New(testBits)
	assert.Nil(t, err)

	privKeyPEM := r1.PrivKeyPEM()
	pubKeyPEM1 := r1.PubKeyPEM()

	r2, err := NewFromPEM(privKeyPEM)
	assert.Nil(t, err)
	assert.Equal(t, r1, r2, "initial and re-read instances should be equal on reference-level")
	assert.Equal(t, *r1, *r2, "initial and re-read instances should be equal on value-level")

	pubKeyPEM2 := r2.PubKeyPEM()
	assert.Equal(t, pubKeyPEM1, pubKeyPEM2, "initial and re-read public keys should be equal")
}

func TestStringConversion(t *testing.T) {
	r1, err := New(testBits)
	assert.Nil(t, err)

	privKeyString := r1.PrivKeyString()

	r2, err := NewFromString(privKeyString)
	assert.Nil(t, err)
	assert.Equal(t, r1, r2, "initial and re-read instances should be equal on reference-level")
	assert.Equal(t, *r1, *r2, "initial and re-read instances should be equal on value-level")
}

func TestEncryption(t *testing.T) {

	r, err := New(1024)
	assert.Nil(t, err)

	clearText := []byte("This is a test message")
	cipherText, err := r.Encrypt(clearText, nil)
	assert.Nil(t, err)

	// Try encryption with hash exceeding maximum encryption size (limited by 1024 bits)
	_, err = r.Encrypt(clearText, sha512.New())
	if assert.Error(t, err) {
		assert.Equal(t, errors.New("crypto/rsa: message too long for RSA key size"), err)
	}

	clearText2, err := r.Decrypt(cipherText, nil)
	assert.Nil(t, err)
	assert.Equal(t, string(clearText), string(clearText2), "initial cleartext and cleartext after encryption round-trip should be equal")
}

func TestEncryptionCustomHash(t *testing.T) {

	r, err := New(Bits2048)
	assert.Nil(t, err)

	clearText := []byte("This is a test message")
	cipherText, err := r.Encrypt(clearText, sha512.New())
	assert.Nil(t, err)

	clearText2, err := r.Decrypt(cipherText, sha512.New())
	assert.Nil(t, err)
	assert.Equal(t, string(clearText), string(clearText2), "initial cleartext and cleartext after encryption round-trip should be equal")
}
