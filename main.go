package main

import (
	"embed"
	"fmt"
	"html/template"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

//go:embed templates/*
var templatesFS embed.FS

const (
	uploadDir = "./uploads"
	username  = "admin"
	password  = "password" // ユーザーが後で変更できるようにするか、固定にするか
)

func main() {
	// アップロード用ディレクトリの作成
	if _, err := os.Stat(uploadDir); os.IsNotExist(err) {
		os.Mkdir(uploadDir, 0755)
	}

	http.HandleFunc("/", basicAuth(handleIndex))
	http.HandleFunc("/upload", basicAuth(handleUpload))
	http.HandleFunc("/download/", basicAuth(handleDownload))
	http.HandleFunc("/delete/", basicAuth(handleDelete))

	port := "8080"
	ip := getLocalIP()
	fmt.Printf("Server started at http://%s:%s\n", ip, port)
	fmt.Printf("Access from other devices using the same Wi-Fi/Hotspot.\n")

	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func getLocalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "localhost"
	}
	for _, address := range addrs {
		if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String()
			}
		}
	}
	return "localhost"
}

func basicAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok || user != username || pass != password {
			w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	}
}

func handleIndex(w http.ResponseWriter, r *http.Request) {
	files, err := os.ReadDir(uploadDir)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var fileList []string
	for _, file := range files {
		if !file.IsDir() {
			fileList = append(fileList, file.Name())
		}
	}

	tmpl, err := template.ParseFS(templatesFS, "templates/index.html")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	tmpl.Execute(w, fileList)
}

func handleUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	err := r.ParseMultipartForm(100 << 20) // 100MB limit
	if err != nil {
		http.Error(w, "File too large", http.StatusBadRequest)
		return
	}

	files := r.MultipartForm.File["files"]
	for _, fileHeader := range files {
		file, err := fileHeader.Open()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer file.Close()

		dst, err := os.Create(filepath.Join(uploadDir, filepath.Base(fileHeader.Filename)))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer dst.Close()

		if _, err := io.Copy(dst, file); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func handleDownload(w http.ResponseWriter, r *http.Request) {
	filename := strings.TrimPrefix(r.URL.Path, "/download/")
	filePath := filepath.Join(uploadDir, filename)
	http.ServeFile(w, r, filePath)
}

func handleDelete(w http.ResponseWriter, r *http.Request) {
	filename := strings.TrimPrefix(r.URL.Path, "/delete/")
	filePath := filepath.Join(uploadDir, filename)
	err := os.Remove(filePath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}
