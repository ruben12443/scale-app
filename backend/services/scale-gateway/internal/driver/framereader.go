package driver

import (
	"fmt"
	"io"

	"scale-app/backend/services/scale-gateway/internal/protocol"
)

// maxFrameLen bounds how many payload bytes we'll accept between STX and ETX
// before giving up, so a stuck connection that never sends ETX can't grow
// this buffer unbounded.
const maxFrameLen = 256

// readFrame reads one complete STX...ETX+BCC frame from r, discarding any
// stray bytes before STX. It does not validate the BCC; callers pass the
// result to a protocol.Codec, which does.
func readFrame(r io.Reader) ([]byte, error) {
	frame := make([]byte, 0, 32)
	one := make([]byte, 1)

	for {
		if _, err := io.ReadFull(r, one); err != nil {
			return nil, fmt.Errorf("driver: read looking for STX: %w", err)
		}
		if one[0] == protocol.STX {
			frame = append(frame, one[0])
			break
		}
	}

	for {
		if _, err := io.ReadFull(r, one); err != nil {
			return nil, fmt.Errorf("driver: read frame body: %w", err)
		}
		frame = append(frame, one[0])
		if one[0] == protocol.ETX {
			break
		}
		if len(frame) > maxFrameLen {
			return nil, fmt.Errorf("driver: frame exceeded %d bytes without ETX", maxFrameLen)
		}
	}

	if _, err := io.ReadFull(r, one); err != nil {
		return nil, fmt.Errorf("driver: read BCC byte: %w", err)
	}
	frame = append(frame, one[0])

	return frame, nil
}
