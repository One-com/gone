package jconf

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
)

// SyntaxError is an extension of encoding/json.SyntaxError but with an error text
// with a better description of the position in the input data (marker and line)
type SyntaxError struct {
	Cause *json.SyntaxError
	help  string
}

func (e *SyntaxError) Error() string { return e.help }

/*
 ParseInto loads a JSON stream into a destination object, which is a datatype
 defined by the caller.
 It will ignore values in the file it can't fit into dst. On a parsing error, it
 returns an error with the line and the location in the input data.

 The input JSON data stream is allowed to contain line comments in the C++ style
 where // outside a JSON string object denotes that the rest of the line is a comment.
*/
func ParseInto(source io.Reader, dest interface{}) (err error) {

	var data []byte
	data, err = ioutil.ReadAll(source)
	if err != nil {
		return
	}

	filterComments(data)

	err = json.Unmarshal(data, &dest)
	if err != nil {
		if syntax, ok := err.(*json.SyntaxError); ok {
			err = fmtSyntaxError(data, syntax)
		} else {
			err = fmt.Errorf("Parse error: %s", err.Error())
		}
	}
	return
}

func filterComments(data []byte) {

	var index int
	var curChar byte

	// Parser state
	var inString bool = false
	var inCommentSingleLine bool = false

	var size = len(data)

	for index = 0; index < size; index++ {
		curChar = data[index]

		// Check if any inString whould be stopped before we just take next char
		// it's either closing or opening inString and it is not escaped
		if !inCommentSingleLine && curChar == '"' && index >= 1 && data[index-1] != '\\' {
			// We met a " which is not escaped. Either start a string or stop it
			inString = !inString
		}

		if inString || index == 0 {
			continue
		}

		// If single line comment flag is on, then check for end of it
		if inCommentSingleLine && curChar == '\n' {
			inCommentSingleLine = false
		} else if curChar == '/' && data[index-1] == '/' {
			// if we have a start on single-line comment
			inCommentSingleLine = true
			data[index] = ' '
			data[index-1] = ' '
		} else if inCommentSingleLine {
			// If this character is in a comment, blank it out
			data[index] = ' '
		}
	}
}

// Find out where a Syntax Error occurred in the JSON string
func fmtSyntaxError(js []byte, syntax *json.SyntaxError) error {

	start := bytes.LastIndex(js[:syntax.Offset], []byte{'\n'}) + 1

	line := bytes.Count(js[:start], []byte{'\n'}) + 1

	help := string(js[start:syntax.Offset]) + "<---"

	err := &SyntaxError{
		Cause: syntax,
		help: fmt.Sprintf("Parse error: %s (byte=%d line=%d): %s",
			syntax.Error(), syntax.Offset, line, help),
	}

	return err
}
