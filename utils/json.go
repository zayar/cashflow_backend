package utils

import (
	"encoding/json"
	"fmt"
	"os"
)

// Marshal generic struct to JSON
func MarshalToJSON[T any](input T) (string, error) {
	jsonData, err := json.Marshal(input)
	if err != nil {
		return "", err
	}
	return string(jsonData), nil
}

// Unmarshal JSON to generic struct
func UnmarshalFromJSON[T any](data []byte, output *T) error {
	return json.Unmarshal(data, output)
}

func MarshalToPrint[T any](input T) {
	// Marshal the input struct to pretty JSON
	jsonData, err := json.MarshalIndent(input, "", "  ")
	if err != nil {
		fmt.Println("Error marshaling to JSON:", err)

	}
	file, err := os.Create("out.json")
	if err != nil {
		fmt.Printf("error creating file: %v", err)
	}
	defer file.Close()

	// Write the JSON data to the file
	_, err = file.Write(jsonData)
	if err != nil {
		fmt.Printf("error writing to file: %v", err)
	}
}
