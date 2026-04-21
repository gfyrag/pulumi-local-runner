package git

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"os"

	"golang.org/x/crypto/pbkdf2"
)

const pbkdf2Iterations = 1_000_000

const defaultPassphrase = "plr"

func getPassphrase() string {
	if p := os.Getenv("PULUMI_CONFIG_PASSPHRASE"); p != "" {
		return p
	}
	return defaultPassphrase
}

// EnsurePassphrase sets PULUMI_CONFIG_PASSPHRASE to match what plr uses,
// so Pulumi can decrypt secrets encrypted by plr.
func EnsurePassphrase() {
	os.Setenv("PULUMI_CONFIG_PASSPHRASE", getPassphrase())
}

// deriveKey extracts the PBKDF2 salt from an encryptionsalt string and derives the AES key.
// Format: v1:<base64salt>:v1:<base64nonce>:<base64ciphertext>
func deriveKey(encryptionSalt string) ([]byte, error) {
	passphrase := getPassphrase()

	if len(encryptionSalt) < 4 || encryptionSalt[:3] != "v1:" {
		return nil, fmt.Errorf("invalid encryption salt format")
	}
	rest := encryptionSalt[3:]
	idx := 0
	for i, c := range rest {
		if c == ':' {
			idx = i
			break
		}
	}
	if idx == 0 {
		return nil, fmt.Errorf("invalid encryption salt format")
	}

	salt, err := base64.StdEncoding.DecodeString(rest[:idx])
	if err != nil {
		return nil, fmt.Errorf("decoding salt: %w", err)
	}

	return pbkdf2.Key([]byte(passphrase), salt, pbkdf2Iterations, 32, sha256.New), nil
}

// EncryptSecret encrypts a plaintext value using the global encryption salt.
// Returns the Pulumi-compatible encrypted format: v1:<base64(nonce)>:<base64(ciphertext)>
func EncryptSecret(encryptionSalt, plaintext string) (string, error) {
	key, err := deriveKey(encryptionSalt)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}

	ciphertext := gcm.Seal(nil, nonce, []byte(plaintext), nil)

	return fmt.Sprintf("v1:%s:%s",
		base64.StdEncoding.EncodeToString(nonce),
		base64.StdEncoding.EncodeToString(ciphertext),
	), nil
}

// DecryptSecret decrypts a Pulumi-encrypted secret value.
// Format: v1:<base64(nonce)>:<base64(ciphertext)>
func DecryptSecret(encryptionSalt, encrypted string) (string, error) {
	if len(encrypted) < 4 || encrypted[:3] != "v1:" {
		return "", fmt.Errorf("invalid secret format")
	}

	key, err := deriveKey(encryptionSalt)
	if err != nil {
		return "", err
	}

	// Split "v1:<nonce>:<ciphertext>"
	rest := encrypted[3:]
	idx := 0
	for i, c := range rest {
		if c == ':' {
			idx = i
			break
		}
	}
	if idx == 0 {
		return "", fmt.Errorf("invalid secret format")
	}

	nonce, err := base64.StdEncoding.DecodeString(rest[:idx])
	if err != nil {
		return "", fmt.Errorf("decoding nonce: %w", err)
	}

	ciphertext, err := base64.StdEncoding.DecodeString(rest[idx+1:])
	if err != nil {
		return "", fmt.Errorf("decoding ciphertext: %w", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("decryption failed (wrong passphrase?): %w", err)
	}

	return string(plaintext), nil
}

// GenerateEncryptionSalt creates a Pulumi-compatible passphrase encryption salt.
// Format: v1:<base64salt>:v1:<base64nonce>:<base64ciphertext>
// The ciphertext is an encryption of "pulumi" for passphrase verification.
func GenerateEncryptionSalt() (string, error) {
	passphrase := getPassphrase()

	salt := make([]byte, 8)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generating random salt: %w", err)
	}

	key := pbkdf2.Key([]byte(passphrase), salt, pbkdf2Iterations, 32, sha256.New)

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("creating cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("creating GCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("generating nonce: %w", err)
	}

	ciphertext := gcm.Seal(nil, nonce, []byte("pulumi"), nil)

	return fmt.Sprintf("v1:%s:v1:%s:%s",
		base64.StdEncoding.EncodeToString(salt),
		base64.StdEncoding.EncodeToString(nonce),
		base64.StdEncoding.EncodeToString(ciphertext),
	), nil
}
