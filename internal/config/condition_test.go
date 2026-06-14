package config

import "testing"

func TestPackageMatches_OSExpr(t *testing.T) {
	pkg := &Package{OS: "linux || darwin"}
	ok, osName, arch, err := PackageMatches(pkg, "linux", "amd64")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("expected package to match")
	}
	if osName != "linux" || arch != "amd64" {
		t.Fatalf("unexpected effective values: %s %s", osName, arch)
	}

	ok, _, _, err = PackageMatches(pkg, "windows", "amd64")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Fatal("expected package to be skipped")
	}
}

func TestPackageMatches_ArchExpr(t *testing.T) {
	pkg := &Package{Arch: "amd64 || arm64"}
	ok, _, _, err := PackageMatches(pkg, "linux", "arm64")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("expected package to match")
	}

	ok, _, _, err = PackageMatches(pkg, "linux", "386")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Fatal("expected package to be skipped")
	}
}

func TestPackageMatches_ExactMismatch(t *testing.T) {
	pkg := &Package{OS: "darwin", Arch: "arm64"}
	ok, _, _, err := PackageMatches(pkg, "linux", "arm64")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Fatal("expected mismatch to skip")
	}
}

func TestPackageMatches_NormalizeArch(t *testing.T) {
	pkg := &Package{Arch: "x86_64"}
	ok, _, _, err := PackageMatches(pkg, "linux", "amd64")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("expected arch normalization to match")
	}
}

func TestPackageMatches_InvalidExpr(t *testing.T) {
	pkg := &Package{OS: "linux &&"}
	_, _, _, err := PackageMatches(pkg, "linux", "amd64")
	if err == nil {
		t.Fatal("expected error for invalid expression")
	}
}

func TestPackageMatches_ComplexExpressionsAndErrors(t *testing.T) {
	pkg := &Package{OS: "!(windows) && (linux || darwin)", Arch: "!(386) && amd64"}
	ok, _, _, err := PackageMatches(pkg, "linux", "amd64")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("expected complex expression to match")
	}

	pkg = &Package{OS: "linux && @ windows"}
	if _, _, _, err := PackageMatches(pkg, "linux", "amd64"); err == nil {
		t.Fatal("expected tokenizer error")
	}

	pkg = &Package{OS: "(linux || darwin"}
	if _, _, _, err := PackageMatches(pkg, "linux", "amd64"); err == nil {
		t.Fatal("expected missing paren error")
	}

	pkg = &Package{Arch: "amd64 &&"}
	if _, _, _, err := PackageMatches(pkg, "linux", "amd64"); err == nil {
		t.Fatal("expected arch expression error")
	}
}
