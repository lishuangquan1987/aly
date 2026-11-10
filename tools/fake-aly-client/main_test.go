package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testScenario = `{
  "name": "t",
  "commands": {
    "check_update": {
      "lines": [
        {"isSuccess": true, "errorMsg": "", "data": {"has_update": true}},
        {"isSuccess": true, "errorMsg": "", "data": null}
      ]
    },
    "apply_update": {
      "lines": [{"isSuccess": true, "errorMsg": "", "data": null}],
      "exit_code": 3
    }
  }
}`

func writeScenario(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "scenario.json")
	if err := os.WriteFile(path, []byte(testScenario), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestRunReplaysCompactLinesAndExitCode(t *testing.T) {
	path := writeScenario(t)
	var out, errOut bytes.Buffer
	code := run([]string{"check_update", "--close-timeout", "60"}, path, &out, &errOut)
	if code != 0 {
		t.Fatalf("exit = %d, stderr=%s", code, errOut.String())
	}
	want := "{\"isSuccess\":true,\"errorMsg\":\"\",\"data\":{\"has_update\":true}}\n" +
		"{\"isSuccess\":true,\"errorMsg\":\"\",\"data\":null}\n"
	if out.String() != want {
		t.Fatalf("stdout = %q, want %q", out.String(), want)
	}

	out.Reset()
	if code := run([]string{"apply_update", "--must-close-process-name", "X"}, path, &out, &errOut); code != 3 {
		t.Fatalf("apply exit = %d", code)
	}
}

func TestRunAcceptsScenarioFlagBeforeCommand(t *testing.T) {
	path := writeScenario(t)
	var out, errOut bytes.Buffer
	if code := run([]string{"-scenario", path, "check_update"}, "", &out, &errOut); code != 0 {
		t.Fatalf("exit = %d, stderr=%s", code, errOut.String())
	}
}

func TestRunRequiresScenarioAndKnownCommand(t *testing.T) {
	path := writeScenario(t)

	var out, errOut bytes.Buffer
	if code := run([]string{"check_update"}, "", &out, &errOut); code != 2 {
		t.Fatalf("missing scenario exit = %d", code)
	}
	if !strings.Contains(errOut.String(), "未指定场景") {
		t.Fatalf("stderr = %s", errOut.String())
	}

	errOut.Reset()
	if code := run([]string{"nope"}, path, &out, &errOut); code != 2 {
		t.Fatalf("unknown command exit = %d", code)
	}
	if !strings.Contains(errOut.String(), `"nope"`) {
		t.Fatalf("stderr = %s", errOut.String())
	}

	errOut.Reset()
	if code := run([]string{}, path, &out, &errOut); code != 2 {
		t.Fatalf("missing command exit = %d", code)
	}
}
