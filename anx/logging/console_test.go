package logging

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/go-logr/logr"
)

func TestConsoleSinkStructuredOutput(t *testing.T) {
	originalTimeNow := timeNow
	timeNow = func() time.Time {
		return time.Date(2026, time.October, 20, 16, 36, 28, 27_000_000, time.FixedZone("CEST", 2*60*60))
	}
	defer func() { timeNow = originalTimeNow }()

	var output bytes.Buffer
	state := &consoleState{output: &output}
	state.verbosity.Store(2)
	logger := logr.New(&consoleSink{state: state}).WithName("controller").WithValues("namespace", "default")

	logger.Info("Adding finalizer to service", "service", "my-service")
	logger.V(2).Info("Reconciliation details", "attempt", 3)
	logger.Error(errors.New("boom"), "Reconciliation failed", "service", "my-service")

	lines := strings.Split(strings.TrimSpace(output.String()), "\n")
	if len(lines) != 3 {
		t.Fatalf("got %d log lines, want 3:\n%s", len(lines), output.String())
	}

	assertContains(t, lines[0], "2026-10-20T16:36:28.027+02:00  INFO")
	assertContains(t, lines[0], "Adding finalizer to service  | logger=\"controller\" namespace=\"default\" service=\"my-service\"")
	assertContains(t, lines[1], " DEBUG ")
	assertContains(t, lines[1], " | logger=\"controller\" namespace=\"default\" v=2 attempt=3")
	assertContains(t, lines[2], " ERROR ")
	assertContains(t, lines[2], "error=\"boom\" service=\"my-service\"")
}

func TestConsoleSinkPreservesLegacyKlogSeverity(t *testing.T) {
	originalTimeNow := timeNow
	timeNow = func() time.Time {
		return time.Date(2026, time.October, 20, 16, 36, 28, 27_000_000, time.UTC)
	}
	defer func() { timeNow = originalTimeNow }()

	var output bytes.Buffer
	sink := &consoleSink{state: &consoleState{output: &output}}
	sink.WriteKlogBuffer([]byte("W1020 16:36:28.027717   36871 controller.go:944] Adding finalizer to service default/my-service\n"))

	want := "2026-10-20T16:36:28.027Z  WARN   Adding finalizer to service default/my-service\n"
	if got := output.String(); got != want {
		t.Fatalf("unexpected output:\n got: %q\nwant: %q", got, want)
	}
}

func TestConsoleSinkVerbosity(t *testing.T) {
	state := &consoleState{}
	state.verbosity.Store(2)
	sink := &consoleSink{state: state}

	if !sink.Enabled(0) || !sink.Enabled(2) || sink.Enabled(3) || sink.Enabled(-1) {
		t.Fatal("unexpected verbosity filtering")
	}
}

func assertContains(t *testing.T, value, substring string) {
	t.Helper()
	if !strings.Contains(value, substring) {
		t.Fatalf("%q does not contain %q", value, substring)
	}
}
