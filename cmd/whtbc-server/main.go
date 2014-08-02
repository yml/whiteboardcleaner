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
	indexTmpl string = `<!DOCTYPE html>
<html>
	<head>
		<meta charset="UTF-8"/>
	</head>
	<body>
		<form action="/upload/" method="POST" enctype="multipart/form-data">
			<fieldset>
			<legend>Edge detection</legend>
			{{ if .Errors.EdgeDetectionKernelSize }}<div class="error">{{ .Errors.EdgeDetectionKernelSize }}</div>{{ end }}
			<label for="EdgeDetectionKernelSize">EdgeDetectionKernelSize</label>
			<input name="EdgeDetectionKernelSize" type="text" value="{{ .Opts.EdgeDetectionKernelSize }}"></input>

			{{ if .Errors.ConvolutionMultiplicator }}<div class="error">{{ .Errors.ConvolutionMultiplicator }}</div>{{ end }}
			<label for="ConvolutionMultiplicator">ConvolutionMultiplicator</label>
			<input name="ConvolutionMultiplicator" type="text" value="{{ .Opts.ConvolutionMultiplicator }}"></input>
			</fieldset>

			<fieldset>
			<legend>cleanup the image to get a white backgound</legend>

			{{ if .Errors.GaussianBlurSigma }}<div class="error">{{ .Errors.GaussianBlurSigma }}</div>{{ end }}
			<label for="GaussianBlurSigma">GaussianBlurSigma</label>
			<input name="GaussianBlurSigma" type="text" value="{{ .Opts.GaussianBlurSigma }}"></input>
		
			{{ if .Errors.SigmoidMidpoint }}<div class="error">{{ .Errors.SigmoidMidpoint }}</div>{{ end }}
			<label for="SigmoidMidpoint">SigmoidMidpoint</label>
			<input name="SigmoidMidpoint" type="text" value="{{ .Opts.SigmoidMidpoint }}"></input>

			{{ if .Errors.MedianKsize }}<div class="error">{{ .Errors.MedianKsize }}</div>{{ end }}
			<label for="MedianKsize">MedianKsize</label>
			<input name="MedianKsize" type="text" value="{{ .Opts.MedianKsize }}"></input>
			</fieldset>
			
			<fieldset>
			<legend>Image</legend>
			{{ if .Errors.file }}<div class="error">{{ .Errors.file }}</div>{{ end }}
			<label for="file">File:</label>
			<input name="file" type="file"></input>
			</fieldset>

			<input type="submit"></input>
		</form>
	</body>
</html>
`

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
		errors := make(map[string]string)
		// Update filterOpts with the values from the form
		for k, v := range r.MultipartForm.Value {
			switch k {
			case "EdgeDetectionKernelSize":
				val, err := strconv.Atoi(v[0])
				if err != nil {
					errors["EdgeDetectionKernelSize"] = err.Error()
				}
				filterOpts.EdgeDetectionKernelSize = val
			case "ConvolutionMultiplicator":
				val, err := strconv.ParseFloat(v[0], 32)
				if err != nil {
					errors["ConvolutionMultiplicator"] = err.Error()
				}
				filterOpts.ConvolutionMultiplicator = float32(val)
			case "GaussianBlurSigma":
				val, err := strconv.ParseFloat(v[0], 32)
				if err != nil {
					errors["GaussianBlurSigma"] = err.Error()
				}
				filterOpts.GaussianBlurSigma = float32(val)
			case "SigmoidMidpoint":
				val, err := strconv.ParseFloat(v[0], 32)
				if err != nil {
					errors["SigmoidMidpoint"] = err.Error()
				}
				filterOpts.SigmoidMidpoint = float32(val)
			case "SigmoidFactor":
				val, err := strconv.ParseFloat(v[0], 32)
				if err != nil {
					errors["SigmoidFactor"] = err.Error()
				}
				filterOpts.SigmoidFactor = float32(val)
			case "MedianKsize":
				val, err := strconv.Atoi(v[0])
				if err != nil {
					errors["MedianKsize"] = err.Error()
				}
				filterOpts.MedianKsize = val

			}
		}
		if len(errors) > 0 {
			tmpl := template.New("index")
			tmpl, err := tmpl.Parse(indexTmpl)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			tmpl.Execute(
				w,
				struct {
					Opts   *whiteboardcleaner.Options
					Errors map[string]string
				}{Opts: filterOpts, Errors: errors})

			return

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
	tmpl := template.New("index")
	tmpl, err := tmpl.Parse(indexTmpl)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	filterOpts := whiteboardcleaner.NewOptions()
	errors := make(map[string]string)
	tmpl.Execute(
		w,
		struct {
			Opts   *whiteboardcleaner.Options
			Errors map[string]string
		}{Opts: filterOpts, Errors: errors})
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
