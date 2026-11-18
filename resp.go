package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

type SimpleString string

func readCommand(reader *bufio.Reader) ([]string, error) {
	firstByte, err := reader.Peek(1)
	if err != nil { return nil, err }

	if firstByte[0] == '*' {
		return readRESPArray(reader)
	}

	line, err := reader.ReadString('\n')
	if err != nil { return nil, err }
	line = strings.TrimSpace(line)
	if line == "" { return nil, nil }
	return strings.Fields(line), nil
}

func readRESPArray(reader *bufio.Reader) ([]string, error) {
	line, err := reader.ReadString('\n')
	if err != nil { return nil, err }
	line = strings.TrimSpace(line)
	count, err := strconv.Atoi(line[1:])
	if err != nil { return nil, errors.New("invalid RESP array length") }

	parts := make([]string, 0, count)
	for i := 0; i < count; i++ {
		lenLine, err := reader.ReadString('\n')
		if err != nil { return nil, err }
		lenLine = strings.TrimSpace(lenLine)
		if len(lenLine) == 0 || lenLine[0] != '$' {
			return nil, errors.New("expected bulk string '$'")
		}
		strLen, err := strconv.Atoi(lenLine[1:])
		if err != nil { return nil, errors.New("invalid bulk string length") }

		buf := make([]byte, strLen)
		_, err = io.ReadFull(reader, buf)
		if err != nil { return nil, err }
		parts = append(parts, string(buf))
		reader.ReadString('\n') // Consume \r\n
	}
	return parts, nil
}

// writeResponse updated to support Array (for Lists and Hashes).
func writeResponse(conn io.Writer, value interface{}) error {
	var response string

	switch v := value.(type) {
	case nil:
		response = "$-1\r\n"
	case error:
		response = "-ERR " + v.Error() + "\r\n"
	case SimpleString:
		response = "+" + string(v) + "\r\n"
	case int:
		response = fmt.Sprintf(":%d\r\n", v)
	case string:
		response = fmt.Sprintf("$%d\r\n%s\r\n", len(v), v)
	case []string:
		// RESP Array of Bulk Strings (For LRANGE)
		response = fmt.Sprintf("*%d\r\n", len(v))
		for _, str := range v {
			response += fmt.Sprintf("$%d\r\n%s\r\n", len(str), str)
		}
	case map[string]string:
		// RESP Array of alternating key-value pairs (For HGETALL)
		response = fmt.Sprintf("*%d\r\n", len(v)*2)
		for k, val := range v {
			response += fmt.Sprintf("$%d\r\n%s\r\n", len(k), k)
			response += fmt.Sprintf("$%d\r\n%s\r\n", len(val), val)
		}
	default:
		response = "-ERR unknown response type\r\n"
	}

	_, err := conn.Write([]byte(response))
	return err
}