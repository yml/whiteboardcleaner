package main

import (
	"fmt"
	"image"
	"image/jpeg"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
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
		<form action="/upload" method="POST" enctype="multipart/form-data">
			<label for="file">File:</label>
			<input name="file" type="file"></input>
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

func uploadHandler(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(maxMemory); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	for k, v := range r.MultipartForm.Value {
		fmt.Println("form field:", k, v)
	}
	for _, fileHeaders := range r.MultipartForm.File {
		for _, fileHeader := range fileHeaders {
			file, err := fileHeader.Open()
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			dirPath, err := ioutil.TempDir(os.TempDir(), "whiteboardcleaner_")
			_, dirName := filepath.Split(dirPath)
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
			g := whiteboardcleaner.NewFilter()
			dst := image.NewRGBA(g.Bounds(img.Bounds()))
			g.Draw(dst, img)
			// Create the dstTemporaryFile
			dstTemporaryFile, err := ioutil.TempFile(dirPath, fmt.Sprintf("cleaned_%s_", fileHeader.Filename))
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			jpeg.Encode(dstTemporaryFile, dst, &jpeg.Options{Quality: 99})
			http.Redirect(
				w, r, fmt.Sprintf("/cleaned/%s", dirName), http.StatusMovedPermanently)
		}
	}
}

func resultHandler(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	dirName, err := filepath.Rel("/cleaned", path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	files, err := filepath.Glob(filepath.Join(os.TempDir(), dirName, "*"))
	for i, file := range files {
		rel, err := filepath.Rel(os.TempDir(), file)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		files[i] = filepath.Join("/static", rel)
	}
	tmpl := template.New("result")
	tmpl, err = tmpl.Parse(resultTmpl)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	tmpl.Execute(w, files)
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	w.Write(indexPage)
}
func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/upload", uploadHandler)
	mux.HandleFunc("/cleaned/", resultHandler)
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir(os.TempDir()))))
	mux.HandleFunc("/", indexHandler)
	http.ListenAndServe(":8080", mux)
}
