package grpcwebproxy

import (
	"fmt"

	gogoproto "github.com/gogo/protobuf/proto"
	"google.golang.org/protobuf/proto"
)

// noopCodec is a no-operation codec that passes through raw bytes without encoding/decoding
type noopCodec struct{}

// Name returns the name of the codec
func (c *noopCodec) Name() string {
	return "proto"
}

// Marshal returns the raw bytes without any encoding
func (c *noopCodec) Marshal(v interface{}) ([]byte, error) {
	switch msg := v.(type) {
	case proto.Message:
		return proto.Marshal(msg)
	case gogoproto.Message:
		return gogoproto.Marshal(msg)
	case []byte:
		return msg, nil
	case *[]byte:
		if msg != nil {
			return *msg, nil
		}
		return nil, nil
	default:
		return nil, fmt.Errorf("unsupported message type: %T", v)
	}
}

// Unmarshal returns the raw bytes without any decoding
func (c *noopCodec) Unmarshal(data []byte, v interface{}) error {
	switch msg := v.(type) {
	case proto.Message:
		return proto.Unmarshal(data, msg)
	case gogoproto.Message:
		return gogoproto.Unmarshal(data, msg)
	case *[]byte:
		*msg = data
		return nil
	default:
		return fmt.Errorf("unsupported message type: %T", v)
	}
}

// Note: We don't register this codec globally anymore to avoid affecting
// other gRPC clients. It's only used via ForceServerCodec in the proxy server.
