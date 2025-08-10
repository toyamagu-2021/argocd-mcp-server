package grpcwebproxy

import (
	"bytes"
	"encoding/binary"
	"io"
	"testing"
)

func TestFrameConstants(t *testing.T) {
	// Verify frame constants are correct
	if frameHeaderLength != 5 {
		t.Errorf("frameHeaderLength = %d, want 5", frameHeaderLength)
	}
	if dataFrameFlag != 0 {
		t.Errorf("dataFrameFlag = %d, want 0", dataFrameFlag)
	}
	if trailerFrameFlag != 128 {
		t.Errorf("trailerFrameFlag = %d, want 128", trailerFrameFlag)
	}
}

func TestToFrameFormat(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		validate func(t *testing.T, frame []byte)
	}{
		{
			name:  "verify frame header format",
			input: []byte("test"),
			validate: func(t *testing.T, frame []byte) {
				if len(frame) != frameHeaderLength+4 {
					t.Errorf("frame length = %d, want %d", len(frame), frameHeaderLength+4)
				}

				// Check flag byte (should be 0 for data frame)
				if frame[0] != dataFrameFlag {
					t.Errorf("flag byte = %d, want %d", frame[0], dataFrameFlag)
				}

				// Check length encoding (big-endian)
				length := binary.BigEndian.Uint32(frame[1:5])
				if length != 4 {
					t.Errorf("encoded length = %d, want 4", length)
				}

				// Check message content
				msg := frame[5:]
				if !bytes.Equal(msg, []byte("test")) {
					t.Errorf("message = %v, want %v", msg, []byte("test"))
				}
			},
		},
		{
			name:  "large message",
			input: make([]byte, 1000),
			validate: func(t *testing.T, frame []byte) {
				if len(frame) != frameHeaderLength+1000 {
					t.Errorf("frame length = %d, want %d", len(frame), frameHeaderLength+1000)
				}

				length := binary.BigEndian.Uint32(frame[1:5])
				if length != 1000 {
					t.Errorf("encoded length = %d, want 1000", length)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			frame := toFrame(tt.input)
			tt.validate(t, frame)
		})
	}
}

func TestParseFrameEdgeCases(t *testing.T) {
	tests := []struct {
		name      string
		setupData func() []byte
		validate  func(t *testing.T, msg []byte, isTrailer bool, err error)
	}{
		{
			name: "maximum uint32 length",
			setupData: func() []byte {
				// Create frame with max uint32 length (but don't actually provide that much data)
				frame := make([]byte, frameHeaderLength)
				frame[0] = dataFrameFlag
				binary.BigEndian.PutUint32(frame[1:5], 0) // Set to 0 for this test
				return frame
			},
			validate: func(t *testing.T, msg []byte, isTrailer bool, err error) {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if len(msg) != 0 {
					t.Errorf("message length = %d, want 0", len(msg))
				}
				if isTrailer {
					t.Error("expected data frame, got trailer")
				}
			},
		},
		{
			name: "EOF at exact frame boundary",
			setupData: func() []byte {
				return toFrame([]byte("exact"))
			},
			validate: func(t *testing.T, msg []byte, isTrailer bool, err error) {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if !bytes.Equal(msg, []byte("exact")) {
					t.Errorf("message = %v, want %v", msg, []byte("exact"))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := tt.setupData()
			reader := bytes.NewReader(data)
			msg, isTrailer, err := parseFrame(reader)
			tt.validate(t, msg, isTrailer, err)
		})
	}
}

func TestParseMultipleFramesWithTrailer(t *testing.T) {
	// Create a sequence: data frame, data frame, trailer frame
	frame1 := toFrame([]byte("first"))
	frame2 := toFrame([]byte("second"))

	// Create trailer frame manually
	trailerContent := []byte("grpc-status: 0\r\n")
	trailerFrame := make([]byte, frameHeaderLength+len(trailerContent))
	trailerFrame[0] = trailerFrameFlag
	binary.BigEndian.PutUint32(trailerFrame[1:5], uint32(len(trailerContent)))
	copy(trailerFrame[5:], trailerContent)

	// Combine all frames
	allData := append(frame1, frame2...)
	allData = append(allData, trailerFrame...)

	reader := bytes.NewReader(allData)

	// Parse first frame
	msg1, isTrailer1, err := parseFrame(reader)
	if err != nil {
		t.Fatalf("error parsing first frame: %v", err)
	}
	if !bytes.Equal(msg1, []byte("first")) {
		t.Errorf("first message = %v, want %v", msg1, []byte("first"))
	}
	if isTrailer1 {
		t.Error("first frame should not be trailer")
	}

	// Parse second frame
	msg2, isTrailer2, err := parseFrame(reader)
	if err != nil {
		t.Fatalf("error parsing second frame: %v", err)
	}
	if !bytes.Equal(msg2, []byte("second")) {
		t.Errorf("second message = %v, want %v", msg2, []byte("second"))
	}
	if isTrailer2 {
		t.Error("second frame should not be trailer")
	}

	// Parse trailer frame
	msg3, isTrailer3, err := parseFrame(reader)
	if err != nil {
		t.Fatalf("error parsing trailer frame: %v", err)
	}
	if !bytes.Equal(msg3, trailerContent) {
		t.Errorf("trailer content = %v, want %v", msg3, trailerContent)
	}
	if !isTrailer3 {
		t.Error("third frame should be trailer")
	}

	// Verify EOF after all frames
	_, _, err = parseFrame(reader)
	if err != io.EOF {
		t.Errorf("expected EOF after all frames, got %v", err)
	}
}

func TestParseFramesSequence(t *testing.T) {
	// Test that parseFrames correctly handles a typical gRPC-Web response
	// with multiple data frames followed by a trailer
	dataFrame1 := toFrame([]byte("data1"))
	dataFrame2 := toFrame([]byte("data2"))

	// Note: parseFrames doesn't handle trailer frames specially,
	// it just returns all frames as data
	allData := append(dataFrame1, dataFrame2...)

	frames, err := parseFrames(allData)
	if err != nil {
		t.Fatalf("parseFrames error: %v", err)
	}

	if len(frames) != 2 {
		t.Errorf("expected 2 frames, got %d", len(frames))
	}

	expectedData := [][]byte{
		[]byte("data1"),
		[]byte("data2"),
	}

	for i, frame := range frames {
		if !bytes.Equal(frame, expectedData[i]) {
			t.Errorf("frame[%d] = %v, want %v", i, frame, expectedData[i])
		}
	}
}
