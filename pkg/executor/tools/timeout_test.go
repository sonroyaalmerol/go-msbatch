package tools

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/sonroyaalmerol/go-msbatch/pkg/processor"
)

// Real cmd writes a backspace before each countdown digit; /t 1 emits exactly "\b0".
func TestTimeoutCountdownBytes(t *testing.T) {
	var buf bytes.Buffer
	env := processor.NewEnvironment(true)
	proc := processor.New(env, []string{"test.bat"}, nil)
	proc.Stdout = &buf
	proc.Stdin = strings.NewReader("")

	start := time.Now()
	if err := Timeout(proc, cmd("timeout", "/t", "1")); err != nil {
		t.Fatalf("Timeout returned error: %v", err)
	}
	if elapsed := time.Since(start); elapsed < 900*time.Millisecond {
		t.Fatalf("timeout returned after %v, want at least a 1s countdown", elapsed)
	}

	want := "Waiting for 1 seconds, press a key to continue ...\b0\n"
	if got := buf.String(); got != want {
		t.Errorf("timeout output = %q, want %q", got, want)
	}
}
