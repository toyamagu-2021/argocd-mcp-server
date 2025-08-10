package grpcwebproxy

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
)

const (
	// frameHeaderLength is the length of the gRPC-Web frame header
	frameHeaderLength = 5
	// dataFrameFlag indicates a data frame
	dataFrameFlag = 0
	// trailerFrameFlag indicates a trailer frame
	trailerFrameFlag = 128
)

// toFrame converts a message to gRPC-Web frame format
// Frame format: [1 byte flag] + [4 bytes length] + [message]
func toFrame(msg []byte) []byte {
	frame := make([]byte, frameHeaderLength+len(msg))
	frame[0] = 0 // flag byte (0 for normal message)
	binary.BigEndian.PutUint32(frame[1:5], uint32(len(msg)))
	copy(frame[5:], msg)
	return frame
}

// parseFrame reads a single frame from the reader
func parseFrame(r io.Reader) ([]byte, bool, error) {
	header := make([]byte, frameHeaderLength)
	if _, err := io.ReadFull(r, header); err != nil {
		if err == io.EOF {
			return nil, false, io.EOF
		}
		return nil, false, fmt.Errorf("failed to read frame header: %w", err)
	}

	flag := header[0]
	length := binary.BigEndian.Uint32(header[1:5])

	// Check if this is a trailer frame
	isTrailer := flag == trailerFrameFlag

	// Read the message body
	msg := make([]byte, length)
	if _, err := io.ReadFull(r, msg); err != nil {
		return nil, false, fmt.Errorf("failed to read frame body: %w", err)
	}

	return msg, isTrailer, nil
}

// parseFrames reads all frames from the reader
func parseFrames(data []byte) ([][]byte, error) {
	var frames [][]byte
	r := bytes.NewReader(data)

	for {
		frame, _, err := parseFrame(r)
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		frames = append(frames, frame)
	}

	return frames, nil
}
