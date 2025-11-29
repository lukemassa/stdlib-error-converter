package processor

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testDir = "testdata"

func fileExists(filename string) (bool, error) {
	_, err := os.Stat(filename)
	if err == nil {
		return true, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return false, err
	}
	return false, nil
}

func TestProcessFile(t *testing.T) {

	entries, err := os.ReadDir(testDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".go") {
			continue
		}
		beforePath := filepath.Join(testDir, name)
		shortName := strings.TrimSuffix(name, ".go")
		afterPath := filepath.Join(testDir, fmt.Sprintf("%s.after", shortName))
		errorPath := filepath.Join(testDir, fmt.Sprintf("%s.error", shortName))
		hasAfter, err := fileExists(afterPath)
		if err != nil {
			t.Fatal(err)
		}
		hasError, err := fileExists(errorPath)
		if err != nil {
			t.Fatal(err)
		}
		expectErr := false
		expectedPath := ""
		if hasAfter {
			if hasError {
				t.Fatalf("Cannot have both after and error file for %s", shortName)
			} else {
				// After path is set, test against that
				expectErr = false
				expectedPath = afterPath
			}
		} else {
			if hasError {
				// Error path is set, test against that
				expectErr = true
				expectedPath = errorPath
			} else {
				// Neither paths are set, we expect the file to be unchanged
				expectErr = false
				expectedPath = beforePath
			}

		}
		t.Run(name, func(t *testing.T) {
			testOneFile(t, beforePath, expectedPath, expectErr)
		})

	}

}

func testOneFile(t *testing.T, currentFile, expectedPath string, expectErr bool) {
	if !strings.Contains(currentFile, "wrapWithOneArg") {
		return
	}
	beforeContent, err := os.ReadFile(currentFile)
	if err != nil {
		t.Fatalf("failed to read current file: %v", err)
	}

	backupFile := currentFile + ".bak"
	if err := os.WriteFile(backupFile, beforeContent, 0644); err != nil {
		t.Fatalf("failed to create backup file: %v", err)
	}
	defer func() {
		// Restore original file after test
		os.Rename(backupFile, currentFile)
	}()

	expectedContent, err := os.ReadFile(expectedPath)
	if err != nil {
		t.Fatalf("failed to read after file: %v", err)
	}

	processedContent, err := Process(currentFile, false)
	if expectErr {
		if err == nil {
			t.Errorf("no error occurred, expected:\n%s", expectedContent)
			return
		}
		if err.Error() != string(expectedContent) {
			t.Errorf("processed file does not match expected output\nExpected:\n%s\nGot:\n%s", expectedContent, err.Error())
		}
		return
	}
	if err != nil {
		t.Errorf("processFile failed: %v", err)
	}

	if string(processedContent) != string(expectedContent) {
		t.Errorf("processed file does not match expected output\nExpected:\n%s\nGot:\n%s", expectedContent, processedContent)
	}
}
