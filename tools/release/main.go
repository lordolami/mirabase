// Command release builds MiraBase with the actual embedded engine's digest pins.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

func main() {
	version := flag.String("version", "dev", "product version")
	targetOS := flag.String("goos", runtime.GOOS, "target OS")
	targetArch := flag.String("goarch", runtime.GOARCH, "target architecture")
	output := flag.String("output", "", "output executable")
	check := flag.Bool("check", false, "run the full native-provider Go test suite")
	flag.Parse()
	if _, err := os.Stat("go.mod"); err != nil {
		fatal(fmt.Errorf("run from the server source root: %w", err))
	}
	name := "mirabase_engine.so"
	if *targetOS == "windows" {
		name = "mirabase_engine.dll"
	}
	payload := filepath.Join("internal", "mbengine", "platform", "payload", *targetOS+"_"+*targetArch, name)
	bytes, err := os.ReadFile(payload)
	if err != nil {
		fatal(fmt.Errorf("target payload is missing; build and qualify it before packaging: %w", err))
	}
	digest := sha256.Sum256(bytes)
	pin := hex.EncodeToString(digest[:])
	if strings.ContainsAny(*version, " \t\r\n\"'") {
		fatal(fmt.Errorf("version must be a single safe value"))
	}
	flags := "-X mirabase.Version=" + *version + " -X mirabase/internal/mbengine.pinnedLibraryDigest=" + pin + " -X github.com/tursodatabase/turso-go-platform-libs.expectedDigest=" + pin
	goExe := filepath.Join(runtime.GOROOT(), "bin", "go")
	if runtime.GOOS == "windows" {
		goExe += ".exe"
	}
	args := []string{"build", "-trimpath", "-tags", "no_default_driver", "-ldflags", "-s -w " + flags}
	if *check {
		if *targetOS != runtime.GOOS || *targetArch != runtime.GOARCH {
			fatal(fmt.Errorf("test qualification must execute on its target"))
		}
		args = []string{"test", "-tags", "no_default_driver", "-ldflags", flags, "-timeout", "15m", "./..."}
	} else {
		if *output == "" {
			*output = filepath.Join("dist", "mirabase-"+*targetOS+"-"+*targetArch)
			if *targetOS == "windows" {
				*output += ".exe"
			}
		}
		if err := os.MkdirAll(filepath.Dir(*output), 0755); err != nil {
			fatal(err)
		}
		args = append(args, "-o", *output, "./examples/base")
	}
	cmd := exec.Command(goExe, args...)
	env := []string{}
	for _, item := range os.Environ() {
		if !strings.HasPrefix(item, "CGO_ENABLED=") && !strings.HasPrefix(item, "GOOS=") && !strings.HasPrefix(item, "GOARCH=") {
			env = append(env, item)
		}
	}
	cmd.Env = append(env, "CGO_ENABLED=0", "GOOS="+*targetOS, "GOARCH="+*targetArch)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		fatal(err)
	}
	if !*check {
		artifact, err := os.ReadFile(*output)
		if err != nil {
			fatal(err)
		}
		sum := sha256.Sum256(artifact)
		manifest := map[string]any{"version": *version, "target": *targetOS + "/" + *targetArch, "engineRevision": "b0e1b8fde8eff66ef9843fdd1139fa3305cb16dd", "nativeSHA256": pin, "executableSHA256": hex.EncodeToString(sum[:]), "executableBytes": len(artifact), "toolchain": runtime.Version(), "releaseQualified": false, "qualification": "Build completed; execute target tests, security/fault acceptance, signatures and launch gates separately."}
		body, _ := json.MarshalIndent(manifest, "", "  ")
		if err := os.WriteFile(*output+".manifest.json", append(body, '\n'), 0644); err != nil {
			fatal(err)
		}
		fmt.Println("Built", *output, "(qualification pending)")
	}
}

func fatal(err error) { fmt.Fprintln(os.Stderr, err); os.Exit(1) }
