package interp

import (
	"bufio"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/tools/txtar"
)

func TestInterp(t *testing.T) {
	testdata := "testdata"
	entries, err := os.ReadDir(testdata)
	if os.IsNotExist(err) {
		t.Skipf("testdata directory %q not found, skipping integration tests", testdata)
		return
	}
	if err != nil {
		t.Fatal(err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".txtar") {
			continue
		}

		t.Run(entry.Name(), func(t *testing.T) {
			path := filepath.Join(testdata, entry.Name())
			archive, err := txtar.ParseFile(path)
			if err != nil {
				t.Fatalf("failed to parse txtar file: %v", err)
			}
			var meta map[string]string

			var inputScript, expectedOut, expectedErr string
			for _, file := range archive.Files {
				switch file.Name {
				case "meta":
					meta = parseMetadata(file.Data)
				case "input":
					inputScript = string(file.Data)
				case "stdout":
					expectedOut = string(file.Data)
				case "stderr":
					expectedErr = string(file.Data)
				}
			}
			afs, err := txtar.FS(archive)
			if err != nil {
				t.Fatal(err)
			}

			if inputScript == "" {
				t.Fatal("missing 'input' file in txtar archive")
			}

			var stdout bytes.Buffer
			var stderr bytes.Buffer
			errOut := &stdout
			errFmt := func(err error) string {
				return "ERR: " + err.Error()
			}
			if expectedErr != "" {
				errOut = &stderr
				errFmt = func(err error) string {
					return err.Error()
				}
			}

			r := strings.NewReader(inputScript)
			s := bufio.NewScanner(r)
			cli := NewCmdInterp(s, nil, WithStdout(&stdout), WithStderr(errOut), WithErrFormatter(errFmt))

			cli.Open = func(filename string) (io.ReadCloser, error) {
				f, err := afs.Open(filename)
				if err != nil {
					return nil, err
				}
				return f, nil
			}
			runError := cli.Process()

			if meta["mustFail"] == "true" {
				if runError == nil {
					t.Errorf("expected script execution to fail, but it succeeded")
				}
			} else {
				if runError != nil {
					t.Errorf("unexpected execution error: %v", runError)
				}
			}

			gotOut := stdout.String()
			if gotOut != expectedOut {
				t.Errorf("stdout mismatch:\ngot:\n%s\nwant:\n%s", gotOut, expectedOut)
			}

			if expectedErr != "" {
				gotErr := stderr.String()
				if !strings.Contains(gotErr, strings.TrimSpace(expectedErr)) {
					t.Errorf("stderr mismatch:\ngot:\n%s\nwant:\n%s", gotErr, expectedErr)
				}
			}
		})
	}
}

func parseMetadata(comment []byte) map[string]string {
	meta := make(map[string]string)
	lines := strings.SplitSeq(string(comment), "\n")
	for line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 {
			meta[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
	}
	return meta
}
