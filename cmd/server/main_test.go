package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestNonInteractiveStartupFailureReturnsWithoutReadingInput(t *testing.T) {
	in := strings.NewReader("sentinel\n")
	before := in.Len()
	var out bytes.Buffer

	pauseOnStartupFailure(true, in, &out)

	if got := in.Len(); got != before {
		t.Fatalf("non-interactive mode consumed input: before=%d after=%d", before, got)
	}
	if out.Len() != 0 {
		t.Fatalf("non-interactive mode wrote prompt %q", out.String())
	}
}

func TestInteractiveStartupFailurePromptsAndReturnsAfterEnter(t *testing.T) {
	in := strings.NewReader("\n")
	var out bytes.Buffer

	pauseOnStartupFailure(false, in, &out)

	if !strings.Contains(out.String(), "Press Enter to exit") {
		t.Fatalf("interactive mode prompt = %q", out.String())
	}
}
