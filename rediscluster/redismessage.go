package rediscluster

import (
	"fmt"
	"strings"
)

var (
	EOL = []byte("\r\n")
)

type RedisMessage struct {
	Message [][2][]byte
}

func NewRedisMessage() *RedisMessage {
	rm := RedisMessage{
		Message: *new([][2][]byte),
	}
	return &rm
}

func (rm *RedisMessage) String() string {
	return string(rm.Bytes())
}

func (rm *RedisMessage) Bytes() []byte {
	if rm == nil {
		return nil
	}
	output := make([]byte, rm.Length())
	i := 0
	for _, vals := range rm.Message {
		for _, val := range vals {
			for j, v := range val {
				output[i+j] = v
			}
			i += len(val)
		}
	}
	return output
}

func (rm *RedisMessage) Key() string {
	if len(rm.Message) < 3 {
		return ""
	}
	return string(rm.Message[2][1])
}

func (rm *RedisMessage) Command() string {
	if len(rm.Message) < 2 {
		return ""
	}
	return strings.ToUpper(strings.TrimSpace(string(rm.Message[1][1])))
}

func (rm *RedisMessage) Length() int {
	i := 0
	if rm == nil {
		return 0
	}
	for _, vals := range rm.Message {
		for _, val := range vals {
			i += len(val)
		}
	}
	return i
}

func MessageFromString(input string) *RedisMessage {
	message := RedisMessage{}
	if input[0] == '+' || input[0] == '-' {
		message.Message = make([][2][]byte, 1)
		message.Message[0][0] = []byte(input)
	} else {
		parts := strings.Fields(input)
		message.Message = make([][2][]byte, len(parts)+1)

		message.Message[0] = [2][]byte{
			[]byte(fmt.Sprintf("*%d\r\n", len(parts))),
			nil,
		}
		for i, comp := range parts {
			message.Message[i+1] = [2][]byte{
				[]byte(fmt.Sprintf("$%d\r\n", len(comp))),
				[]byte(fmt.Sprintf("%s\r\n", comp)),
			}
		}
	}

	return &message
}
