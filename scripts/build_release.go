package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
)

type Target struct {
	OS   string
	Arch string
	Name string
}

func main() {
	fmt.Println("🚀 Starting build process for Forza Horizon 6 Telemetry release binaries...")

	// 1. Build Svelte assets
	fmt.Println("\n📦 Building frontend Svelte assets...")
	npmCmd := exec.Command("npm", "run", "build")
	npmCmd.Stdout = os.Stdout
	npmCmd.Stderr = os.Stderr
	if err := npmCmd.Run(); err != nil {
		fmt.Printf("❌ Failed to build Svelte assets: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("✅ Frontend assets built successfully!")

	// 2. Prepare embedding folders
	fmt.Println("\n📁 Copying build and ui assets to go-server for embedding...")
	embedBuildDir := filepath.Join("go-server", "embed_build")
	embedUiDir := filepath.Join("go-server", "embed_ui")

	// Clean up old ones if exist
	os.RemoveAll(embedBuildDir)
	os.RemoveAll(embedUiDir)

	if err := copyDir("build", embedBuildDir); err != nil {
		fmt.Printf("❌ Failed to copy build folder: %v\n", err)
		os.Exit(1)
	}
	if err := copyDir("ui", embedUiDir); err != nil {
		fmt.Printf("❌ Failed to copy ui folder: %v\n", err)
		cleanupEmbed(embedBuildDir, embedUiDir)
		os.Exit(1)
	}
	fmt.Println("✅ Assets prepared for embedding!")

	// 3. Create dist directory
	distDir := "dist"
	if err := os.MkdirAll(distDir, 0755); err != nil {
		fmt.Printf("❌ Failed to create dist directory: %v\n", err)
		cleanupEmbed(embedBuildDir, embedUiDir)
		os.Exit(1)
	}

	// 4. Define targets
	targets := []Target{
		{OS: "windows", Arch: "amd64", Name: "fh6-telemetry-windows-amd64.exe"},
		{OS: "linux", Arch: "amd64", Name: "fh6-telemetry-linux-amd64"},
		{OS: "darwin", Arch: "amd64", Name: "fh6-telemetry-darwin-amd64"},
		{OS: "darwin", Arch: "arm64", Name: "fh6-telemetry-darwin-arm64"},
	}

	// 5. Compile for each target
	for _, target := range targets {
		outputPath := filepath.Join(distDir, target.Name)
		fmt.Printf("\n⚙️ Compiling for %s (%s)... ", target.OS, target.Arch)

		cmd := exec.Command("go", "build", "-tags", "release", "-ldflags", "-s -w", "-o", filepath.Join("..", outputPath))
		cmd.Dir = "go-server"
		cmd.Env = append(os.Environ(),
			"GOOS="+target.OS,
			"GOARCH="+target.Arch,
		)
		
		output, err := cmd.CombinedOutput()
		if err != nil {
			fmt.Printf("❌ FAILED!\nError details:\n%s\n", string(output))
			cleanupEmbed(embedBuildDir, embedUiDir)
			os.Exit(1)
		}
		fmt.Println("✅ Done!")
	}

	// 6. Cleanup
	cleanupEmbed(embedBuildDir, embedUiDir)
	fmt.Println("\n🎉 All release binaries built successfully inside the 'dist' directory:")
	for _, target := range targets {
		fmt.Printf("   - %s\n", filepath.Join(distDir, target.Name))
	}
}

func cleanupEmbed(buildDir, uiDir string) {
	fmt.Println("\n🧹 Cleaning up temporary embed directories...")
	os.RemoveAll(buildDir)
	os.RemoveAll(uiDir)
}

func copyDir(src string, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Calculate destination path
		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		targetPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			return os.MkdirAll(targetPath, info.Mode())
		}

		// Copy file content
		srcFile, err := os.Open(path)
		if err != nil {
			return err
		}
		defer srcFile.Close()

		dstFile, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode())
		if err != nil {
			return err
		}
		defer dstFile.Close()

		_, err = io.Copy(dstFile, srcFile)
		return err
	})
}
