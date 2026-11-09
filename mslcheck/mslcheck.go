// Package mslcheck compiles emitted Metal Shading Language with Apple's
// offline compiler, `xcrun metal`, to prove the Metal emitter's output is
// valid. The check is optional: it reports Skipped on hosts without the
// Metal toolchain (Linux, or a Mac with only the Command Line Tools), so it
// gates wherever the toolchain exists and never fails elsewhere.
//
// xcrun finds the compiler through the active developer directory. A Mac
// whose xcode-select points at the Command Line Tools but that has
// Xcode.app installed still works: the check retries with
// DEVELOPER_DIR=/Applications/Xcode.app/Contents/Developer. Set DEVELOPER_DIR
// yourself to pick another Xcode.
package mslcheck

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

// Result is the outcome of one compile.
type Result struct {
	// Skipped is true when no Metal compiler is installed.
	Skipped bool
	// Output is the compiler's combined output.
	Output string
}

// Toolchain describes the Metal compiler a check uses.
type Toolchain struct {
	// Path is the metal binary xcrun resolved.
	Path string
	// DeveloperDir is the DEVELOPER_DIR the check sets, or empty when the
	// active developer directory already has the compiler.
	DeveloperDir string
}

const xcodeDeveloperDir = "/Applications/Xcode.app/Contents/Developer"

var (
	findOnce  sync.Once
	toolchain Toolchain
	found     bool
)

// Find locates the Metal compiler. ok is false on hosts without one.
func Find() (Toolchain, bool) {
	findOnce.Do(func() { toolchain, found = find() })
	return toolchain, found
}

func find() (Toolchain, bool) {
	if runtime.GOOS != "darwin" {
		return Toolchain{}, false
	}
	if path, err := xcrunFind(""); err == nil {
		return Toolchain{Path: path}, true
	}
	if os.Getenv("DEVELOPER_DIR") != "" {
		return Toolchain{}, false
	}
	if _, err := os.Stat(xcodeDeveloperDir); err != nil {
		return Toolchain{}, false
	}
	if path, err := xcrunFind(xcodeDeveloperDir); err == nil {
		return Toolchain{Path: path, DeveloperDir: xcodeDeveloperDir}, true
	}
	return Toolchain{}, false
}

func xcrunFind(developerDir string) (string, error) {
	cmd := exec.Command("xcrun", "--sdk", "macosx", "--find", "metal")
	cmd.Env = environ(developerDir)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	path := strings.TrimSpace(string(out))
	if path == "" {
		return "", errors.New("xcrun returned no metal path")
	}
	return path, nil
}

func environ(developerDir string) []string {
	env := os.Environ()
	if developerDir != "" {
		env = append(env, "DEVELOPER_DIR="+developerDir)
	}
	return env
}

// Compile compiles one MSL source to an AIR object and discards it. A
// non-nil error means the compiler rejected the source; Output holds its
// diagnostics.
func Compile(source string) (Result, error) {
	tc, ok := Find()
	if !ok {
		return Result{Skipped: true}, nil
	}
	dir, err := os.MkdirTemp("", "selena-msl-*")
	if err != nil {
		return Result{}, err
	}
	defer os.RemoveAll(dir)
	src := filepath.Join(dir, "shader.metal")
	if err := os.WriteFile(src, []byte(source), 0o644); err != nil {
		return Result{}, err
	}
	cmd := exec.Command("xcrun", "--sdk", "macosx", "metal", "-c", src, "-o", filepath.Join(dir, "shader.air"))
	cmd.Dir = dir
	cmd.Env = environ(tc.DeveloperDir)
	out, err := cmd.CombinedOutput()
	res := Result{Output: string(out)}
	if err != nil {
		return res, fmt.Errorf("xcrun metal: %w\n%s", err, out)
	}
	return res, nil
}
