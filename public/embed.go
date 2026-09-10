package public

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed index.html app.js styles.css sift.js topsearch.js home-browse.js theme.js
var files embed.FS

func FileSystem() http.FileSystem {
	return http.FS(files)
}

func FS() fs.FS {
	return files
}
