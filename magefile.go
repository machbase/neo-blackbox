//go:build mage
// +build mage

package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
)

const (
	binaryName = "neo-blackbox"
	configFile = "internal/config/config.yaml"
	distDir    = "dist"
	binDir     = "bin"
	tmpDir     = "tmp"
)

// Build builds the neo-blackbox binary (CGO disabled for static linking)
func Build() error {
	mg.Deps(InstallDeps)
	fmt.Println("Building (CGO_ENABLED=0)...")

	os.Setenv("CGO_ENABLED", "0")

	// Create tmp directory if it doesn't exist
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		return fmt.Errorf("failed to create tmp directory: %w", err)
	}

	output := filepath.Join(tmpDir, binaryName)
	if runtime.GOOS == "windows" {
		output += ".exe"
	}

	return sh.RunV("go", "build", "-o", output, ".")
}

// Run runs the application with config file
func Run() error {
	mg.Deps(Build)
	fmt.Printf("Running with config: %s\n", configFile)

	binary := filepath.Join(tmpDir, binaryName)
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}

	return sh.RunV(binary, "-config", configFile)
}

// Dev runs the application in development mode (with config.yaml)
func Dev() error {
	fmt.Printf("Running in dev mode with config: %s\n", configFile)
	return sh.RunV("go", "run", ".", "-config", configFile)
}

// Test runs all tests
func Test() error {
	fmt.Println("Running tests...")
	return sh.RunV("go", "test", "-v", "./...")
}

// Clean removes build artifacts
func Clean() error {
	fmt.Println("Cleaning...")

	files := []string{
		filepath.Join(tmpDir, binaryName),
		filepath.Join(tmpDir, binaryName+".exe"),
	}

	for _, f := range files {
		if err := sh.Rm(f); err != nil {
			// Ignore errors if file doesn't exist
			if !os.IsNotExist(err) {
				return err
			}
		}
	}

	return nil
}

// CleanDist removes the dist directory
func CleanDist() error {
	fmt.Println("Cleaning dist directory...")
	return sh.Rm(distDir)
}

// CleanAll removes all build artifacts and dist directory
func CleanAll() error {
	mg.Deps(Clean, CleanDist)
	return nil
}

// InstallDeps installs Go dependencies
func InstallDeps() error {
	fmt.Println("Installing dependencies...")
	return sh.RunV("go", "mod", "download")
}

// Fmt formats the code
func Fmt() error {
	fmt.Println("Formatting code...")
	return sh.RunV("go", "fmt", "./...")
}

// Vet runs go vet
func Vet() error {
	fmt.Println("Running go vet...")
	return sh.RunV("go", "vet", "./...")
}

// Check runs fmt, vet, and test
func Check() error {
	mg.Deps(Fmt, Vet)
	return Test()
}

// Install builds and installs the binary to $GOPATH/bin
func Install() error {
	fmt.Println("Installing...")
	return sh.RunV("go", "install", ".")
}

// RunWithConfig runs the application with a custom config file
// Usage: mage runwithconfig path/to/config.yaml
func RunWithConfig(configPath string) error {
	mg.Deps(Build)

	absPath, err := filepath.Abs(configPath)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		return fmt.Errorf("config file not found: %s", absPath)
	}

	fmt.Printf("Running with config: %s\n", absPath)

	binary := filepath.Join(tmpDir, binaryName)
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}

	return sh.RunV(binary, "-config", absPath)
}

// DevWithConfig runs the application in dev mode with a custom config file
// Usage: mage devwithconfig path/to/config.yaml
func DevWithConfig(configPath string) error {
	absPath, err := filepath.Abs(configPath)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		return fmt.Errorf("config file not found: %s", absPath)
	}

	fmt.Printf("Running in dev mode with config: %s\n", absPath)
	return sh.RunV("go", "run", ".", "-config", absPath)
}

// Version prints version information
func Version() error {
	cmd := exec.Command("go", "version")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// Package creates a distributable package for the specified target platform.
// Usage: mage package linux-amd64
// Supported targets: linux-amd64, linux-arm64, darwin-amd64, darwin-arm64, windows-amd64
func Package(target string) error {
	parts := strings.SplitN(target, "-", 2)
	if len(parts) != 2 {
		return fmt.Errorf("invalid target %q: expected format os-arch (e.g. linux-amd64)", target)
	}
	targetOS, targetArch := parts[0], parts[1]

	fmt.Printf("Building for %s/%s (CGO_ENABLED=0)...\n", targetOS, targetArch)
	mg.Deps(InstallDeps)

	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		return fmt.Errorf("failed to create tmp directory: %w", err)
	}

	// Cross-compile binary
	binaryOutput := filepath.Join(tmpDir, binaryName+"-"+target)
	if targetOS == "windows" {
		binaryOutput += ".exe"
	}

	buildEnv := map[string]string{
		"CGO_ENABLED": "0",
		"GOOS":        targetOS,
		"GOARCH":      targetArch,
	}
	if err := sh.RunWith(buildEnv, "go", "build", "-o", binaryOutput, "."); err != nil {
		return fmt.Errorf("failed to build for %s: %w", target, err)
	}
	fmt.Printf("Built: %s\n", binaryOutput)

	// Create dist directory structure
	packageName := fmt.Sprintf("%s-%s", binaryName, target)
	packageDir := filepath.Join(distDir, packageName)
	packageBinDir := filepath.Join(packageDir, binDir)

	// Clean and create directories
	if err := sh.Rm(distDir); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to clean dist: %w", err)
	}
	if err := os.MkdirAll(packageBinDir, 0755); err != nil {
		return fmt.Errorf("failed to create package bin directory: %w", err)
	}

	// Copy binary
	binaryDest := filepath.Join(packageBinDir, binaryName)
	if targetOS == "windows" {
		binaryDest += ".exe"
	}
	fmt.Printf("Copying %s to %s\n", binaryOutput, binaryDest)
	if err := sh.Copy(binaryDest, binaryOutput); err != nil {
		return fmt.Errorf("failed to copy binary: %w", err)
	}
	if targetOS != "windows" {
		if err := os.Chmod(binaryDest, 0755); err != nil {
			return fmt.Errorf("failed to make binary executable: %w", err)
		}
	}

	// Copy config files
	configSrcDir := "internal/config"
	configDestDir := filepath.Join(packageDir, "config")
	if err := os.MkdirAll(configDestDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}
	for _, cfg := range []string{"config.yaml", "test.yaml"} {
		src := filepath.Join(configSrcDir, cfg)
		if _, err := os.Stat(src); err == nil {
			dest := filepath.Join(configDestDir, cfg)
			fmt.Printf("Copying %s to %s\n", src, dest)
			if err := sh.Copy(dest, src); err != nil {
				fmt.Printf("Warning: failed to copy %s: %v\n", cfg, err)
			}
		}
	}

	// Copy web directory
	if _, err := os.Stat("web"); err == nil {
		webDestDir := filepath.Join(packageDir, "web")
		fmt.Printf("Copying web to %s\n", webDestDir)
		if err := copyDir("web", webDestDir); err != nil {
			fmt.Printf("Warning: failed to copy web directory: %v\n", err)
		}
	} else {
		fmt.Println("Warning: web directory not found, skipping")
	}

	// Copy tools/{target}/ directory (only the matching platform)
	toolsSrcDir := filepath.Join("tools", target)
	if _, err := os.Stat(toolsSrcDir); err == nil {
		toolsDestDir := filepath.Join(packageDir, "tools")
		fmt.Printf("Copying %s to %s\n", toolsSrcDir, toolsDestDir)
		if err := copyDir(toolsSrcDir, toolsDestDir); err != nil {
			fmt.Printf("Warning: failed to copy tools directory: %v\n", err)
		}
	} else {
		fmt.Printf("Warning: tools/%s not found, skipping\n", target)
	}

	// Create README
	readmeContent := fmt.Sprintf(`Blackbox Backend Package
========================

Build Date: %s
Platform: %s

Contents:
- bin/%s: Main application binary
- config/: Configuration files
- tools/: Platform-specific tools (ffmpeg, mediamtx, ai manager, ...)

Usage:
  ./bin/%s -config config/config.yaml

For more information, see the project documentation.
`, time.Now().Format("2006-01-02 15:04:05"), target, binaryName, binaryName)

	if err := os.WriteFile(filepath.Join(packageDir, "README.txt"), []byte(readmeContent), 0644); err != nil {
		fmt.Printf("Warning: failed to create README: %v\n", err)
	}

	// Create archive
	archiveName := packageName
	if targetOS == "windows" {
		archiveName += ".zip"
		fmt.Printf("Creating archive %s...\n", archiveName)
		if err := createZip(packageDir, filepath.Join(distDir, archiveName)); err != nil {
			return fmt.Errorf("failed to create zip: %w", err)
		}
	} else {
		archiveName += ".tar.gz"
		fmt.Printf("Creating archive %s...\n", archiveName)
		if err := createTarGz(packageDir, filepath.Join(distDir, archiveName)); err != nil {
			return fmt.Errorf("failed to create tar.gz: %w", err)
		}
	}

	fmt.Printf("\n✓ Package created: %s\n", filepath.Join(distDir, archiveName))
	return nil
}

// createTarGz creates a tar.gz archive
func createTarGz(sourceDir, targetFile string) error {
	dir := filepath.Dir(sourceDir)
	base := filepath.Base(sourceDir)

	return sh.RunV("tar", "-czf", targetFile, "-C", dir, base)
}

// createZip creates a zip archive
func createZip(sourceDir, targetFile string) error {
	dir := filepath.Dir(sourceDir)
	base := filepath.Base(sourceDir)

	// Use PowerShell on Windows
	if runtime.GOOS == "windows" {
		absSource, _ := filepath.Abs(sourceDir)
		absTarget, _ := filepath.Abs(targetFile)
		cmd := fmt.Sprintf("Compress-Archive -Path '%s' -DestinationPath '%s' -Force", absSource, strings.TrimSuffix(absTarget, ".zip"))
		return sh.RunV("powershell", "-Command", cmd)
	}

	// Use zip command on Unix-like systems
	return sh.RunV("zip", "-r", targetFile, base, "-C", dir)
}

// copyDir recursively copies a directory
func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Get relative path
		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}

		// Target path
		targetPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			// Create directory
			return os.MkdirAll(targetPath, info.Mode())
		}

		// Copy file
		return sh.Copy(targetPath, path)
	})
}

// loadEnv reads .env file and returns a map of key-value pairs
func loadEnv() (map[string]string, error) {
	env := make(map[string]string)

	file, err := os.Open(".env")
	if err != nil {
		return env, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse KEY=VALUE
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			// Remove quotes if present
			value = strings.Trim(value, `"'`)
			env[key] = value
		}
	}

	return env, scanner.Err()
}

// Dp (Deploy Package) deploys the package to remote server via scp.
// Usage: mage dp linux-amd64
func Dp(target string) error {
	// Run package first
	if err := Package(target); err != nil {
		return fmt.Errorf("failed to package: %w", err)
	}

	// Load .env file
	env, err := loadEnv()
	if err != nil {
		fmt.Printf("Warning: failed to load .env file: %v\n", err)
		fmt.Println("Using default values...")
		env = make(map[string]string)
	}

	// Find the created archive
	targetOS := strings.SplitN(target, "-", 2)[0]
	packageName := fmt.Sprintf("%s-%s", binaryName, target)
	archiveName := packageName + ".tar.gz"
	if targetOS == "windows" {
		archiveName = packageName + ".zip"
	}
	archivePath := filepath.Join(distDir, archiveName)

	// Check if archive exists
	if _, err := os.Stat(archivePath); os.IsNotExist(err) {
		return fmt.Errorf("package file not found: %s", archivePath)
	}

	// Get remote server details from .env or use defaults
	remoteUser := getEnvOrDefault(env, "DEPLOY_USER", "eleven")
	remoteHost := getEnvOrDefault(env, "DEPLOY_HOST", "192.168.0.87")
	remotePath := getEnvOrDefault(env, "DEPLOY_PATH", "/blackbox/be/pkg")

	remoteTarget := fmt.Sprintf("%s@%s:%s/", remoteUser, remoteHost, remotePath)

	fmt.Printf("\n📦 Deploying %s to %s\n", archiveName, remoteTarget)
	fmt.Println("Please enter password when prompted...")
	fmt.Println()

	// Run scp command (interactive for password)
	cmd := exec.Command("scp", archivePath, remoteTarget)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to scp: %w", err)
	}

	fmt.Printf("\n✓ Deployed successfully to %s\n", remoteTarget)
	return nil
}

// DpG4u (Deploy Package to G4U) deploys the package to g4u server via scp.
// Usage: mage dpg4u linux-amd64
func DpG4u(target string) error {
	// Run package first
	if err := Package(target); err != nil {
		return fmt.Errorf("failed to package: %w", err)
	}

	// Find the created archive
	targetOS := strings.SplitN(target, "-", 2)[0]
	packageName := fmt.Sprintf("%s-%s", binaryName, target)
	archiveName := packageName + ".tar.gz"
	if targetOS == "windows" {
		archiveName = packageName + ".zip"
	}
	archivePath := filepath.Join(distDir, archiveName)

	// Check if archive exists
	if _, err := os.Stat(archivePath); os.IsNotExist(err) {
		return fmt.Errorf("package file not found: %s", archivePath)
	}

	remoteUser := "demo"
	remoteHost := "192.168.1.248"
	remotePath := "/data/pkgs"

	remoteTarget := fmt.Sprintf("%s@%s:%s/", remoteUser, remoteHost, remotePath)

	fmt.Printf("\n📦 Deploying %s to %s\n", archiveName, remoteTarget)
	fmt.Println("Please enter password when prompted...")
	fmt.Println()

	// Run scp command (interactive for password)
	cmd := exec.Command("scp", archivePath, remoteTarget)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to scp: %w", err)
	}

	fmt.Printf("\n✓ Deployed successfully to %s\n", remoteTarget)
	return nil
}

// getEnvOrDefault returns env value or default if not found
func getEnvOrDefault(env map[string]string, key, defaultValue string) string {
	if value, ok := env[key]; ok && value != "" {
		return value
	}
	return defaultValue
}
