package download

import (
	"os"
	"path/filepath"
	"testing"
)

func TestVerifyChecksum(t *testing.T) {
	// Create a test file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	testContent := []byte("hello world")
	if err := os.WriteFile(testFile, testContent, 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// SHA256 of "hello world"
	sha256Hash := "b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9"
	sha512Hash := "309ecc489c12d6eb4cc40f50c902f2b4d0ed77ee511a7c7a9bcd3ca86d4cd86f989dd35bc5ff499670da34255b45b0cfd830e81f605dcf7dc5542e93ae9cd76f"

	tests := []struct {
		name         string
		expected     string
		checksumType ChecksumType
		wantErr      bool
	}{
		{"valid sha256", sha256Hash, ChecksumTypeSHA256, false},
		{"valid sha512", sha512Hash, ChecksumTypeSHA512, false},
		{"invalid sha256", "invalid", ChecksumTypeSHA256, true},
		{"wrong sha256", "b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde8", ChecksumTypeSHA256, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := VerifyChecksum(testFile, tt.expected, tt.checksumType)
			if (err != nil) != tt.wantErr {
				t.Errorf("VerifyChecksum() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestIsHexString(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9", true},
		{"B94D27B9934D3E08A52E52D7DA7DABFAC484EFE37A5380EE9088F7ACE2EF CDE9", false}, // contains space
		{"b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde", true},  // odd length
		{"", false},
		{"gg", false}, // invalid hex
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := isHexString(tt.input); got != tt.want {
				t.Errorf("isHexString(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseChecksumFile(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a checksum file
	checksumContent := `b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9  test.txt
309ecc489c12d6eb4cc40f50c902f2b4d0ed77ee511a7c7a9bcd3ca86d4cd86f989dd35bc5ff499670da34255b45b0cfd830e81f605dcf7dc5542e93ae9cd76f  other.txt`
	checksumFile := filepath.Join(tmpDir, "checksums.txt")
	if err := os.WriteFile(checksumFile, []byte(checksumContent), 0644); err != nil {
		t.Fatalf("failed to create checksum file: %v", err)
	}

	// Test parsing
	info, err := ParseChecksumFile(checksumFile, "test.txt")
	if err != nil {
		t.Fatalf("failed to parse checksum: %v", err)
	}

	if info.Type != ChecksumTypeSHA256 {
		t.Errorf("expected SHA256, got %s", info.Type)
	}

	expectedHash := "b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9"
	if info.Expected != expectedHash {
		t.Errorf("expected hash %s, got %s", expectedHash, info.Expected)
	}
}

func TestParseChecksumFileWithAsterisk(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a checksum file with asterisk prefix (GPG-signed format)
	checksumContent := `b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9 *test.txt`
	checksumFile := filepath.Join(tmpDir, "checksums.txt")
	if err := os.WriteFile(checksumFile, []byte(checksumContent), 0644); err != nil {
		t.Fatalf("failed to create checksum file: %v", err)
	}

	// Test parsing
	info, err := ParseChecksumFile(checksumFile, "test.txt")
	if err != nil {
		t.Fatalf("failed to parse checksum: %v", err)
	}

	if info.Type != ChecksumTypeSHA256 {
		t.Errorf("expected SHA256, got %s", info.Type)
	}
}

func TestParseChecksumFileNotFound(t *testing.T) {
	tmpDir := t.TempDir()

	checksumFile := filepath.Join(tmpDir, "nonexistent.txt")
	_, err := ParseChecksumFile(checksumFile, "test.txt")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestFindChecksumFile(t *testing.T) {
	tmpDir := t.TempDir()

	// Create some checksum files
	testFile := filepath.Join(tmpDir, "test.tar.gz")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Create a sha256 file
	sha256File := filepath.Join(tmpDir, "test.tar.gz.sha256")
	if err := os.WriteFile(sha256File, []byte("hash"), 0644); err != nil {
		t.Fatalf("failed to create sha256 file: %v", err)
	}

	// Test finding
	found := FindChecksumFile(testFile)
	if found != sha256File {
		t.Errorf("expected %s, got %s", sha256File, found)
	}
}

func TestComputeSHA256(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	testContent := []byte("hello world")
	if err := os.WriteFile(testFile, testContent, 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	hash, err := ComputeSHA256(testFile)
	if err != nil {
		t.Fatalf("failed to compute SHA256: %v", err)
	}

	expected := "b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9"
	if hash != expected {
		t.Errorf("expected %s, got %s", expected, hash)
	}
}

func TestComputeSHA512(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	testContent := []byte("hello world")
	if err := os.WriteFile(testFile, testContent, 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	hash, err := ComputeSHA512(testFile)
	if err != nil {
		t.Fatalf("failed to compute SHA512: %v", err)
	}

	expected := "309ecc489c12d6eb4cc40f50c902f2b4d0ed77ee511a7c7a9bcd3ca86d4cd86f989dd35bc5ff499670da34255b45b0cfd830e81f605dcf7dc5542e93ae9cd76f"
	if hash != expected {
		t.Errorf("expected %s, got %s", expected, hash)
	}
}
