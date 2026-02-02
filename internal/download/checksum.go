package download

import (
	"bufio"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ChecksumType represents the type of checksum
type ChecksumType string

const (
	ChecksumTypeSHA256 ChecksumType = "sha256"
	ChecksumTypeSHA512 ChecksumType = "sha512"
	ChecksumTypeMD5    ChecksumType = "md5"
	ChecksumTypeNone   ChecksumType = ""
)

// ChecksumInfo contains checksum verification info
type ChecksumInfo struct {
	Type     ChecksumType
	Expected string
}

// VerifyChecksum verifies a file against the expected checksum
func VerifyChecksum(filePath string, expected string, checksumType ChecksumType) error {
	// Read file
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	// Calculate checksum
	var actual string
	switch checksumType {
	case ChecksumTypeSHA256:
		hash := sha256.Sum256(data)
		actual = hex.EncodeToString(hash[:])
	case ChecksumTypeSHA512:
		hash := sha512.Sum512(data)
		actual = hex.EncodeToString(hash[:])
	default:
		return fmt.Errorf("unsupported checksum type: %s", checksumType)
	}

	// Compare (case-insensitive)
	expected = strings.ToLower(expected)
	actual = strings.ToLower(actual)

	if actual != expected {
		return fmt.Errorf("checksum mismatch: expected %s, got %s", expected, actual)
	}

	return nil
}

// ParseChecksumFile parses a checksum file and returns the checksum for the given filename
// Supports common formats:
// - SHA256: "abc123...  filename" or "abc123... *filename"
// - SHA512: "abc123...  filename" or "abc123... *filename"
// - Simple: just the hash value
func ParseChecksumFile(checksumFile string, filename string) (*ChecksumInfo, error) {
	file, err := os.Open(checksumFile)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Try to parse as "hash  filename" format
		parts := strings.Fields(line)
		if len(parts) >= 2 {
			hash := parts[0]
			// Check if the filename matches (with or without path)
			filenameInLine := parts[len(parts)-1]

			// Remove leading * if present (indicates binary file in GPG-signed checksums)
			filenameInLine = strings.TrimPrefix(filenameInLine, "*")

			// Try matching just the basename
			if filepath.Base(filenameInLine) == filepath.Base(filename) ||
				filenameInLine == filename ||
				strings.HasSuffix(filenameInLine, "/"+filepath.Base(filename)) {

				// Determine checksum type based on hash length
				var checksumType ChecksumType
				switch len(hash) {
				case 64:
					checksumType = ChecksumTypeSHA256
				case 128:
					checksumType = ChecksumTypeSHA512
				case 32:
					checksumType = ChecksumTypeMD5
				default:
					checksumType = ChecksumTypeSHA256 // Default to SHA256
				}

				return &ChecksumInfo{
					Type:     checksumType,
					Expected: hash,
				}, nil
			}
		}

		// Try to parse as just a hash (first word in line)
		if len(parts) >= 1 {
			hash := parts[0]
			// Check if it looks like a valid hash
			if isHexString(hash) {
				var checksumType ChecksumType
				switch len(hash) {
				case 64:
					checksumType = ChecksumTypeSHA256
				case 128:
					checksumType = ChecksumTypeSHA512
				default:
					continue // Skip unknown hash lengths
				}

				// This might be a simple hash file with just the hash
				return &ChecksumInfo{
					Type:     checksumType,
					Expected: hash,
				}, nil
			}
		}
	}

	return nil, fmt.Errorf("checksum for %s not found in %s", filename, checksumFile)
}

// FindChecksumFile looks for a checksum file near the asset
func FindChecksumFile(assetName string) string {
	// Common checksum file names
	baseName := strings.TrimSuffix(assetName, filepath.Ext(assetName))

	checksumFiles := []string{
		assetName + ".sha256",
		assetName + ".sha512",
		assetName + ".sha256sum",
		assetName + ".sha512sum",
		assetName + ".checksum",
		assetName + ".md5",
		"checksums.txt",
		"checksums.txt.sha256",
		"checksums.txt.sha512",
		"checksum.txt",
		"sha256.txt",
		"sha512.txt",
		baseName + ".sha256",
		baseName + ".sha512",
	}

	for _, f := range checksumFiles {
		if _, err := os.Stat(f); err == nil {
			return f
		}
	}

	return ""
}

// isHexString checks if a string is a valid hexadecimal string
func isHexString(s string) bool {
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return len(s) > 0
}

// ComputeSHA256 computes the SHA256 checksum of a file
func ComputeSHA256(filePath string) (string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}

	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:]), nil
}

// ComputeSHA512 computes the SHA512 checksum of a file
func ComputeSHA512(filePath string) (string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}

	hash := sha512.Sum512(data)
	return hex.EncodeToString(hash[:]), nil
}
