// Package ocr defines OCR wire values independently of the host.
package ocr

import (
	"context"
	"errors"
)

// Region 是以左上角为原点的像素矩形。
type Region struct {
	X      int `json:"x"`
	Y      int `json:"y"`
	Width  int `json:"width"`
	Height int `json:"height"`
}

// Validate 确保 Region 能描述非空图像区域。
func (r Region) Validate() error {
	if r.X < 0 || r.Y < 0 {
		return errors.New("region origin must not be negative")
	}
	if r.Width <= 0 || r.Height <= 0 {
		return errors.New("region dimensions must be positive")
	}
	return nil
}

// Point 是以左上角为原点的像素坐标。
type Point struct {
	X int `json:"x"`
	Y int `json:"y"`
}

// Text 表示一条 OCR 结果。
type Text struct {
	Value      string   `json:"value"`
	Bounds     Region   `json:"bounds"`
	Polygon    []Point  `json:"polygon,omitempty"`
	Confidence *float64 `json:"confidence,omitempty"`
}

// TextRecognizer 从编码图像中提取可见文本。
type TextRecognizer interface {
	Recognize(ctx context.Context, image []byte, roi *Region) ([]Text, error)
}
