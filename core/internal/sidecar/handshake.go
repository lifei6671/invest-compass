package sidecar

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
)

const ProtocolVersion = "1"

var ErrInvalidHandshake = errors.New("invalid sidecar handshake")

type Handshake struct {
	Token           string `json:"token"`
	ProtocolVersion string `json:"protocolVersion"`
}

type ReadyMessage struct {
	Status string `json:"status"`
	Port   int    `json:"port"`
	PID    int    `json:"pid"`
}

// ReadHandshake 从 stdin 读取单行 JSON 握手，并受 context 超时控制。
func ReadHandshake(ctx context.Context, reader io.Reader) (Handshake, error) {
	type result struct {
		line string
		err  error
	}

	results := make(chan result, 1)
	go func() {
		line, err := bufio.NewReader(reader).ReadString('\n')
		results <- result{
			line: line,
			err:  err,
		}
	}()

	select {
	case <-ctx.Done():
		return Handshake{}, ctx.Err()
	case result := <-results:
		if result.err != nil && !errors.Is(result.err, io.EOF) {
			return Handshake{}, ErrInvalidHandshake
		}
		return parseHandshake(result.line)
	}
}

// parseHandshake 校验握手内容，确保 token 非空且协议版本与当前 core 兼容。
func parseHandshake(line string) (Handshake, error) {
	var handshake Handshake
	if err := json.Unmarshal([]byte(strings.TrimSpace(line)), &handshake); err != nil {
		return Handshake{}, ErrInvalidHandshake
	}
	if handshake.Token == "" || handshake.ProtocolVersion != ProtocolVersion {
		return Handshake{}, ErrInvalidHandshake
	}
	return handshake, nil
}
