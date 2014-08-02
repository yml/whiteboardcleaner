package whiteboardcleaner

import (
	"fmt"
	"image"
	"image/jpeg"
	"io/ioutil"
	"log"
	"os"

	"github.com/disintegration/gift"
)

func NewTempFile(prefix string) (*os.File, error) {
	dirName, err := ioutil.TempDir("/tmp", "whiteboardcleaner")
	if err != nil {
		panic(fmt.Sprintf("An error occured while creating a the temp dir", err))
	}
	return ioutil.TempFile(dirName, prefix)
}

type Options struct {
	EdgeDetectionKernelSize        int     // Need to be an odd kernel size (default 15)
	ConvolutionMultiplicator       float32 // default 15
	GaussianBlurSigma              float32 // default 1
	SigmoidMidpoint, SigmoidFactor float32 // SigmoidMidpoint must be between 0 and 1 (default 0.75 100)
	MedianKsize                    int     // MedianKsize must be positive odd number 3, 5, 7 (default 3)
}

func NewOptions() *Options {
	return &Options{
		EdgeDetectionKernelSize:  15,
		ConvolutionMultiplicator: 15,
		GaussianBlurSigma:        3,
		SigmoidMidpoint:          0.75,
		SigmoidFactor:            100,
		MedianKsize:              3,
	}
}

func NewFilter(opts *Options) *gift.GIFT {
	// Vodoo suggested by gift author https://github.com/disintegration/gift/issues/5
	return gift.New(
		gift.Convolution(EdgeKernel(opts.EdgeDetectionKernelSize), true, false, false, 0),
		gift.Convolution([]float32{opts.ConvolutionMultiplicator}, false, false, false, 0),
		gift.Invert(),
		gift.GaussianBlur(opts.GaussianBlurSigma),
		gift.Sigmoid(opts.SigmoidMidpoint, opts.SigmoidFactor),
		gift.Median(opts.MedianKsize, true),
	)
}

func EdgeKernel(size int) []float32 {
	center := size / 2
	kernel := make([]float32, size*size)
	for x := 0; x < size; x++ {
		for y := 0; y < size; y++ {
			if x == center && y == center {
				kernel[size*y+x] = (1.0 - float32(size*size))
			} else {
				kernel[size*y+x] = 1.0
			}
		}
	}
	return kernel
}

func LoadImage(f string) image.Image {
	file, err := os.Open(f)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()
	img, _, err := image.Decode(file)
	if err != nil {
		log.Fatal(err)
	}
	return img
}

func SaveImage(img image.Image, f string) {
	out, err := os.Create(f)
	if err != nil {
		log.Fatal(err)
	}
	defer out.Close()
	jpeg.Encode(out, img, &jpeg.Options{Quality: 99})
}
