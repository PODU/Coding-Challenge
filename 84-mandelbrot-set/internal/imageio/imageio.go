// Package imageio writes images to PNG, including a pHYs chunk carrying the
// physical resolution (DPI) which the standard encoder does not expose.
package imageio

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"image"
	"image/png"
	"io"
	"math"
	"os"
)

const inchesPerMetre = 39.3701

// DPIToPPM converts a DPI value to pixels-per-metre, rounded to the nearest int.
func DPIToPPM(dpi int) uint32 {
	return uint32(math.Round(float64(dpi) * inchesPerMetre))
}

// Encode writes img to w as a PNG. If dpi > 0 it also records the resolution.
func Encode(w io.Writer, img image.Image, dpi int) error {
	var buf bytes.Buffer
	enc := png.Encoder{CompressionLevel: png.DefaultCompression}
	if err := enc.Encode(&buf, img); err != nil {
		return err
	}
	out := buf.Bytes()
	if dpi > 0 {
		var err error
		out, err = insertPHYs(out, DPIToPPM(dpi))
		if err != nil {
			return err
		}
	}
	_, err := w.Write(out)
	return err
}

// EncodeFile encodes img to the file at path.
func EncodeFile(path string, img image.Image, dpi int) (err error) {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := f.Close(); err == nil {
			err = cerr
		}
	}()
	return Encode(f, img, dpi)
}

const pngSignatureLen = 8

// insertPHYs adds a pHYs chunk right after IHDR.
func insertPHYs(pngData []byte, ppm uint32) ([]byte, error) {
	if len(pngData) < pngSignatureLen+8 {
		return nil, fmt.Errorf("imageio: PNG too short")
	}
	// IHDR is always the first chunk. Chunks are length(4) type(4) data crc(4).
	ihdrLen := binary.BigEndian.Uint32(pngData[pngSignatureLen : pngSignatureLen+4])
	ihdrEnd := pngSignatureLen + 4 + 4 + int(ihdrLen) + 4
	if ihdrEnd > len(pngData) {
		return nil, fmt.Errorf("imageio: malformed IHDR chunk")
	}

	chunk := buildPHYs(ppm)
	out := make([]byte, 0, len(pngData)+len(chunk))
	out = append(out, pngData[:ihdrEnd]...)
	out = append(out, chunk...)
	out = append(out, pngData[ihdrEnd:]...)
	return out, nil
}

func buildPHYs(ppm uint32) []byte {
	data := make([]byte, 9)
	binary.BigEndian.PutUint32(data[0:4], ppm) // pixels per unit, X
	binary.BigEndian.PutUint32(data[4:8], ppm) // pixels per unit, Y
	data[8] = 1                                // unit specifier: metre

	chunk := make([]byte, 0, 4+4+9+4)
	var length [4]byte
	binary.BigEndian.PutUint32(length[:], uint32(len(data)))
	chunk = append(chunk, length[:]...)
	chunk = append(chunk, []byte("pHYs")...)
	chunk = append(chunk, data...)

	crc := crc32.NewIEEE()
	crc.Write([]byte("pHYs"))
	crc.Write(data)
	var crcBytes [4]byte
	binary.BigEndian.PutUint32(crcBytes[:], crc.Sum32())
	chunk = append(chunk, crcBytes[:]...)
	return chunk
}

// ReadPHYs returns the X pixels-per-metre from a PNG's pHYs chunk, if any.
func ReadPHYs(pngData []byte) (ppm uint32, ok bool) {
	pos := pngSignatureLen
	for pos+8 <= len(pngData) {
		length := binary.BigEndian.Uint32(pngData[pos : pos+4])
		typ := string(pngData[pos+4 : pos+8])
		dataStart := pos + 8
		dataEnd := dataStart + int(length)
		if dataEnd+4 > len(pngData) {
			break
		}
		if typ == "pHYs" {
			return binary.BigEndian.Uint32(pngData[dataStart : dataStart+4]), true
		}
		pos = dataEnd + 4
		if typ == "IEND" {
			break
		}
	}
	return 0, false
}
