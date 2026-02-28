//go:build mage
// +build mage

package main

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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

	return sh.RunV("go", "build", "-o", output, "./cmd/neo-blackbox")
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
	return sh.RunV("go", "run", "./cmd/neo-blackbox", "-config", configFile)
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
	return sh.RunV("go", "install", "./cmd/neo-blackbox")
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
	return sh.RunV("go", "run", "./cmd/neo-blackbox", "-config", absPath)
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
	if err := sh.RunWith(buildEnv, "go", "build", "-o", binaryOutput, "./cmd/neo-blackbox"); err != nil {
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

	// Copy web directory → bin/web/ (서버가 실행파일 기준 상대경로로 탐색)
	if _, err := os.Stat("web"); err == nil {
		webDestDir := filepath.Join(packageBinDir, "web")
		fmt.Printf("Copying web to %s\n", webDestDir)
		if err := copyDir("web", webDestDir); err != nil {
			fmt.Printf("Warning: failed to copy web directory: %v\n", err)
		}
	} else {
		fmt.Println("Warning: web directory not found, skipping")
	}

	// Copy tools/{target}/ → tools/ and ai/
	// ai/ 파일: blackbox-ai-manager, blackbox-ai-core, config.json
	// tools/ 파일: 나머지 모두
	toolsSrcDir := filepath.Join("tools", target)
	if _, err := os.Stat(toolsSrcDir); err == nil {
		toolsDestDir := filepath.Join(packageDir, "tools")
		aiDestDir := filepath.Join(packageDir, "ai")

		// ai/ 하위 디렉토리 미리 생성 (런타임에 필요한 빈 디렉토리)
		for _, sub := range []string{aiDestDir, filepath.Join(aiDestDir, "models"), filepath.Join(aiDestDir, "mvs")} {
			if err := os.MkdirAll(sub, 0o755); err != nil {
				fmt.Printf("Warning: failed to create ai subdir %s: %v\n", sub, err)
			}
		}

		// 파일명 → 복사될 목적 디렉토리 (빈 문자열이면 tools/ 로 이동)
		aiFileDestDir := map[string]string{
			"blackbox-ai-manager": aiDestDir,
			"blackbox-ai-core":    aiDestDir,
			"config.json":         aiDestDir,
			"libonnxruntime.so":   aiDestDir,
		}

		entries, err := os.ReadDir(toolsSrcDir)
		if err != nil {
			fmt.Printf("Warning: failed to read tools directory: %v\n", err)
		} else {
			for _, entry := range entries {
				if entry.IsDir() {
					continue
				}
				// Windows ADS(Zone.Identifier) 파일 스킵
				if strings.Contains(entry.Name(), ":") {
					continue
				}
				src := filepath.Join(toolsSrcDir, entry.Name())
				var dest string
				if filepath.Ext(entry.Name()) == ".onnx" {
					// .onnx 모델 파일은 ai/models/ 로
					modelsDestDir := filepath.Join(aiDestDir, "models")
					if err := os.MkdirAll(modelsDestDir, 0o755); err != nil {
						fmt.Printf("Warning: failed to create models dir: %v\n", err)
						continue
					}
					dest = filepath.Join(modelsDestDir, entry.Name())
				} else if destDir, ok := aiFileDestDir[entry.Name()]; ok {
					if err := os.MkdirAll(destDir, 0o755); err != nil {
						fmt.Printf("Warning: failed to create dir %s: %v\n", destDir, err)
						continue
					}
					dest = filepath.Join(destDir, entry.Name())
				} else {
					if err := os.MkdirAll(toolsDestDir, 0o755); err != nil {
						fmt.Printf("Warning: failed to create tools dir: %v\n", err)
						continue
					}
					dest = filepath.Join(toolsDestDir, entry.Name())
				}
				fmt.Printf("Copying %s to %s\n", src, dest)
				if err := sh.Copy(dest, src); err != nil {
					fmt.Printf("Warning: failed to copy %s: %v\n", entry.Name(), err)
					continue
				}
				// 실행 파일 권한 부여 (Windows 제외)
				if targetOS != "windows" {
					info, _ := entry.Info()
					if info.Mode()&0o111 != 0 {
						_ = os.Chmod(dest, 0o755)
					}
				}
			}
		}
	} else {
		fmt.Printf("Warning: tools/%s not found, skipping\n", target)
	}

	// 특정 실행 파일에 명시적으로 실행 권한 부여 (Windows 제외)
	// 소스 파일의 권한 상태와 무관하게 항상 0755로 설정합니다.
	if targetOS != "windows" {
		knownExecutables := []string{
			filepath.Join(packageDir, "tools", "ffmpeg"),
			filepath.Join(packageDir, "tools", "ffprobe"),
			filepath.Join(packageDir, "tools", "mediamtx"),
			filepath.Join(packageDir, "ai", "blackbox-ai-core"),
			filepath.Join(packageDir, "ai", "blackbox-ai-manager"),
		}
		for _, exePath := range knownExecutables {
			if _, err := os.Stat(exePath); err != nil {
				continue // 파일이 없으면 스킵
			}
			if err := os.Chmod(exePath, 0o755); err != nil {
				fmt.Printf("Warning: failed to chmod +x %s: %v\n", filepath.Base(exePath), err)
			} else {
				fmt.Printf("chmod +x %s\n", exePath)
			}
		}
	}

	// Copy backend runtime files required by neo package contract.
	backendDir := filepath.Join(packageDir, ".backend")
	if err := os.MkdirAll(backendDir, 0755); err != nil {
		return fmt.Errorf("failed to create backend directory: %w", err)
	}

	backendConfigSrc := ".backend.yml"
	backendConfigDest := filepath.Join(packageDir, ".backend.yml")
	if _, err := os.Stat(backendConfigSrc); err != nil {
		return fmt.Errorf("missing required backend config %s: %w", backendConfigSrc, err)
	}
	fmt.Printf("Copying %s to %s\n", backendConfigSrc, backendConfigDest)
	if err := sh.Copy(backendConfigDest, backendConfigSrc); err != nil {
		return fmt.Errorf("failed to copy backend config: %w", err)
	}

	for _, script := range []string{"start.sh", "stop.sh"} {
		scriptSrc := filepath.Join("scripts", script)
		scriptDest := filepath.Join(backendDir, script)

		if _, err := os.Stat(scriptSrc); err != nil {
			return fmt.Errorf("missing required backend script %s: %w", scriptSrc, err)
		}
		fmt.Printf("Copying %s to %s\n", scriptSrc, scriptDest)
		if err := sh.Copy(scriptDest, scriptSrc); err != nil {
			return fmt.Errorf("failed to copy backend script %s: %w", script, err)
		}
		if targetOS != "windows" {
			if err := os.Chmod(scriptDest, 0755); err != nil {
				return fmt.Errorf("failed to make backend script executable %s: %w", scriptDest, err)
			}
		}
	}

	// Copy frontend single-file bundle at repository root.
	// External packager flattens {packageDir} -> dist/, so this becomes dist/index.html.
	rootIndexSrc := "index.html"
	rootIndexDest := filepath.Join(packageDir, "index.html")
	if _, err := os.Stat(rootIndexSrc); err != nil {
		return fmt.Errorf("required frontend bundle not found: %s: %w", rootIndexSrc, err)
	}
	fmt.Printf("Copying %s to %s\n", rootIndexSrc, rootIndexDest)
	if err := sh.Copy(rootIndexDest, rootIndexSrc); err != nil {
		return fmt.Errorf("failed to copy frontend bundle: %w", err)
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
- .backend.yml and .backend/: Runtime launcher configuration and scripts
- ai/: AI manager and core binaries (blackbox-ai-manager, blackbox-ai-core, config.json)
  - ai/models/: AI model files
  - ai/mvs/: MVS working files

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

// createTarGz creates a tar.gz archive using Go's native implementation.
// 시스템 tar 명령어 대신 사용하여 exit code 1 (warning) 문제를 피한다.
func createTarGz(sourceDir, targetFile string) error {
	out, err := os.Create(targetFile)
	if err != nil {
		return fmt.Errorf("create archive: %w", err)
	}
	defer out.Close()

	gw := gzip.NewWriter(out)
	tw := tar.NewWriter(gw)

	base := filepath.Base(sourceDir)
	walkErr := filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return err
		}
		// archive 내 경로: {packageName}/{rel}
		arcName := filepath.Join(base, rel)
		if info.IsDir() {
			return tw.WriteHeader(&tar.Header{
				Typeflag: tar.TypeDir,
				Name:     arcName + "/",
				Mode:     int64(info.Mode()),
				ModTime:  info.ModTime(),
			})
		}

		hdr := &tar.Header{
			Typeflag: tar.TypeReg,
			Name:     arcName,
			Size:     info.Size(),
			Mode:     int64(info.Mode()),
			ModTime:  info.ModTime(),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}

		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = io.Copy(tw, f)
		return err
	})

	if walkErr != nil {
		tw.Close()
		gw.Close()
		return walkErr
	}

	// tar EOF marker 명시적으로 닫고 에러 체크
	if err := tw.Close(); err != nil {
		gw.Close()
		return fmt.Errorf("finalize tar: %w", err)
	}
	// gzip footer 명시적으로 플러시하고 에러 체크
	if err := gw.Close(); err != nil {
		return fmt.Errorf("finalize gzip: %w", err)
	}
	return nil
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

	// Load .env file
	env, err := loadEnv()
	if err != nil {
		fmt.Printf("Warning: failed to load .env file: %v\n", err)
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
	remoteUser := getEnvOrDefault(env, "DEPLOY_G4U_USER", "demo")
	remoteHost := getEnvOrDefault(env, "DEPLOY_G4U_HOST", "192.168.1.185")
	remotePath := getEnvOrDefault(env, "DEPLOY_G4U_PATH", "/data/pkgs")

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

// ToolsAI downloads the latest blackbox-ai release for the specified target
// from GitHub (machbase/neo-blackbox-ai) and extracts it into tools/{target}/.
// GITHUB_TOKEN은 환경변수 또는 .env 파일에서 읽습니다.
// Usage: mage toolsai linux-amd64
func ToolsAI(target string) error {
	parts := strings.SplitN(target, "-", 2)
	if len(parts) != 2 {
		return fmt.Errorf("invalid target %q: expected format os-arch (e.g. linux-amd64)", target)
	}
	targetOS := parts[0]

	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		env, _ := loadEnv()
		token = env["GITHUB_TOKEN"]
	}

	ext := "tar.gz"
	if targetOS == "windows" {
		ext = "zip"
	}

	// asset 이름 패턴: blackbox-ai-v{version}-{target}.{ext}
	prefix := "blackbox-ai-"
	suffix := fmt.Sprintf("-%s.%s", target, ext)

	const owner, repo = "machbase", "neo-blackbox-ai"
	assetURL, assetName, releaseTag, err := githubReleaseAssetByPattern(owner, repo, token, prefix, suffix)
	if err != nil {
		return fmt.Errorf("find asset for %s: %w", target, err)
	}
	fmt.Printf("Found: %s (release: %s)\n", assetName, releaseTag)

	destDir := filepath.Join("tools", target)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("create tools dir: %w", err)
	}

	// Download
	req, err := http.NewRequest("GET", assetURL, nil)
	if err != nil {
		return err
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	req.Header.Set("Accept", "application/octet-stream")

	client := &http.Client{Timeout: 10 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("download returned %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	fmt.Printf("Extracting to %s/...\n", destDir)

	if targetOS == "windows" {
		// zip은 random access 필요 → 임시 파일로 저장 후 추출
		if err := os.MkdirAll(tmpDir, 0755); err != nil {
			return err
		}
		tmpFile := filepath.Join(tmpDir, assetName)
		f, err := os.Create(tmpFile)
		if err != nil {
			return fmt.Errorf("create tmp file: %w", err)
		}
		if _, err := io.Copy(f, resp.Body); err != nil {
			f.Close()
			return fmt.Errorf("write tmp file: %w", err)
		}
		f.Close()
		defer os.Remove(tmpFile)
		return extractZipFlat(tmpFile, destDir)
	}

	return extractTarGzStream(resp.Body, destDir)
}

// githubReleaseAssetByPattern은 최신 release에서 prefix와 suffix가 모두 일치하는
// asset의 API URL, 파일명, 태그를 반환합니다.
func githubReleaseAssetByPattern(owner, repo, token, prefix, suffix string) (assetURL, assetName, tagName string, err error) {
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases", owner, repo)
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return "", "", "", err
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", "", "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", "", "", fmt.Errorf("GitHub API returned %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var releases []struct {
		TagName string `json:"tag_name"`
		Assets  []struct {
			Name string `json:"name"`
			URL  string `json:"url"`
		} `json:"assets"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return "", "", "", fmt.Errorf("decode response: %w", err)
	}
	if len(releases) == 0 {
		return "", "", "", fmt.Errorf("no releases found")
	}

	for _, rel := range releases {
		for _, asset := range rel.Assets {
			if strings.HasPrefix(asset.Name, prefix) && strings.HasSuffix(asset.Name, suffix) {
				return asset.URL, asset.Name, rel.TagName, nil
			}
		}
	}
	return "", "", "", fmt.Errorf("no asset matching prefix=%q suffix=%q found in any release", prefix, suffix)
}

// extractZipFlat extracts a zip archive flat (top-level files only, no dir structure) into destDir.
func extractZipFlat(zipPath, destDir string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("open zip: %w", err)
	}
	defer r.Close()

	for _, f := range r.File {
		if f.FileInfo().IsDir() {
			continue
		}
		name := filepath.Base(f.Name)
		if name == "" || name == "." {
			continue
		}
		destPath := filepath.Join(destDir, name)
		rc, err := f.Open()
		if err != nil {
			return fmt.Errorf("open zip entry %s: %w", name, err)
		}
		out, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, f.Mode()&0o777)
		if err != nil {
			rc.Close()
			return fmt.Errorf("create %s: %w", name, err)
		}
		if _, err := io.Copy(out, rc); err != nil {
			rc.Close()
			out.Close()
			return fmt.Errorf("write %s: %w", name, err)
		}
		rc.Close()
		out.Close()
		fmt.Printf("  Extracted: %s\n", name)
	}
	return nil
}

// extractTarGzStream은 io.Reader로 받은 tar.gz를 destDir에 flat하게 추출합니다.
func extractTarGzStream(r io.Reader, destDir string) error {
	gr, err := gzip.NewReader(r)
	if err != nil {
		return fmt.Errorf("gzip reader: %w", err)
	}
	defer gr.Close()

	tr := tar.NewReader(gr)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("tar read: %w", err)
		}
		if hdr.Typeflag == tar.TypeDir {
			continue
		}

		name := filepath.Base(hdr.Name)
		if name == "" || name == "." {
			continue
		}

		destPath := filepath.Join(destDir, name)
		f, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(hdr.Mode)&0o777)
		if err != nil {
			return fmt.Errorf("create %s: %w", name, err)
		}
		if _, err := io.Copy(f, tr); err != nil {
			f.Close()
			return fmt.Errorf("write %s: %w", name, err)
		}
		f.Close()
		fmt.Printf("  Extracted: %s\n", name)
	}
	return nil
}
