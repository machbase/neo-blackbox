//go:build mage
// +build mage

package main

import (
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
	binaryName = "blackbox-backend"
	configFile = "internal/config/config.yaml"
	distDir    = "dist"
	binDir     = "bin"
)

// Build builds the blackbox-backend binary
func Build() error {
	mg.Deps(InstallDeps)
	fmt.Println("Building...")

	output := binaryName
	if runtime.GOOS == "windows" {
		output += ".exe"
	}

	return sh.RunV("go", "build", "-o", output, ".")
}

// Run runs the application with config file
func Run() error {
	mg.Deps(Build)
	fmt.Printf("Running with config: %s\n", configFile)

	binary := "./" + binaryName
	if runtime.GOOS == "windows" {
		binary = binaryName + ".exe"
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
		binaryName,
		binaryName + ".exe",
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

	binary := "./" + binaryName
	if runtime.GOOS == "windows" {
		binary = binaryName + ".exe"
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

// Package creates a distributable package with binary, ffmpeg, ffprobe, and configs
func Package() error {
	mg.Deps(Build)
	fmt.Println("Creating package...")

	// Create dist directory structure
	packageName := fmt.Sprintf("%s-%s-%s", binaryName, runtime.GOOS, runtime.GOARCH)
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
	binarySource := binaryName
	binaryDest := filepath.Join(packageBinDir, binaryName)
	if runtime.GOOS == "windows" {
		binarySource += ".exe"
		binaryDest += ".exe"
	}

	fmt.Printf("Copying %s to %s\n", binarySource, binaryDest)
	if err := sh.Copy(binaryDest, binarySource); err != nil {
		return fmt.Errorf("failed to copy binary: %w", err)
	}

	// Make binary executable
	if runtime.GOOS != "windows" {
		if err := os.Chmod(binaryDest, 0755); err != nil {
			return fmt.Errorf("failed to make binary executable: %w", err)
		}
	}

	// Copy ffmpeg
	ffmpegSource := "ffmpeg"
	ffmpegDest := filepath.Join(packageBinDir, "ffmpeg")
	if runtime.GOOS == "windows" {
		ffmpegSource += ".exe"
		ffmpegDest += ".exe"
	}

	if _, err := os.Stat(ffmpegSource); err == nil {
		fmt.Printf("Copying %s to %s\n", ffmpegSource, ffmpegDest)
		if err := sh.Copy(ffmpegDest, ffmpegSource); err != nil {
			return fmt.Errorf("failed to copy ffmpeg: %w", err)
		}
		if runtime.GOOS != "windows" {
			if err := os.Chmod(ffmpegDest, 0755); err != nil {
				return fmt.Errorf("failed to make ffmpeg executable: %w", err)
			}
		}
	} else {
		fmt.Println("Warning: ffmpeg not found, skipping")
	}

	// Copy ffprobe
	ffprobeSource := "ffprobe"
	ffprobeDest := filepath.Join(packageBinDir, "ffprobe")
	if runtime.GOOS == "windows" {
		ffprobeSource += ".exe"
		ffprobeDest += ".exe"
	}

	if _, err := os.Stat(ffprobeSource); err == nil {
		fmt.Printf("Copying %s to %s\n", ffprobeSource, ffprobeDest)
		if err := sh.Copy(ffprobeDest, ffprobeSource); err != nil {
			return fmt.Errorf("failed to copy ffprobe: %w", err)
		}
		if runtime.GOOS != "windows" {
			if err := os.Chmod(ffprobeDest, 0755); err != nil {
				return fmt.Errorf("failed to make ffprobe executable: %w", err)
			}
		}
	} else {
		fmt.Println("Warning: ffprobe not found, skipping")
	}

	// Copy config files
	configSrcDir := "internal/config"
	configDestDir := filepath.Join(packageDir, "config")
	if err := os.MkdirAll(configDestDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	configs := []string{"config.yaml", "test.yaml"}
	for _, cfg := range configs {
		src := filepath.Join(configSrcDir, cfg)
		if _, err := os.Stat(src); err == nil {
			dest := filepath.Join(configDestDir, cfg)
			fmt.Printf("Copying %s to %s\n", src, dest)
			if err := sh.Copy(dest, src); err != nil {
				fmt.Printf("Warning: failed to copy %s: %v\n", cfg, err)
			}
		}
	}

	// Create README
	readmePath := filepath.Join(packageDir, "README.txt")
	readmeContent := fmt.Sprintf(`Blackbox Backend Package
========================

Build Date: %s
Platform: %s/%s

Contents:
- bin/%s: Main application binary
- bin/ffmpeg: FFmpeg binary
- bin/ffprobe: FFprobe binary
- config/: Configuration files

Usage:
  ./bin/%s -config config/config.yaml

For more information, see the project documentation.
`, time.Now().Format("2006-01-02 15:04:05"), runtime.GOOS, runtime.GOARCH, binaryName, binaryName)

	if err := os.WriteFile(readmePath, []byte(readmeContent), 0644); err != nil {
		fmt.Printf("Warning: failed to create README: %v\n", err)
	}

	// Create archive
	archiveName := packageName
	if runtime.GOOS == "windows" {
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
