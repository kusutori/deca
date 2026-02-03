package install

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"syscall"

	"golang.org/x/term"
)

// InstallSystemPackage installs a system package (.deb, .rpm)
func (i *Installer) InstallSystemPackage(name string, assetName string, pkgType string) error {
	// Determine package manager based on type
	var pkgManager string
	var installArgs []string

	switch pkgType {
	case "deb":
		pkgManager = "apt"
		installArgs = []string{"install", "-y", "./" + assetName}
	case "rpm":
		// Try dnf first, then yum
		if _, err := exec.LookPath("dnf"); err == nil {
			pkgManager = "dnf"
		} else {
			pkgManager = "yum"
		}
		installArgs = []string{"install", "-y", "./" + assetName}
	default:
		return fmt.Errorf("unsupported package type: %s", pkgType)
	}

	// Check if we need sudo
	// Root user doesn't need sudo
	if syscall.Getuid() == 0 {
		// Running as root, just install
		cmd := exec.Command(pkgManager, installArgs...)
		cmd.Dir = i.BinDir
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}

	// Check if sudo is available
	if _, err := exec.LookPath("sudo"); err != nil {
		return fmt.Errorf("sudo is required but not available")
	}

	// Check if sudo is cached (no password needed)
	cmd := exec.Command("sudo", "-n", pkgManager, "--version")
	if err := cmd.Run(); err == nil {
		// sudo is cached, run without password prompt
		sudoArgs := append([]string{pkgManager}, installArgs...)
		sudoCmd := exec.Command("sudo", sudoArgs...)
		sudoCmd.Dir = i.BinDir
		sudoCmd.Stdin = os.Stdin
		sudoCmd.Stdout = os.Stdout
		sudoCmd.Stderr = os.Stderr
		return sudoCmd.Run()
	}

	// Need to prompt for password
	fmt.Printf("Installing %s requires sudo privileges.\n", name)
	fmt.Print("Password: ")

	// Read password securely
	password, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Println()
	if err != nil {
		return fmt.Errorf("failed to read password: %w", err)
	}

	// Run the install command with sudo
	sudoArgs := append([]string{pkgManager}, installArgs...)
	sudoCmd := exec.Command("sudo", sudoArgs...)
	sudoCmd.Dir = i.BinDir
	sudoCmd.Stdout = os.Stdout
	sudoCmd.Stderr = os.Stderr

	// Set up the password pipe
	pipe, err := sudoCmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	// Start the command
	if err := sudoCmd.Start(); err != nil {
		return fmt.Errorf("failed to start sudo: %w", err)
	}

	// Write password followed by newline (sudo expects password on stdin)
	pipe.Write([]byte(string(password) + "\n"))
	pipe.Close()

	// Wait for command to complete
	if err := sudoCmd.Wait(); err != nil {
		return fmt.Errorf("installation failed: %w", err)
	}

	return nil
}

// DetectPackageType detects the package type from filename
func DetectPackageType(filename string) string {
	filename = strings.ToLower(filename)
	switch {
	case strings.HasSuffix(filename, ".deb"):
		return "deb"
	case strings.HasSuffix(filename, ".rpm"):
		return "rpm"
	case strings.HasSuffix(filename, ".apk"):
		return "apk"
	case strings.HasSuffix(filename, ".dmg"):
		return "dmg"
	default:
		return ""
	}
}

// InstallWithSystemManager installs using system package manager if applicable
func (i *Installer) InstallWithSystemManager(name, assetName string) (bool, error) {
	pkgType := DetectPackageType(assetName)
	if pkgType == "" {
		return false, nil // Not a system package
	}

	// Only support system packages on Linux
	if runtime.GOOS != "linux" {
		return false, fmt.Errorf("system packages (%s) are only supported on Linux", pkgType)
	}

	return true, i.InstallSystemPackage(name, assetName, pkgType)
}

// RunSudoCommand runs a command with sudo, prompting for password if needed
func RunSudoCommand(cmd *exec.Cmd) error {
	// Root user doesn't need sudo
	if syscall.Getuid() == 0 {
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}

	// Check if sudo is cached
	if _, err := exec.LookPath("sudo"); err != nil {
		return fmt.Errorf("sudo is required but not available")
	}

	// Check if sudo is cached (no password needed)
	testCmd := exec.Command("sudo", "-n", "true")
	if err := testCmd.Run(); err == nil {
		// Sudo is cached, prepend sudo to command
		sudoArgs := append([]string{cmd.Args[0]}, cmd.Args[1:]...)
		sudoCmd := exec.Command("sudo", sudoArgs...)
		sudoCmd.Dir = cmd.Dir
		sudoCmd.Stdin = cmd.Stdin
		sudoCmd.Stdout = cmd.Stdout
		sudoCmd.Stderr = cmd.Stderr
		return sudoCmd.Run()
	}

	// Need to prompt for password
	fmt.Print("Password: ")

	// Read password securely
	password, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Println()
	if err != nil {
		return fmt.Errorf("failed to read password: %w", err)
	}

	// Prepare sudo command
	sudoArgs := append([]string{cmd.Args[0]}, cmd.Args[1:]...)
	sudoCmd := exec.Command("sudo", sudoArgs...)
	sudoCmd.Dir = cmd.Dir
	sudoCmd.Stdout = os.Stdout
	sudoCmd.Stderr = os.Stderr

	// Set up the password pipe
	pipe, err := sudoCmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	// Start the command
	if err := sudoCmd.Start(); err != nil {
		return fmt.Errorf("failed to start sudo: %w", err)
	}

	// Write password
	pipe.Write([]byte(string(password) + "\n"))
	pipe.Close()

	// Wait for command to complete
	if err := sudoCmd.Wait(); err != nil {
		return fmt.Errorf("command failed: %w", err)
	}

	return nil
}

// AskSudoPassword prompts user for sudo password and returns it
func AskSudoPassword() ([]byte, error) {
	fmt.Print("Password: ")
	return term.ReadPassword(int(os.Stdin.Fd()))
}

// PromptSudo prompts for password and runs command with sudo
func PromptSudo(command string, args ...string) error {
	fmt.Printf("This operation requires sudo: %s %s\n", command, strings.Join(args, " "))
	fmt.Print("Password: ")

	password, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Println()
	if err != nil {
		return fmt.Errorf("failed to read password: %w", err)
	}

	// Build the full command
	fullArgs := append([]string{command}, args...)
	sudoArgs := append([]string{"-S"}, fullArgs...)

	cmd := exec.Command("sudo", sudoArgs...)
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

// ReadPassword reads a password from stdin with echo disabled
func ReadPassword(prompt string) (string, error) {
	fmt.Print(prompt)
	password, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Println()
	if err != nil {
		return "", err
	}
	return string(password), nil
}

// IsSudoCached checks if sudo password is cached (no prompt needed)
func IsSudoCached() bool {
	cmd := exec.Command("sudo", "-n", "true")
	return cmd.Run() == nil
}

// SudoRun runs a command with sudo, handling password prompt
func SudoRun(name string, args ...string) error {
	// Check if running as root
	if syscall.Getuid() == 0 {
		cmd := exec.Command(name, args...)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}

	// Check if sudo is available
	if _, err := exec.LookPath("sudo"); err != nil {
		return fmt.Errorf("sudo is not available")
	}

	// Check if sudo is cached
	if IsSudoCached() {
		cmd := exec.Command("sudo", append([]string{name}, args...)...)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}

	// Need password
	fmt.Printf("This operation requires sudo: %s %s\n", name, strings.Join(args, " "))
	fmt.Print("Password: ")

	password, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Println()
	if err != nil {
		return fmt.Errorf("failed to read password: %w", err)
	}

	// Build command args: sudo -S <name> <args...>
	sudoArgs := []string{"-S", name}
	sudoArgs = append(sudoArgs, args...)
	cmd := exec.Command("sudo", sudoArgs...)
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
