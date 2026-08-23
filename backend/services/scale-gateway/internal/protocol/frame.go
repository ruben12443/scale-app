// Package protocol implements the wire framing and message codecs for Bizerba's
// Dialog serial protocol (Dialog 02 and Dialog 04 variants), as spoken over a raw
// TCP socket to a serial-to-Ethernet adapter attached to the scale.
package protocol

import "fmt"

const (
	// STX marks the start of a frame. It carries no information of its own (its
	// value is always constant), so it is excluded from the BCC checksum.
	STX byte = 0x02
	// ETX marks the end of a frame's payload. It is included in the BCC checksum
	// so that a transmission fault which truncates a frame before ETX is caught
	// immediately, rather than being silently accepted as a short valid frame.
	ETX byte = 0x03
)

// ComputeBCC computes the block check character for a payload: the XOR of every
// payload byte followed by XOR with ETX. STX is intentionally excluded.
func ComputeBCC(payload []byte) byte {
	var bcc byte
	for _, b := range payload {
		bcc ^= b
	}
	bcc ^= ETX
	return bcc
}

// BuildFrame wraps payload as STX + payload + ETX + BCC.
func BuildFrame(payload []byte) []byte {
	frame := make([]byte, 0, len(payload)+3)
	frame = append(frame, STX)
	frame = append(frame, payload...)
	frame = append(frame, ETX)
	frame = append(frame, ComputeBCC(payload))
	return frame
}

// ParseFrame validates a complete frame (STX + payload + ETX + BCC) and returns
// the payload bytes between STX and ETX. It returns an error if the framing
// bytes are missing/misplaced or the BCC does not match.
func ParseFrame(frame []byte) ([]byte, error) {
	const minLen = 3 // STX + ETX + BCC, empty payload
	if len(frame) < minLen {
		return nil, fmt.Errorf("protocol: frame too short: got %d bytes, want at least %d", len(frame), minLen)
	}
	if frame[0] != STX {
		return nil, fmt.Errorf("protocol: frame does not start with STX: got 0x%02X", frame[0])
	}
	etxIndex := len(frame) - 2
	if frame[etxIndex] != ETX {
		return nil, fmt.Errorf("protocol: frame missing ETX at expected position %d: got 0x%02X", etxIndex, frame[etxIndex])
	}
	payload := frame[1:etxIndex]
	wantBCC := frame[len(frame)-1]
	gotBCC := ComputeBCC(payload)
	if gotBCC != wantBCC {
		return nil, fmt.Errorf("protocol: BCC mismatch: frame declares 0x%02X, computed 0x%02X", wantBCC, gotBCC)
	}
	return payload, nil
}
