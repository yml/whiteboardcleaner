package main

import (
	"flag"
	"fmt"
	"image"
	"image/jpeg"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"text/template"

	"github.com/yml/whiteboardcleaner"
)

var (
	maxMemory int64  = 1 * 1024 * 1024 // 1MB
	indexPage []byte = []byte(`<!DOCTYPE html>
<html>
	<head>
		<meta charset="UTF-8"/>
	</head>
	<body>
		<form action="/upload/" method="POST" enctype="multipart/form-data">
			<fieldset>
			<legend>Edge detection</legend>
			<label for="EdgeDetectionKernelSize">EdgeDetectionKernelSize</label>
			<input name="EdgeDetectionKernelSize" type="text"></input>

			<label for="ConvolutionMultiplicator">ConvolutionMultiplicator</label>
			<input name="ConvolutionMultiplicator" type="text"></input>
			</fieldset>

			<fieldset>
			<legend>cleanup the image to get a white backgound</legend>
			<label for="GaussianBlurSigma">GaussianBlurSigma</label>
			<input name="GaussianBlurSigma" type="text"></input>
			<label for="SigmoidMidpoint">SigmoidMidpoint</label>
			<input name="SigmoidMidpoint" type="text"></input>
			<label for="MedianKsize">MedianKsize</label>
			<input name="MedianKsize" type="text"></input>
			</fieldset>
			
			<fieldset>
			<legend>Image</legend>
			<label for="file">File:</label>
			<input name="file" type="file"></input>
			</fieldset>

			<input type="submit"></input>
		</form>
	</body>
</html>
`)

	resultTmpl string = `<!DOCTYPE html>
<html>
	<head>
		<meta charset="UTF-8"/>
	</head>
	<body>
	{{ range . }}<div><img src="{{ . }}"/></div>{{ end }}
	</body>
</html>
`
)

type appContext struct {
	TmpDir                          string
	PrefixTmpDir                    string
	UploadURL, ResultURL, StaticURL string
}

func uploadHandler(ctx *appContext) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(maxMemory); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		filterOpts := whiteboardcleaner.NewOptions()
		// Update filterOpts with the values from the form
		for k, v := range r.MultipartForm.Value {
			switch k {
			case "EdgeDetectionKernelSize":
				val, err := strconv.Atoi(v[0])
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
				}
				filterOpts.EdgeDetectionKernelSize = val
			case "ConvolutionMultiplicator":
				val, err := strconv.ParseFloat(v[0], 32)
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
				}
				filterOpts.ConvolutionMultiplicator = float32(val)
			case "GaussianBlurSigma":
				val, err := strconv.ParseFloat(v[0], 32)
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
				}
				filterOpts.GaussianBlurSigma = float32(val)
			case "SigmoidMidpoint":
				val, err := strconv.ParseFloat(v[0], 32)
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
				}
				filterOpts.SigmoidMidpoint = float32(val)
			case "SigmoidFactor":
				val, err := strconv.ParseFloat(v[0], 32)
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
				}
				filterOpts.SigmoidFactor = float32(val)
			case "MedianKsize":
				val, err := strconv.Atoi(v[0])
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
				}
				filterOpts.MedianKsize = val

			}
		}

		dirPath, err := ioutil.TempDir(ctx.TmpDir, ctx.PrefixTmpDir)
		_, dirName := filepath.Split(dirPath)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}

		for _, fileHeaders := range r.MultipartForm.File {
			for _, fileHeader := range fileHeaders {
				file, err := fileHeader.Open()
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
				}
				tf, err := ioutil.TempFile(dirPath, fmt.Sprintf("%s_", fileHeader.Filename))

				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
				}
				io.Copy(tf, file)
				// rewind the file to the  begining
				tf.Seek(0, 0)
				// Decode the image
				img, err := jpeg.Decode(tf)
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
				}
				g := whiteboardcleaner.NewFilter(filterOpts)
				dst := image.NewRGBA(g.Bounds(img.Bounds()))
				g.Draw(dst, img)
				// Create the dstTemporaryFile
				dstTemporaryFile, err := ioutil.TempFile(dirPath, fmt.Sprintf("cleaned_%s_", fileHeader.Filename))
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
				}
				jpeg.Encode(dstTemporaryFile, dst, &jpeg.Options{Quality: 99})
				http.Redirect(
					w, r, fmt.Sprintf("%s%s", ctx.ResultURL, dirName), http.StatusMovedPermanently)
			}
		}
	}
}

func resultHandler(ctx *appContext) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		dirName, err := filepath.Rel(ctx.ResultURL, path)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		files, err := filepath.Glob(filepath.Join(ctx.TmpDir, dirName, "*"))
		for i, file := range files {
			rel, err := filepath.Rel(os.TempDir(), file)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			files[i] = filepath.Join(ctx.StaticURL, rel)
		}
		tmpl := template.New("result")
		tmpl, err = tmpl.Parse(resultTmpl)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		tmpl.Execute(w, files)
	}
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	w.Write(indexPage)
}

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	addr := flag.String("addr", ":8080", "path to the source image")
	ctx := &appContext{
		TmpDir:       os.TempDir(),
		PrefixTmpDir: "whiteboardcleaner_",
		UploadURL:    "/upload/",
		ResultURL:    "/cleaned/",
		StaticURL:    "/static/",
	}

	fmt.Println("Starting whiteboard cleaner server listening on addr", *addr)

	mux := http.NewServeMux()
	mux.HandleFunc(ctx.UploadURL, uploadHandler(ctx))
	mux.HandleFunc(ctx.ResultURL, resultHandler(ctx))
	mux.Handle(ctx.StaticURL,
		http.StripPrefix(ctx.StaticURL, http.FileServer(http.Dir(os.TempDir()))))
	mux.HandleFunc("/", indexHandler)
	http.ListenAndServe(*addr, mux)
}
