package pgputil

import (
	"bytes"
	"fmt"
	"io"

	"golang.org/x/crypto/openpgp/armor"
	"golang.org/x/crypto/openpgp/clearsign"
	"golang.org/x/crypto/openpgp/packet"
)

// DecodeKeys decodes public and private key packets from armored data.
func DecodeKeys(armoredData []byte) (pubKeys []*packet.PublicKey, privKeys []*packet.PrivateKey, err error) {
	block, err := armor.Decode(bytes.NewReader(armoredData))
	if err != nil {
		return nil, nil, fmt.Errorf("cannot decode armored data")
	}

	reader := packet.NewReader(block.Body)
	for {
		p, err := reader.Next()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, nil, err
		}
		if privKey, ok := p.(*packet.PrivateKey); ok {
			privKeys = append(privKeys, privKey)
		}
		if pubKey, ok := p.(*packet.PublicKey); ok {
			pubKeys = append(pubKeys, pubKey)
		}
	}
	return pubKeys, privKeys, nil
}

// DecodePubKey decodes a single public key packet from armored data. The
// data should contain exactly one public key packet and no private key packets.
func DecodePubKey(armoredData []byte) (*packet.PublicKey, error) {
	pubKeys, privKeys, err := DecodeKeys(armoredData)
	if err != nil {
		return nil, err
	}
	if len(privKeys) > 0 {
		return nil, fmt.Errorf("armored data contains private key")
	}
	if len(pubKeys) > 1 {
		return nil, fmt.Errorf("armored data contains more than one public key")
	}
	if len(pubKeys) == 0 {
		return nil, fmt.Errorf("armored data contains no public key")
	}
	return pubKeys[0], nil
}

// DecodeClearSigned decodes the first clearsigned message in the data and
// returns the signatures and the message body.
//
// The returned canonicalBody is canonicalized by converting line endings to
// <CR><LF> per the openPGP RCF: https://www.rfc-editor.org/rfc/rfc4880#section-5.2.4
func DecodeClearSigned(clearData []byte) (sigs []*packet.Signature, canonicalBody []byte, err error) {
	block, _ := clearsign.Decode(clearData)
	if block == nil {
		return nil, nil, fmt.Errorf("cannot decode clearsign text")
	}
	reader := packet.NewReader(block.ArmoredSignature.Body)
	for {
		p, err := reader.Next()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, nil, fmt.Errorf("cannot parse armored data: %w", err)
		}
		if sig, ok := p.(*packet.Signature); ok {
			sigs = append(sigs, sig)
		}
	}
	if len(sigs) == 0 {
		return nil, nil, fmt.Errorf("clearsigned data contains no signatures")
	}
	return sigs, block.Bytes, nil
}

// VerifySignature returns nil if sig is a valid signature from pubKey.
func VerifySignature(pubKey *packet.PublicKey, sig *packet.Signature, body []byte) error {
	hash := sig.Hash.New()
	_, err := io.Copy(hash, bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	return pubKey.VerifySignature(hash, sig)
}

// VerifyAnySignature returns nil if any signature in sigs is a valid signature
// mady by any of the public keys in pubKeys.
func VerifyAnySignature(pubKeys []*packet.PublicKey, sigs []*packet.Signature, body []byte) error {
	var err error
	for _, sig := range sigs {
		for _, key := range pubKeys {
			err = VerifySignature(key, sig, body)
			if err == nil {
				return nil
			}
		}
	}
	if len(sigs) == 1 && len(pubKeys) == 1 {
		return err
	}
	return fmt.Errorf("cannot verify any signatures")
}
