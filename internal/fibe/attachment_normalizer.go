package fibe

import (
	"bytes"
	"image"
	"image/color"
	stddraw "image/draw"
	"image/jpeg"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	xdraw "golang.org/x/image/draw"

	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/tiff"
	_ "golang.org/x/image/webp"
	_ "image/gif"
	_ "image/png"
)

const maxConvertedImageDimension = 3072

type preparedAttachment struct {
	path    string
	cleanup func()
}

func prepareMessageAttachmentPaths(paths []string) ([]preparedAttachment, error) {
	prepared := make([]preparedAttachment, 0, len(paths))
	for _, path := range paths {
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}
		normalized, cleanup, err := normalizeImageAttachmentForDelivery(path)
		if err != nil {
			for _, attachment := range prepared {
				if attachment.cleanup != nil {
					attachment.cleanup()
				}
			}
			return nil, err
		}
		prepared = append(prepared, preparedAttachment{path: normalized, cleanup: cleanup})
	}
	return prepared, nil
}

func cleanupPreparedAttachments(prepared []preparedAttachment) {
	for _, attachment := range prepared {
		if attachment.cleanup != nil {
			attachment.cleanup()
		}
	}
}

func normalizeImageAttachmentForDelivery(path string) (string, func(), error) {
	contentType := attachmentContentType(path)
	if !shouldConvertImageAttachment(path, contentType) {
		return path, nil, nil
	}

	data, err := convertedJPEGAttachment(path)
	if err != nil || len(data) == 0 {
		return path, nil, nil
	}
	file, err := os.CreateTemp("", "likeable-attachment-*.jpg")
	if err != nil {
		return "", nil, err
	}
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		_ = os.Remove(file.Name())
		return "", nil, err
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(file.Name())
		return "", nil, err
	}
	return file.Name(), func() { _ = os.Remove(file.Name()) }, nil
}

func shouldConvertImageAttachment(path, contentType string) bool {
	if contentType == "" || !strings.HasPrefix(contentType, "image/") {
		return false
	}
	if contentType == "image/svg+xml" {
		return false
	}
	if !supportedInlineImageContentType(contentType) {
		return true
	}
	info, err := os.Stat(path)
	return err == nil && info.Size() > maxInlineImageAttachmentBytes
}

func convertedJPEGAttachment(path string) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	src, _, err := image.Decode(file)
	if err != nil {
		return nil, err
	}
	attempts := []struct {
		maxDimension int
		quality      int
	}{
		{maxConvertedImageDimension, 88},
		{maxConvertedImageDimension, 80},
		{2400, 82},
		{1800, 82},
		{1400, 80},
	}
	for i, attempt := range attempts {
		img := flattenAndResizeImage(src, attempt.maxDimension)
		var buf bytes.Buffer
		if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: attempt.quality}); err != nil {
			return nil, err
		}
		if buf.Len() <= maxInlineImageAttachmentBytes || i == len(attempts)-1 {
			return buf.Bytes(), nil
		}
	}
	return nil, nil
}

func flattenAndResizeImage(src image.Image, maxDimension int) image.Image {
	bounds := src.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	if width <= 0 || height <= 0 {
		return src
	}
	targetWidth := width
	targetHeight := height
	if max := max(width, height); max > maxDimension {
		targetWidth = width * maxDimension / max
		targetHeight = height * maxDimension / max
	}
	if targetWidth < 1 {
		targetWidth = 1
	}
	if targetHeight < 1 {
		targetHeight = 1
	}
	dst := image.NewRGBA(image.Rect(0, 0, targetWidth, targetHeight))
	stddraw.Draw(dst, dst.Bounds(), &image.Uniform{C: color.White}, image.Point{}, stddraw.Src)
	if targetWidth == width && targetHeight == height {
		stddraw.Draw(dst, dst.Bounds(), src, bounds.Min, stddraw.Over)
		return dst
	}
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), src, bounds, xdraw.Over, nil)
	return dst
}

func attachmentContentType(path string) string {
	contentType := strings.ToLower(strings.TrimSpace(mime.TypeByExtension(filepath.Ext(path))))
	contentType = strings.TrimSpace(strings.Split(contentType, ";")[0])
	if contentType != "" && contentType != "application/octet-stream" {
		return contentType
	}
	file, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer file.Close()
	buf := make([]byte, 512)
	n, _ := file.Read(buf)
	detected := strings.ToLower(strings.TrimSpace(http.DetectContentType(buf[:n])))
	return strings.TrimSpace(strings.Split(detected, ";")[0])
}

func supportedInlineImageContentType(contentType string) bool {
	switch contentType {
	case "image/png", "image/jpeg", "image/gif":
		return true
	default:
		return false
	}
}
