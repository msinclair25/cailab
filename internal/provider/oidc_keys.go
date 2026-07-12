package provider

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"sort"
	"time"
)

type oidcSigningKey struct {
	PrivateKey *rsa.PrivateKey
	PublicKey  *rsa.PublicKey
	KeyID      string
	RetireAt   time.Time
}

type oidcPersistentState struct {
	Keys []oidcPersistentKey `json:"keys"`
}

type oidcPersistentKey struct {
	PrivateKeyPEM string `json:"privateKeyPem,omitempty"`
	PublicKeyPEM  string `json:"publicKeyPem,omitempty"`
	KeyID         string `json:"kid"`
	RetireAt      int64  `json:"retireAt,omitempty"`
}

func (f *oidcFacade) loadOrInitializeKeys() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	data, err := os.ReadFile(f.statePath)
	if err == nil {
		var state oidcPersistentState
		if err := json.Unmarshal(data, &state); err != nil {
			return fmt.Errorf("decode OIDC signing state: %w", err)
		}
		for _, persisted := range state.Keys {
			key, err := decodeOIDCSigningKey(persisted)
			if err != nil {
				return err
			}
			f.keys = append(f.keys, key)
		}
		f.pruneLocked(f.now())
		if len(f.keys) == 0 {
			return errors.New("OIDC signing state contains no usable key")
		}
		return nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("read OIDC signing state: %w", err)
	}
	key, err := f.generateKey()
	if err != nil {
		return err
	}
	jwk, err := publicJWK(&key.PublicKey)
	if err != nil {
		return err
	}
	f.keys = []oidcSigningKey{{PrivateKey: key, PublicKey: &key.PublicKey, KeyID: jwk.KeyID}}
	return f.persistLocked()
}

func decodeOIDCSigningKey(persisted oidcPersistentKey) (oidcSigningKey, error) {
	if (persisted.PrivateKeyPEM == "") == (persisted.PublicKeyPEM == "") {
		return oidcSigningKey{}, errors.New("OIDC signing key state must contain exactly one private or public key")
	}
	var privateKey *rsa.PrivateKey
	var publicKey *rsa.PublicKey
	var err error
	if persisted.PrivateKeyPEM != "" {
		block, _ := pem.Decode([]byte(persisted.PrivateKeyPEM))
		if block == nil || block.Type != "RSA PRIVATE KEY" {
			return oidcSigningKey{}, errors.New("decode OIDC RSA private key PEM")
		}
		privateKey, err = x509.ParsePKCS1PrivateKey(block.Bytes)
		if err != nil {
			return oidcSigningKey{}, fmt.Errorf("parse OIDC RSA private key: %w", err)
		}
		publicKey = &privateKey.PublicKey
	} else {
		block, _ := pem.Decode([]byte(persisted.PublicKeyPEM))
		if block == nil || block.Type != "PUBLIC KEY" {
			return oidcSigningKey{}, errors.New("decode OIDC RSA public key PEM")
		}
		parsed, err := x509.ParsePKIXPublicKey(block.Bytes)
		if err != nil {
			return oidcSigningKey{}, fmt.Errorf("parse OIDC RSA public key: %w", err)
		}
		var ok bool
		publicKey, ok = parsed.(*rsa.PublicKey)
		if !ok {
			return oidcSigningKey{}, errors.New("OIDC public key is not RSA")
		}
	}
	jwk, err := publicJWK(publicKey)
	if err != nil || jwk.KeyID != persisted.KeyID {
		return oidcSigningKey{}, errors.New("OIDC persisted key ID does not match key material")
	}
	retireAt := time.Time{}
	if persisted.RetireAt > 0 {
		retireAt = time.Unix(persisted.RetireAt, 0)
	}
	return oidcSigningKey{PrivateKey: privateKey, PublicKey: publicKey, KeyID: persisted.KeyID, RetireAt: retireAt}, nil
}

func (f *oidcFacade) persistLocked() error {
	state := oidcPersistentState{Keys: make([]oidcPersistentKey, 0, len(f.keys))}
	for _, key := range f.keys {
		persisted := oidcPersistentKey{KeyID: key.KeyID}
		if key.PrivateKey != nil {
			persisted.PrivateKeyPEM = string(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key.PrivateKey)}))
		} else {
			publicData, err := x509.MarshalPKIXPublicKey(key.PublicKey)
			if err != nil {
				return fmt.Errorf("encode retired OIDC public key: %w", err)
			}
			persisted.PublicKeyPEM = string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: publicData}))
		}
		if !key.RetireAt.IsZero() {
			persisted.RetireAt = key.RetireAt.Unix()
		}
		state.Keys = append(state.Keys, persisted)
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("encode OIDC signing state: %w", err)
	}
	if err := os.WriteFile(f.statePath, append(data, '\n'), 0o600); err != nil {
		return fmt.Errorf("persist OIDC signing state: %w", err)
	}
	return nil
}

func (f *oidcFacade) pruneLocked(now time.Time) {
	kept := f.keys[:0]
	for _, key := range f.keys {
		if key.RetireAt.IsZero() || now.Before(key.RetireAt) {
			kept = append(kept, key)
		}
	}
	f.keys = kept
}

func (f *oidcFacade) jwksLocked(now time.Time) OIDCJWKSet {
	set := OIDCJWKSet{Keys: make([]OIDCJWK, 0, len(f.keys))}
	for _, key := range f.keys {
		if !key.RetireAt.IsZero() && !now.Before(key.RetireAt) {
			continue
		}
		jwk, err := publicJWK(key.PublicKey)
		if err == nil {
			set.Keys = append(set.Keys, jwk)
		}
	}
	sort.Slice(set.Keys, func(i, j int) bool { return set.Keys[i].KeyID < set.Keys[j].KeyID })
	return set
}
