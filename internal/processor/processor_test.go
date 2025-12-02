package processor

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	require.NoError(t, err)

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".go") {
			continue
		}
		t.Run(name, func(t *testing.T) {
			beforePath := filepath.Join(testDir, name)
			shortName := strings.TrimSuffix(name, ".go")
			afterPath := filepath.Join(testDir, fmt.Sprintf("%s.after", shortName))
			reasonsPath := filepath.Join(testDir, fmt.Sprintf("%s.reasons", shortName))

			expectedReasons := getReasons(t, reasonsPath)

			actualContent, actualReasons := processFile(t, beforePath)

			afterContent, err := os.ReadFile(afterPath)
			require.NoError(t, err)

			assert.Equal(t, string(afterContent), string(actualContent))

			assert.Equal(t, expectedReasons, actualReasons)
		})
	}
}

func getReasons(t *testing.T, filename string) []string {
	data, err := os.ReadFile(filename)
	if err != nil && errors.Is(err, os.ErrNotExist) {
		return nil
	}
	require.NoError(t, err)

	return strings.Split(string(data), "\n")
}

func processFile(t *testing.T, filename string) ([]byte, []string) {
	beforeContent, err := os.ReadFile(filename)
	require.NoError(t, err)

	backupFile := filename + ".bak"
	err = os.WriteFile(backupFile, beforeContent, 0644)
	require.NoError(t, err)

	defer func() {
		// Restore original file after test
		os.Rename(backupFile, filename)
	}()

	processedContent, err := Process(filename)
	require.NoError(t, err)

	return processedContent.Content, processedContent.FailedToFixReasons

}
