package protocol

import "testing"

func TestBuildFrameRoundTrip(t *testing.T) {
	cases := [][]byte{
		[]byte("01499"),
		[]byte("0001499"),
		[]byte("112501874"),
		[]byte("1001250001874"),
		[]byte(""), // empty payload should still round-trip
	}

	for _, payload := range cases {
		frame := BuildFrame(payload)
		got, err := ParseFrame(frame)
		if err != nil {
			t.Fatalf("ParseFrame(BuildFrame(%q)) returned error: %v", payload, err)
		}
		if string(got) != string(payload) {
			t.Fatalf("ParseFrame(BuildFrame(%q)) = %q, want %q", payload, got, payload)
		}
	}
}

// TestBuildFrameKnownExamples locks in the exact bytes for the four example
// frames given by the scale's documentation, so a future change to the framing
// or BCC algorithm is caught immediately.
func TestBuildFrameKnownExamples(t *testing.T) {
	cases := []struct {
		name    string
		payload string
		want    []byte
	}{
		{
			name:    "dialog02 request",
			payload: "01499",
			want:    []byte{STX, '0', '1', '4', '9', '9', ETX, 0x36},
		},
		{
			name:    "dialog04 request",
			payload: "0001499",
			want:    []byte{STX, '0', '0', '0', '1', '4', '9', '9', ETX, 0x36},
		},
		{
			name:    "dialog02 response",
			payload: "112501874",
			want:    []byte{STX, '1', '1', '2', '5', '0', '1', '8', '7', '4', ETX, 0x3E},
		},
		{
			name:    "dialog04 response",
			payload: "1001250001874",
			want:    []byte{STX, '1', '0', '0', '1', '2', '5', '0', '0', '0', '1', '8', '7', '4', ETX, 0x3E},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := BuildFrame([]byte(tc.payload))
			if string(got) != string(tc.want) {
				t.Fatalf("BuildFrame(%q) = % X, want % X", tc.payload, got, tc.want)
			}
		})
	}
}

func TestParseFrameRejectsMissingSTX(t *testing.T) {
	frame := BuildFrame([]byte("01499"))
	frame[0] = 0xFF
	if _, err := ParseFrame(frame); err == nil {
		t.Fatal("expected error for frame with corrupted STX, got nil")
	}
}

func TestParseFrameRejectsMissingETX(t *testing.T) {
	frame := BuildFrame([]byte("01499"))
	frame[len(frame)-2] = 0xFF // overwrite ETX position
	if _, err := ParseFrame(frame); err == nil {
		t.Fatal("expected error for frame with corrupted ETX, got nil")
	}
}

func TestParseFrameRejectsBadBCC(t *testing.T) {
	frame := BuildFrame([]byte("01499"))
	frame[len(frame)-1] ^= 0xFF // flip every bit of the BCC byte
	if _, err := ParseFrame(frame); err == nil {
		t.Fatal("expected error for frame with corrupted BCC, got nil")
	}
}

func TestParseFrameRejectsTruncation(t *testing.T) {
	frame := BuildFrame([]byte("112501874"))
	truncated := frame[:len(frame)-4] // drop the last data byte, ETX and BCC
	if _, err := ParseFrame(truncated); err == nil {
		t.Fatal("expected error for truncated frame, got nil")
	}
}

func TestParseFrameDetectsSingleBitFlipInPayload(t *testing.T) {
	frame := BuildFrame([]byte("112501874"))
	frame[3] ^= 0x01 // flip one bit inside the payload
	if _, err := ParseFrame(frame); err == nil {
		t.Fatal("expected BCC mismatch for single-bit payload corruption, got nil")
	}
}
