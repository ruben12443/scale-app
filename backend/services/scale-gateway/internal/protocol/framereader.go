package protocol

import (
	"fmt"
	"io"
)

// MaxFrameLen bounds how many payload bytes ReadFrame will accept between
// STX and ETX before giving up, so a stuck connection that never sends ETX
// can't grow the buffer unbounded.
const MaxFrameLen = 256

// ReadFrame reads one complete STX...ETX+BCC frame from r, discarding any
// stray bytes before STX. It does not validate the BCC; pass the result to
// a Codec, which does. Shared by both driver (the real client) and any
// server-side simulator, so wire framing is defined in exactly one place.
func ReadFrame(r io.Reader) ([]byte, error) {
	frame := make([]byte, 0, 32)
	one := make([]byte, 1)

	for {
		if _, err := io.ReadFull(r, one); err != nil {
			return nil, fmt.Errorf("protocol: read looking for STX: %w", err)
		}
		if one[0] == STX {
			frame = append(frame, one[0])
			break
		}
	}

	for {
		if _, err := io.ReadFull(r, one); err != nil {
			return nil, fmt.Errorf("protocol: read frame body: %w", err)
		}
		frame = append(frame, one[0])
		if one[0] == ETX {
			break
		}
		if len(frame) > MaxFrameLen {
			return nil, fmt.Errorf("protocol: frame exceeded %d bytes without ETX", MaxFrameLen)
		}
	}

	if _, err := io.ReadFull(r, one); err != nil {
		return nil, fmt.Errorf("protocol: read BCC byte: %w", err)
	}
	frame = append(frame, one[0])

	return frame, nil
}
