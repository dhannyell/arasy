package asset

import (
	"encoding/binary"
	"errors"
	"fmt"
)

const pngSignature = "\x89PNG\r\n\x1a\n"

type textureDimensions struct {
	width  int
	height int
}

func declaredTextureDimensions(texture TextureDecl) (textureDimensions, error) {
	if texture.Dynamic {
		return textureDimensions{width: texture.W, height: texture.H}, nil
	}
	return encodedTextureDimensions(texture.Data)
}

// encodedTextureDimensions reads only the PNG/JPEG headers supported by
// TextureDecl. Full pixel decoding remains owned by asset/pixel, so loading a
// manifest does not pull image decoders into runtime or WASM binaries.
func encodedTextureDimensions(data []byte) (textureDimensions, error) {
	switch {
	case len(data) >= len(pngSignature) && string(data[:len(pngSignature)]) == pngSignature:
		return pngDimensions(data)
	case len(data) >= 2 && data[0] == 0xff && data[1] == 0xd8:
		return jpegDimensions(data)
	default:
		return textureDimensions{}, errors.New("unsupported texture format; want PNG or JPEG")
	}
}

func pngDimensions(data []byte) (textureDimensions, error) {
	const pngHeaderSize = 24
	if len(data) < pngHeaderSize || binary.BigEndian.Uint32(data[8:12]) != 13 || string(data[12:16]) != "IHDR" {
		return textureDimensions{}, errors.New("invalid PNG header")
	}

	width := binary.BigEndian.Uint32(data[16:20])
	height := binary.BigEndian.Uint32(data[20:24])
	if width == 0 || height == 0 || uint64(width) > uint64(maxInt()) || uint64(height) > uint64(maxInt()) {
		return textureDimensions{}, fmt.Errorf("invalid PNG dimensions %dx%d", width, height)
	}
	return textureDimensions{width: int(width), height: int(height)}, nil
}

func jpegDimensions(data []byte) (textureDimensions, error) {
	for offset := 2; offset < len(data); {
		for offset < len(data) && data[offset] == 0xff {
			offset++
		}
		if offset >= len(data) {
			break
		}

		marker := data[offset]
		offset++
		switch {
		case marker == 0xd9 || marker == 0xda:
			return textureDimensions{}, errors.New("JPEG has no dimensions before image data")
		case marker == 0x01 || marker >= 0xd0 && marker <= 0xd8:
			continue
		}

		if offset+2 > len(data) {
			return textureDimensions{}, errors.New("truncated JPEG segment length")
		}
		segmentLength := int(binary.BigEndian.Uint16(data[offset : offset+2]))
		if segmentLength < 2 || segmentLength > len(data)-offset {
			return textureDimensions{}, errors.New("invalid JPEG segment length")
		}
		if jpegStartOfFrame(marker) {
			if segmentLength < 7 {
				return textureDimensions{}, errors.New("truncated JPEG start-of-frame segment")
			}
			height := int(binary.BigEndian.Uint16(data[offset+3 : offset+5]))
			width := int(binary.BigEndian.Uint16(data[offset+5 : offset+7]))
			if width == 0 || height == 0 {
				return textureDimensions{}, fmt.Errorf("invalid JPEG dimensions %dx%d", width, height)
			}
			return textureDimensions{width: width, height: height}, nil
		}
		offset += segmentLength
	}
	return textureDimensions{}, errors.New("JPEG has no start-of-frame segment")
}

func jpegStartOfFrame(marker byte) bool {
	return marker >= 0xc0 && marker <= 0xcf &&
		marker != 0xc4 && marker != 0xc8 && marker != 0xcc
}

func maxInt() int {
	return int(^uint(0) >> 1)
}
