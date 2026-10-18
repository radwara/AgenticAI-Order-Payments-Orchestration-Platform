package main

import (
	"os"
	"strings"
	"testing"
)

func TestDockerfile_DistrolessAndNonRoot(t *testing.T) {
	raw, err := os.ReadFile("../../Dockerfile")
	if err != nil {
		t.Fatalf("read Dockerfile: %v", err)
	}
	for _, want := range []string{"FROM gcr.io/distroless/static-debian12:nonroot", "USER 65532:65532"} {
		if !strings.Contains(string(raw), want) {
			t.Errorf("Dockerfile missing %q", want)
		}
	}
}
