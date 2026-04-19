package sku

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/sofq/sku/internal/batch"
	skuerrors "github.com/sofq/sku/internal/errors"
)

func TestBatch_arrayHappyPath(t *testing.T) {
	t.Setenv("SKU_DATA_DIR", t.TempDir())

	in := `[
	  {"command":"aws ec2 price","args":{"instance_type":"m5.large","region":"us-east-1"}},
	  {"command":"llm price","args":{"model":"anthropic/claude-opus-4.6"}}
	]`
	stdout, stderr, exit := runBatch(t, strings.NewReader(in))
	if exit != skuerrors.CodeNotFound.ExitCode() {
		t.Fatalf("exit = %d, want %d\nstdout=%s\nstderr=%s", exit, skuerrors.CodeNotFound.ExitCode(), stdout.String(), stderr.String())
	}
	var recs []map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &recs); err != nil {
		t.Fatalf("stdout not a JSON array: %v\n%s", err, stdout.String())
	}
	if len(recs) != 2 {
		t.Fatalf("want 2 records, got %d", len(recs))
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr should be empty on per-op errors, got: %s", stderr.String())
	}
}

func TestBatch_ndjsonFiftyOps(t *testing.T) {
	t.Setenv("SKU_DATA_DIR", t.TempDir())
	var b strings.Builder
	for range 50 {
		b.WriteString(`{"command":"aws ec2 price","args":{"instance_type":"m5.large","region":"us-east-1"}}` + "\n")
	}
	stdout, _, exit := runBatch(t, strings.NewReader(b.String()))
	if exit != skuerrors.CodeNotFound.ExitCode() {
		t.Fatalf("aggregate exit = %d, want %d", exit, skuerrors.CodeNotFound.ExitCode())
	}
	lines := bytes.Count(stdout.Bytes(), []byte("\n"))
	if lines != 50 {
		t.Fatalf("want 50 NDJSON output lines, got %d", lines)
	}
}

func TestBatch_badFirstByteReturnsFour(t *testing.T) {
	stdout, stderr, exit := runBatch(t, strings.NewReader("not-json"))
	if exit != skuerrors.CodeValidation.ExitCode() {
		t.Fatalf("exit = %d, want 4", exit)
	}
	if !strings.Contains(stderr.String(), `"code":"validation"`) {
		t.Fatalf("stderr lacks validation envelope: %s", stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout should be empty on parse failure, got: %s", stdout.String())
	}
}

func TestBatch_unknownCommand(t *testing.T) {
	stdout, _, _ := runBatch(t, strings.NewReader(`[{"command":"nope"}]`))
	var recs []map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &recs); err != nil {
		t.Fatalf("%v\n%s", err, stdout.String())
	}
	if recs[0]["error"] == nil {
		t.Fatal("expected error on unknown command")
	}
	errMap := recs[0]["error"].(map[string]any)
	if errMap["code"] != "validation" {
		t.Fatalf("code = %v, want validation", errMap["code"])
	}
}

func runBatch(t *testing.T, in io.Reader) (stdout, stderr *bytes.Buffer, exit int) {
	t.Helper()
	root := newRootCmd()
	stdout = &bytes.Buffer{}
	stderr = &bytes.Buffer{}
	root.SetOut(stdout)
	root.SetErr(stderr)
	root.SetIn(in)
	root.SetArgs([]string{"batch"})
	err := root.Execute()
	if err == nil {
		return stdout, stderr, 0
	}
	if errors.Is(err, batch.ErrAggregate) {
		var e *skuerrors.E
		if errors.As(err, &e) {
			return stdout, stderr, e.Code.ExitCode()
		}
		return stdout, stderr, 1
	}
	return stdout, stderr, skuerrors.Write(stderr, err)
}
