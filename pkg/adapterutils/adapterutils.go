package adapterutils

import (
	"encoding/binary"
	"math"
)

// float32ToBytes serialises a float32 slice into a little-endian byte slice
// suitable for storage in a Redis VECTOR field.
func Float32ToBytes(vec []float32) []byte {
	buf := make([]byte, len(vec)*4)
	for i, v := range vec {
		binary.LittleEndian.PutUint32(buf[i*4:], math.Float32bits(v))
	}
	return buf
}
