package qp

import (
	"encoding"
	"encoding/binary"
	"errors"
	"io"
)

// ErrPayloadTooShort indicates that the message was not complete.
var ErrPayloadTooShort = errors.New("payload too short")

// Default is the protocol used by the raw Encode and Decode functions.
var Default = NineP2000

// Protocol defines a protocol message encoder/decoder
type Protocol interface {
	Decode(r io.Reader) (Message, error)
	Encode(w io.Writer, m Message) error
}

// MessageType is the type of the contained message.
type MessageType byte

// Message is an interface describing an item that can encode itself to a
// writer, decode itself from a reader, and inform how large the encoded form
// would be at the current time. It is also capable of getting/setting the
// message tag, which is merely a convenience feature to save a type assert
// for access to the tag.
type Message interface {
	encoding.BinaryUnmarshaler
	encoding.BinaryMarshaler
	GetTag() Tag
}

// Write write all the provided data unless and io error occurs.
func write(w io.Writer, b []byte) error {
	var (
		written int
		err     error
		l       = len(b)
	)
	for written < l {
		written, err = w.Write(b[written:])
		if err != nil {
			return err
		}
	}

	return nil
}

// DecodeHdr reads 5 bytes and returns the decoded size and message type. It
// may return an error if reading from the Reader fails.
func DecodeHdr(r io.Reader) (uint32, MessageType, error) {
	var (
		n    int
		size uint32
		mt   MessageType
		err  error
	)

	b := make([]byte, 5)
	n, err = io.ReadFull(r, b)
	if n < 5 {
		return 0, 0, err
	}
	size = binary.LittleEndian.Uint32(b[0:4])
	mt = MessageType(b[4])
	return size, mt, err
}

// Codec encodes/decodes messages using the provided message type <-> message
// conversion.
type Codec struct {
	M2MT func(Message) (MessageType, error)
	MT2M func(MessageType) (Message, error)
}

// Decode decodes an entire message, including header, and returns the message.
// It may return an error if reading from the Reader fails, or if a message
// tries to consume more data than the size of the header indicated, making the
// message invalid.
func (c *Codec) Decode(r io.Reader) (Message, error) {
	var (
		size uint32
		mt   MessageType
		err  error
	)
	if size, mt, err = DecodeHdr(r); err != nil {
		return nil, err
	}

	size -= HeaderSize

	b := make([]byte, size)
	n, err := io.ReadFull(r, b)
	if err != nil {
		return nil, err
	}
	if n != int(size) {
		return nil, errors.New("short read")
	}

	m, err := c.MT2M(mt)
	if err != nil {
		return nil, err
	}

	if err = m.UnmarshalBinary(b); err != nil {
		return nil, err
	}
	return m, nil
}

// Encode write a header and message to the provided writer. It returns an
// error if writing failed.
func (c *Codec) Encode(w io.Writer, m Message) error {
	var err error
	var mt MessageType
	if mt, err = c.M2MT(m); err != nil {
		return err
	}

	var b []byte
	if b, err = m.MarshalBinary(); err != nil {
		return err
	}

	h := make([]byte, 5)
	binary.LittleEndian.PutUint32(h[0:4], uint32(len(b)+HeaderSize))
	h[4] = byte(mt)

	if err = write(w, h); err != nil {
		return err
	}

	if err = write(w, b); err != nil {
		return err
	}

	return nil
}

// Decode is a convenience function for calling decode on the default
// protocol.
func Decode(r io.Reader) (Message, error) {
	return Default.Decode(r)
}

// Encode is a convenience function for calling encode on the default
// protocol.
func Encode(w io.Writer, d Message) error {
	return Default.Encode(w, d)
}
