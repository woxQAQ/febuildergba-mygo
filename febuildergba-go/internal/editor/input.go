package editor

import (
	"errors"
	"strconv"
	"strings"

	"github.com/woxQAQ/febuildergba-mygo/internal/rom"
)

func ParseInteger(text string) (int64, error) {
	text = strings.TrimSpace(text)
	base := 10
	if strings.HasPrefix(strings.ToLower(text), "0x") {
		base = 0
	}
	return strconv.ParseInt(text, base, 64)
}

func ParseAddress(text string) (int, error) {
	text = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(text)), "0x")
	value, err := strconv.ParseUint(text, 16, 32)
	if err != nil {
		return 0, errors.New("invalid_address")
	}
	if value >= 0x08000000 && value < 0x0A000000 {
		value -= 0x08000000
	}
	if value >= rom.MaxSize {
		return 0, errors.New("invalid_address")
	}
	return int(value), nil
}
