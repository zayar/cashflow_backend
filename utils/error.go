package utils

import "errors"

var ErrorRecordNotFound = errors.New("record not found")

func ErrorPanic(err error) {
	if err != nil {
		panic(err)
	}
}
