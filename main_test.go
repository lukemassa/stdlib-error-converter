package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProcessFile(t *testing.T) {
	testDir := "tests"

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
		if strings.HasSuffix(name, ".after.go") {
			continue
		}
		shortName := strings.TrimSuffix(name, ".go")
		afterName := fmt.Sprintf("%s.after.go", shortName)
		beforePath := filepath.Join(testDir, name)
		afterPath := filepath.Join(testDir, afterName)
		_, err := os.Stat(afterPath)
		if err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				t.Fatal(err)
			}
			afterPath = beforePath
		}
		testOneFile(t, beforePath, afterPath)
	}

}

func testOneFile(t *testing.T, beforeFile, afterFile string) {

	beforeContent, err := os.ReadFile(beforeFile)
	if err != nil {
		t.Fatalf("failed to read before file: %v", err)
	}

	backupFile := beforeFile + ".bak"
	if err := os.WriteFile(backupFile, beforeContent, 0644); err != nil {
		t.Fatalf("failed to create backup file: %v", err)
	}
	defer func() {
		// Restore original file after test
		os.Rename(backupFile, beforeFile)
	}()

	expectedContent, err := os.ReadFile(afterFile)
	if err != nil {
		t.Fatalf("failed to read after file: %v", err)
	}

	if err := processFile(beforeFile, false); err != nil {
		t.Fatalf("processFile failed: %v", err)
	}

	processedContent, err := os.ReadFile(beforeFile)
	if err != nil {
		t.Fatalf("failed to read processed file: %v", err)
	}

	if string(processedContent) != string(expectedContent) {
		t.Errorf("processed file does not match expected output\nExpected:\n%s\nGot:\n%s", expectedContent, processedContent)
	}
}
