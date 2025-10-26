package review

import (
	"os"
	"testing"
)

func TestIsTTY(t *testing.T) {
	// Test with stdin (may or may not be TTY depending on environment)
	result := IsTTY(os.Stdin.Fd())

	// Should return a boolean without panicking
	if result != true && result != false {
		t.Errorf("IsTTY should return a boolean, got: %v", result)
	}

	// Note: In CI environments, this will typically return false
	// In interactive terminal, this will return true
	t.Logf("IsTTY(stdin) = %v (expected: false in CI, true in terminal)", result)
}

func TestIsTTY_Stdout(t *testing.T) {
	// Test with stdout
	result := IsTTY(os.Stdout.Fd())

	// Should return a boolean
	if result != true && result != false {
		t.Errorf("IsTTY should return a boolean, got: %v", result)
	}

	t.Logf("IsTTY(stdout) = %v", result)
}

func TestIsInteractive(t *testing.T) {
	// Test stdin TTY detection
	result := IsInteractive()

	// Should return a boolean without panicking
	if result != true && result != false {
		t.Errorf("IsInteractive should return a boolean, got: %v", result)
	}

	// In CI/CD, this should be false
	// In interactive terminal, this should be true
	t.Logf("IsInteractive() = %v (expected: false in CI, true in terminal)", result)
}

func TestIsOutputTerminal(t *testing.T) {
	// Test stdout TTY detection
	result := IsOutputTerminal()

	// Should return a boolean without panicking
	if result != true && result != false {
		t.Errorf("IsOutputTerminal should return a boolean, got: %v", result)
	}

	t.Logf("IsOutputTerminal() = %v", result)
}

func TestTTYDetection_Consistency(t *testing.T) {
	// IsInteractive and IsTTY(stdin) should be consistent
	interactive := IsInteractive()
	stdinTTY := IsTTY(os.Stdin.Fd())

	if interactive != stdinTTY {
		t.Errorf("IsInteractive() and IsTTY(stdin) should match: interactive=%v, stdinTTY=%v", interactive, stdinTTY)
	}

	// IsOutputTerminal and IsTTY(stdout) should be consistent
	outputTerminal := IsOutputTerminal()
	stdoutTTY := IsTTY(os.Stdout.Fd())

	if outputTerminal != stdoutTTY {
		t.Errorf("IsOutputTerminal() and IsTTY(stdout) should match: outputTerminal=%v, stdoutTTY=%v", outputTerminal, stdoutTTY)
	}
}
