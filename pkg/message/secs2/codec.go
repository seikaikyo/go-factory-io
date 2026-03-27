package secs2

// Codec implements message.Codec for SECS-II format.
type Codec struct{}

// NewCodec creates a new SECS-II codec.
func NewCodec() *Codec {
	return &Codec{}
}

// Encode serializes a SECS-II Item into binary format.
// body must be *Item.
func (c *Codec) Encode(body interface{}) ([]byte, error) {
	item, ok := body.(*Item)
	if !ok {
		return nil, ErrInvalidBody
	}
	return Encode(item)
}

// Decode deserializes SECS-II binary data into an *Item.
func (c *Codec) Decode(data []byte) (interface{}, error) {
	return Decode(data)
}
