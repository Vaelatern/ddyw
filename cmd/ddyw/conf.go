package main

import (
	"fmt"
	"io"
	"io/fs"

	"github.com/BurntSushi/toml"
	"gopkg.in/yaml.v3"
)

// DeepMergeFiles loads TOML and YAML files from fs.File objects into maps and deep merges them.
func DeepMergeFiles(files []fs.File) (interface{}, error) {
	var result map[string]interface{}

	for _, file := range files {
		filestat, _ := file.Stat()
		filename := filestat.Name()
		// Read file content
		data, err := io.ReadAll(file)
		if err != nil {
			return nil, fmt.Errorf("failed to read file %s: %w", filename, err)
		}

		// Try parsing as TOML
		var tomlData map[string]interface{}
		if err := toml.Unmarshal(data, &tomlData); err == nil {
			result = deepMergeMaps(tomlData, result)
			continue
		}

		// Try parsing as YAML
		var yamlData map[string]interface{}
		if err := yaml.Unmarshal(data, &yamlData); err == nil {
			result = deepMergeMaps(yamlData, result)
			continue
		}

		// If neither TOML nor YAML parsing succeeded
		return nil, fmt.Errorf("file %s is neither valid TOML nor YAML", filename)
	}

	return result, nil
}

// deepMergeMaps merges two maps recursively, with values from source overriding target.
func deepMergeMaps(source map[string]interface{}, target map[string]interface{}) map[string]interface{} {
	if source == nil {
		return target
	}
	merged := make(map[string]interface{})

	// Copy target to merged
	for k, v := range target {
		merged[k] = v
	}

	// Merge source into merged
	for k, v := range source {
		if targetVal, exists := merged[k]; exists {
			// If both values are maps, merge them recursively
			if targetMap, ok := targetVal.(map[string]interface{}); ok {
				if sourceMap, ok := v.(map[string]interface{}); ok {
					merged[k] = deepMergeMaps(sourceMap, targetMap)
					continue
				}
			}
		}
		// Otherwise, override with source value
		merged[k] = v
	}

	return merged
}
