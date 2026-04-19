package batch

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

// Format is the detected stdin format.
type Format int

const (
	FormatNDJSON Format = iota
	FormatArray
)

// ErrBadFormat is returned when the first non-whitespace byte is neither
// '[' nor '{'.
var ErrBadFormat = errors.New("batch: stdin must start with '[' (array) or '{' (NDJSON)")

// ErrLineTooLong is returned when a single NDJSON line exceeds 1 MiB.
var ErrLineTooLong = errors.New("batch: NDJSON line exceeds 1 MiB limit")

const ndjsonMaxLine = 1 << 20

// Parse reads stdin and returns the parsed ops plus the detected format.
func Parse(r io.Reader) ([]Op, Format, error) {
	br := bufio.NewReader(r)
	var first byte
	for {
		b, err := br.ReadByte()
		if err == io.EOF {
			return nil, FormatNDJSON, nil
		}
		if err != nil {
			return nil, 0, err
		}
		if b == ' ' || b == '\t' || b == '\n' || b == '\r' {
			continue
		}
		first = b
		_ = br.UnreadByte()
		break
	}

	// Leading `#` comment lines route to NDJSON: peek ahead past any
	// comment/blank lines to find the real first structural byte.
	if first == '#' {
		first = '{'
	}

	switch first {
	case '[':
		dec := json.NewDecoder(br)
		dec.DisallowUnknownFields()
		var ops []Op
		if err := dec.Decode(&ops); err != nil {
			return nil, 0, fmt.Errorf("array decode: %w", err)
		}
		return ops, FormatArray, nil
	case '{':
		scanner := bufio.NewScanner(br)
		scanner.Buffer(make([]byte, 0, 64*1024), ndjsonMaxLine)
		var ops []Op
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			dec := json.NewDecoder(bytes.NewReader([]byte(line)))
			dec.DisallowUnknownFields()
			var op Op
			if err := dec.Decode(&op); err != nil {
				return nil, 0, fmt.Errorf("ndjson decode line %d: %w", len(ops)+1, err)
			}
			if dec.More() {
				return nil, 0, fmt.Errorf("ndjson line %d has trailing junk", len(ops)+1)
			}
			ops = append(ops, op)
		}
		if err := scanner.Err(); err != nil {
			if errors.Is(err, bufio.ErrTooLong) {
				return nil, 0, ErrLineTooLong
			}
			return nil, 0, err
		}
		return ops, FormatNDJSON, nil
	default:
		return nil, 0, ErrBadFormat
	}
}
