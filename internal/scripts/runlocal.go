package scripts

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func OutFileName(fileName string, t time.Time, suffix string) string {
	return t.Format("2006-01-02T15:04:05Z07:00") +
		"-" +
		strings.TrimSuffix(filepath.Base(fileName), ".json.in") +
		suffix
}

// WriteOutFile, given a file, will go to the file's directory, create
// the directory inside it `output/` if it doesn't exist, and then write
// the file's name and put the resulting text in that file.
func WriteOutFile(dir string, fileName string, t time.Time, suffix string, outBytes []byte) error {
	// Create output directory path
	outputDir := filepath.Join(dir, "output")

	// Create output directory if it doesn't exist
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return err
	}

	// Create error file path
	outFileName := OutFileName(fileName, t, suffix)
	errorFileName := filepath.Join(outputDir, outFileName)

	// Write error message to file
	if err := os.WriteFile(errorFileName, outBytes, 0644); err != nil {
		return err
	}
	return nil
}

func RunLocalViaJson[Args any](config Args, scriptCtx ResolutionContext, dir string, source fs.File) error {
	info, err := source.Stat()
	if err != nil {
		return err
	}

	// Get the directory of the input file
	name := filepath.Base(info.Name())

	// Get source contents as json unmarshalled to interface{}
	sourceContents, err := io.ReadAll(source)
	if err != nil {
		return err
	}
	var data interface{}
	if err := json.Unmarshal(sourceContents, &data); err != nil {
		return err
	}

	// Write the input file as an output file
	err = WriteOutFile(dir, info.Name(), time.Now(), ".json.in", sourceContents)
	if err != nil {
		return err
	}

	cleanedName := strings.TrimSuffix(name, ".json.in")
	// Actually Execute The Script
	file := scriptCtx.Resolve(cleanedName)
	if file == nil {
		return fmt.Errorf("Failed to resolve %s", cleanedName)
	}
	defer file.Close()
	returnData, err := RunViaJson[interface{}](context.Background(), config, data, file)

	if err != nil {
		err = WriteOutFile(dir, info.Name(), time.Now(), ".run.error", []byte(err.Error()))
		return err
	}
	// Re-marshall returnData to bytes and call
	outData, err := json.Marshal(returnData)
	if err != nil {
		return err
	}

	err = WriteOutFile(dir, info.Name(), time.Now(), ".json.out", outData)
	return err
}
