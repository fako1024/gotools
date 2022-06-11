package cryptoutils

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
)

type Bits = int

const (
	Bits2048 = 2048
	Bits4096 = 4096
	Bits8192 = 8192
)

// RSA denotes an RSA public / private key pair
type RSA struct {
	privKey *rsa.PrivateKey
}

// New creates a new elliptic curve key pair
func New(bits Bits) (obj *RSA, err error) {
	obj = &RSA{}
	obj.privKey, err = rsa.GenerateKey(rand.Reader, bits)

	return
}

// NewFromPEM reads a private key from a PEM block
func NewFromPEM(privPEM *pem.Block) (obj *RSA, err error) {
	if privPEM == nil {
		return nil, errors.New("invalid (nil) pem block provided")
	}

	obj = &RSA{}
	obj.privKey, err = x509.ParsePKCS1PrivateKey(privPEM.Bytes)

	return
}

// NewFromString reads a private key / RSA object from a base64 encoded string
func NewFromString(str string) (obj *RSA, err error) {
	var pemBytes []byte
	if pemBytes, err = base64.StdEncoding.DecodeString(str); err != nil {
		return
	}

	return NewFromPEM(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: pemBytes,
	})
}

// PubKey returns the public key
func (e *RSA) PubKey() *rsa.PublicKey {
	return &e.privKey.PublicKey
}

// PrivKey returns the private key
func (e *RSA) PrivKey() *rsa.PrivateKey {
	return e.privKey
}

// PubKeyPEM returns the public key as PEM block
func (e *RSA) PubKeyPEM() *pem.Block {
	return &pem.Block{
		Type:  "RSA PUBLIC KEY",
		Bytes: x509.MarshalPKCS1PublicKey(&e.privKey.PublicKey),
	}
}

// PrivKeyPEM returns the private key as PEM block
func (e *RSA) PrivKeyPEM() *pem.Block {
	return &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(e.privKey),
	}
}

// PrivKeyString returns the private key as base64 encoded PEM block
func (e *RSA) PrivKeyString() string {
	return base64.StdEncoding.EncodeToString(
		x509.MarshalPKCS1PrivateKey(e.privKey),
	)
}
