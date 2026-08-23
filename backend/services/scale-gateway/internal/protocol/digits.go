package protocol

import (
	"fmt"
	"strconv"
	"strings"
)

// formatDigits renders value as a fixed-width, zero-padded ASCII digit string.
// It returns an error if value is negative or does not fit in width digits.
func formatDigits(value, width int) (string, error) {
	if value < 0 {
		return "", fmt.Errorf("protocol: value %d is negative, cannot encode as digits", value)
	}
	s := strconv.Itoa(value)
	if len(s) > width {
		return "", fmt.Errorf("protocol: value %d does not fit in %d digits", value, width)
	}
	return strings.Repeat("0", width-len(s)) + s, nil
}

// parseDigits parses a fixed-width ASCII digit substring into an int.
func parseDigits(field string) (int, error) {
	for _, r := range field {
		if r < '0' || r > '9' {
			return 0, fmt.Errorf("protocol: field %q contains a non-digit character", field)
		}
	}
	return strconv.Atoi(field)
}

// splitFields cuts payload into consecutive fields of the given widths. It
// returns an error if payload's length does not exactly match the sum of widths.
func splitFields(payload []byte, widths ...int) ([]string, error) {
	total := 0
	for _, w := range widths {
		total += w
	}
	if len(payload) != total {
		return nil, fmt.Errorf("protocol: payload length %d does not match expected total field width %d", len(payload), total)
	}
	fields := make([]string, len(widths))
	offset := 0
	for i, w := range widths {
		fields[i] = string(payload[offset : offset+w])
		offset += w
	}
	return fields, nil
}
