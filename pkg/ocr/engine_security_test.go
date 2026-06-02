package ocr

import (
	"archive/zip"
	"math"
	"testing"
)

func TestZipEntrySizeBytes(t *testing.T) {
	t.Run("accepts valid size", func(t *testing.T) {
		file := &zip.File{
			FileHeader: zip.FileHeader{
				Name:               "offer.pdf",
				UncompressedSize64: 8192,
			},
		}

		sizeBytes, err := zipEntrySizeBytes(file)
		if err != nil {
			t.Fatalf("zipEntrySizeBytes returned error: %v", err)
		}
		if sizeBytes != 8192 {
			t.Fatalf("zipEntrySizeBytes = %d, want 8192", sizeBytes)
		}
	})

	t.Run("rejects overflow", func(t *testing.T) {
		file := &zip.File{
			FileHeader: zip.FileHeader{
				Name:               "oversized.pdf",
				UncompressedSize64: math.MaxInt64 + 1,
			},
		}

		if _, err := zipEntrySizeBytes(file); err == nil {
			t.Fatal("zipEntrySizeBytes expected overflow error")
		}
	})
}
