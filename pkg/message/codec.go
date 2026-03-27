package message

// Codec encodes and decodes protocol-specific message bodies.
type Codec interface {
	Encode(body interface{}) ([]byte, error)
	Decode(data []byte) (interface{}, error)
}
