package install

import (
	"fmt"
	"os"
	"strings"

	"golang.org/x/term"
)

var termReadPasswordFunc = term.ReadPassword

// DetectPackageType detects the package type from filename
func DetectPackageType(filename string) string {
	filename = strings.ToLower(filename)
	switch {
	case strings.HasSuffix(filename, ".deb"):
		return "deb"
	case strings.HasSuffix(filename, ".rpm"):
		return "rpm"
	case strings.HasSuffix(filename, ".msi"):
		return "msi"
	case strings.HasSuffix(filename, ".apk"):
		return "apk"
	case strings.HasSuffix(filename, ".dmg"):
		return "dmg"
	default:
		return ""
	}
}

// IsSudoCached checks if sudo password is cached (no prompt needed)
func IsSudoCached() bool {
	cmd := execCommandFunc("sudo", "-n", "true")
	return cmd.Run() == nil
}

// SudoRun runs a command with sudo, handling password prompt
func SudoRun(name string, args ...string) error {
	// Check if running as root
	if getuidFunc() == 0 {
		cmd := execCommandFunc(name, args...)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}

	// Check if sudo is available
	if _, err := execLookPathFunc("sudo"); err != nil {
		return fmt.Errorf("sudo is not available")
	}

	// Check if sudo is cached
	if IsSudoCached() {
		cmd := execCommandFunc("sudo", append([]string{name}, args...)...)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}

	// Need password
	fmt.Printf("This operation requires sudo: %s %s\n", name, strings.Join(args, " "))
	fmt.Print("Password: ")

	password, err := termReadPasswordFunc(int(os.Stdin.Fd()))
	fmt.Println()
	if err != nil {
		return fmt.Errorf("failed to read password: %w", err)
	}

	// Build command args: sudo -S <name> <args...>
	sudoArgs := []string{"-S", name}
	sudoArgs = append(sudoArgs, args...)
	cmd := execCommandFunc("sudo", sudoArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	pipe, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start sudo: %w", err)
	}

	pipe.Write([]byte(string(password) + "\n"))
	pipe.Close()

	return cmd.Wait()
}
