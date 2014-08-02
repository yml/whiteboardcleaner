package main

import (
	"flag"
	"image"
	"runtime"

	"github.com/yml/whiteboardcleaner"
)

func main() {
	filterOpts := whiteboardcleaner.NewOptions()
	srcFile := flag.String("src", "", "path to the source image")
	dstFile := flag.String("dst", "", "path of cleaned image")

	flag.Parse()
	if *srcFile == "" {
		panic("A source image is required")
	}
	if *dstFile == "" {
		panic("A destination file path is required")
	}

	runtime.GOMAXPROCS(runtime.NumCPU())
	src := whiteboardcleaner.LoadImage(*srcFile)

	g := whiteboardcleaner.NewFilter(filterOpts)
	dst := image.NewRGBA(g.Bounds(src.Bounds()))
	g.Draw(dst, src)
	whiteboardcleaner.SaveImage(dst, *dstFile)
}
