package models

import (
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
)

type PageInfo struct {
	StartCursor string `json:"startCursor"`
	EndCursor   string `json:"endCursor"`
	HasNextPage *bool  `json:"hasNextPage,omitempty"`
}

func DecodeCursor(cursor *string) (string, error) {
	decodedCursor := ""
	if cursor != nil {
		b, err := base64.StdEncoding.DecodeString(*cursor)
		if err != nil {
			return decodedCursor, err
		}
		decodedCursor = string(b)
	}
	return decodedCursor, nil
}

func DecodeCompositeCursor(cursor *string) (string, int) {
	if cursor == nil || *cursor == "" {
		return "", 0
	}

	decoded, err := base64.StdEncoding.DecodeString(*cursor)
	if err != nil {
		return "", 0
	}

	parts := strings.Split(string(decoded), "|")
	if len(parts) != 2 {
		return "", 0
	}

	id, err := strconv.Atoi(parts[1])
	if err != nil {
		return "", 0
	}

	return parts[0], id
}

func EncodeCursor(cursor string) string {
	return base64.StdEncoding.EncodeToString([]byte(cursor))
	// return cursor
}

func EncodeCompositeCursor(transactionDateTime string, id int) string {
	cursor := fmt.Sprintf("%s|%d", transactionDateTime, id)
	return base64.StdEncoding.EncodeToString([]byte(cursor))
}
