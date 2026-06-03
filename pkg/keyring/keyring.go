// Package keyring stores and retrieves Debi API secret keys using the host
// operating system's secure credential store (macOS Keychain, Windows
// Credential Manager, Linux Secret Service), with an encrypted file backend
// as a fallback for headless environments.
package keyring

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	keyringlib "github.com/99designs/keyring"
	"github.com/tucuota/debi-cli/pkg/config"
)

const serviceName = "debi-cli"

// ErrNotFound is returned when no key is stored for a profile.
var ErrNotFound = errors.New("no API key found for profile")

func open() (keyringlib.Keyring, error) {
	dir, err := config.Dir()
	if err != nil {
		return nil, err
	}
	fileDir := filepath.Join(dir, "keyring")

	return keyringlib.Open(keyringlib.Config{
		ServiceName: serviceName,

		// macOS
		KeychainName:                   "login",
		KeychainTrustApplication:       true,
		KeychainSynchronizable:         false,
		KeychainAccessibleWhenUnlocked: true,

		// Linux Secret Service
		LibSecretCollectionName: "login",

		// Windows
		WinCredPrefix: serviceName,

		// Encrypted file fallback (headless / no secret service available).
		FileDir:          fileDir,
		FilePasswordFunc: filePassword,
	})
}

// filePassword supplies the passphrase used to encrypt the file backend. It
// reads DEBI_KEYRING_PASSWORD when set so the file backend works without a
// TTY; otherwise it falls back to an interactive prompt.
func filePassword(prompt string) (string, error) {
	if pw := os.Getenv("DEBI_KEYRING_PASSWORD"); pw != "" {
		return pw, nil
	}
	return keyringlib.TerminalPrompt(prompt)
}

// Set stores the secret key for the given profile.
func Set(profile, key string) error {
	ring, err := open()
	if err != nil {
		return fmt.Errorf("opening keychain: %w", err)
	}
	return ring.Set(keyringlib.Item{
		Key:         profile,
		Data:        []byte(key),
		Label:       fmt.Sprintf("Debi API key (%s)", profile),
		Description: "debi-cli secret API key",
	})
}

// Get returns the secret key stored for the given profile.
func Get(profile string) (string, error) {
	ring, err := open()
	if err != nil {
		return "", fmt.Errorf("opening keychain: %w", err)
	}
	item, err := ring.Get(profile)
	if err != nil {
		if errors.Is(err, keyringlib.ErrKeyNotFound) {
			return "", ErrNotFound
		}
		return "", err
	}
	return string(item.Data), nil
}

// Delete removes the secret key stored for the given profile.
func Delete(profile string) error {
	ring, err := open()
	if err != nil {
		return fmt.Errorf("opening keychain: %w", err)
	}
	if err := ring.Remove(profile); err != nil {
		if errors.Is(err, keyringlib.ErrKeyNotFound) {
			return ErrNotFound
		}
		return err
	}
	return nil
}
