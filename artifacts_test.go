package main

import (
	"archive/zip"
	"math"
	"testing"
)

func TestArtifactAZipEntrySizeBytes(t *testing.T) {
	t.Run("accepts valid size", func(t *testing.T) {
		file := &zip.File{
			FileHeader: zip.FileHeader{
				Name:               "ok.pdf",
				UncompressedSize64: 4096,
			},
		}

		sizeBytes, err := artifactA_zipEntrySizeBytes(file)
		if err != nil {
			t.Fatalf("artifactA_zipEntrySizeBytes returned error: %v", err)
		}
		if sizeBytes != 4096 {
			t.Fatalf("artifactA_zipEntrySizeBytes = %d, want 4096", sizeBytes)
		}
	})

	t.Run("rejects overflow", func(t *testing.T) {
		file := &zip.File{
			FileHeader: zip.FileHeader{
				Name:               "overflow.pdf",
				UncompressedSize64: math.MaxInt64 + 1,
			},
		}

		if _, err := artifactA_zipEntrySizeBytes(file); err == nil {
			t.Fatal("artifactA_zipEntrySizeBytes expected overflow error")
		}
	})
}
