package main

import (
	"debug/elf"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The image itself is not built by tests (no Docker). These checks lint the
// Dockerfile structurally and build the binary exactly as its build stage does.
func TestDockerfile_DistrolessNonRootExecFormStaticBinary(t *testing.T) {
	raw, err := os.ReadFile("../../Dockerfile")
	if err != nil {
		t.Fatalf("read Dockerfile: %v", err)
	}
	df := string(raw)
	for _, re := range []string{
		`(?m)^FROM gcr\.io/distroless/static-debian12:nonroot@sha256:[0-9a-f]{64}$`,
		`(?m)^FROM golang:[\d.]+@sha256:[0-9a-f]{64} AS build$`,
		`(?m)^RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/inventory \./cmd/inventory$`,
		`(?m)^USER 65532:65532$`,
		`(?m)^ENTRYPOINT \["/inventory"\]$`,
	} {
		if !regexp.MustCompile(re).MatchString(df) {
			t.Errorf("Dockerfile does not match %s", re)
		}
	}
	if final := df[strings.LastIndex(df, "FROM gcr.io"):]; strings.Contains(final, "RUN ") {
		t.Error("final distroless stage must have no RUN (no shell, no package install)")
	}

	out := filepath.Join(t.TempDir(), "inventory")
	cmd := exec.Command("go", "build", "-trimpath", "-ldflags=-s -w", "-o", out, "./cmd/inventory")
	cmd.Dir, cmd.Env = "../..", append(os.Environ(), "CGO_ENABLED=0", "GOOS=linux")
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, b)
	}
	f, err := elf.Open(out)
	if err != nil {
		t.Fatalf("not an ELF binary: %v", err)
	}
	t.Cleanup(func() { _ = f.Close() })
	for _, p := range f.Progs {
		if p.Type == elf.PT_INTERP || p.Type == elf.PT_DYNAMIC {
			t.Fatalf("binary is dynamically linked (%v); would not run on distroless/static", p.Type)
		}
	}
}
