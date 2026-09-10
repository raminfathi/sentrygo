package vision

import (
	"fmt"
	"image"
	_ "image/jpeg" // Required to decode JPEG images from the camera
	"log"
	"os"

	pigo "github.com/esimov/pigo/core"
)

// FaceDetector wraps the Pigo face detection engine.
type FaceDetector struct {
	classifier *pigo.Pigo
}

// NewFaceDetector loads the cascade file and initializes the engine.
func NewFaceDetector(cascadePath string) (*FaceDetector, error) {
	cascadeFile, err := os.ReadFile(cascadePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read cascade file: %w", err)
	}

	p := pigo.NewPigo()
	classifier, err := p.Unpack(cascadeFile)
	if err != nil {
		return nil, fmt.Errorf("failed to unpack cascade: %w", err)
	}

	log.Println("AI Vision Module: Pigo Face Detector initialized successfully.")
	return &FaceDetector{classifier: classifier}, nil
}

// ContainsFace scans an image file and returns true if at least one face is detected.
func (fd *FaceDetector) ContainsFace(imagePath string) bool {
	file, err := os.Open(imagePath)
	if err != nil {
		log.Printf("Vision Error: Cannot open image %s: %v", imagePath, err)
		return false
	}
	defer file.Close()

	img, _, err := image.Decode(file)
	if err != nil {
		log.Printf("Vision Error: Cannot decode image %s: %v", imagePath, err)
		return false
	}

	// Pigo requires grayscale pixels
	pixels := pigo.RgbToGrayscale(img)
	cols, rows := img.Bounds().Max.X, img.Bounds().Max.Y

	cParams := pigo.CascadeParams{
		MinSize:     50,   // Minimum face size (pixels) - adjust if camera is far
		MaxSize:     1000, // Maximum face size
		ShiftFactor: 0.1,  // Sliding window step
		ScaleFactor: 1.1,  // Image scaling factor
		ImageParams: pigo.ImageParams{
			Pixels: pixels,
			Rows:   rows,
			Cols:   cols,
			Dim:    cols,
		},
	}

	// Run the cascade and filter overlapping detections
	dets := fd.classifier.RunCascade(cParams, 0.0)
	dets = fd.classifier.ClusterDetections(dets, 0.2)

	return len(dets) > 0
}
