package adapter

import (
	"fmt"

	qrcode "github.com/skip2/go-qrcode"
)

// QRCodeGenerator implements QRGenerator by encoding ticket codes into PNG QR images
// using the go-qrcode library.
type QRCodeGenerator struct {
	size int
}

// NewQRCodeGenerator creates a new QRCodeGenerator with the given image size in pixels.
// If size is zero or negative, it defaults to 256px.
//
// Parameters:
//   - size: The width and height of the generated QR image in pixels.
//
// Returns:
//   - *QRCodeGenerator: A pointer to the newly created generator.
func NewQRCodeGenerator(size int) *QRCodeGenerator {
	if size <= 0 {
		size = 256
	}

	return &QRCodeGenerator{size: size}
}

// Generate encodes the given ticket code into a PNG QR image.
//
// Parameters:
//   - code: The ticket UUID code to encode.
//
// Returns:
//   - []byte: The PNG-encoded QR image bytes.
//   - error: A wrapped error if QR generation fails; otherwise, nil.
func (g *QRCodeGenerator) Generate(code string) ([]byte, error) {
	png, err := qrcode.Encode(code, qrcode.Medium, g.size)
	if err != nil {
		return nil, fmt.Errorf("generating QR code: %w", err)
	}

	return png, nil
}
