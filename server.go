package main

import (
	"crypto/subtle"
	_ "embed"
	"encoding/json"
	"errors"
	"html/template"
	"io"
	"log"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

//go:embed template/index.html
var indexHTML string

type server struct {
	cfg       *Config
	uploadDir string
	hotspot   Hotspot
	tmpl      *template.Template
	hsMu      sync.Mutex // アクセスポイントの操作を直列にする
}

func newServer(cfg *Config, uploadDir string, hs Hotspot) *server {
	return &server{
		cfg:       cfg,
		uploadDir: uploadDir,
		hotspot:   hs,
		tmpl:      template.Must(template.New("index").Parse(indexHTML)),
	}
}

func (s *server) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.auth(s.handleIndex))
	mux.HandleFunc("/download/", s.auth(s.handleDownload))
	mux.HandleFunc("/api/files", s.auth(s.handleFiles))
	mux.HandleFunc("/api/upload", s.auth(s.handleUpload))
	mux.HandleFunc("/api/text", s.auth(s.handleText))
	mux.HandleFunc("/api/delete", s.auth(s.handleDelete))
	mux.HandleFunc("/api/hotspot", s.auth(s.handleHotspotStatus))
	mux.HandleFunc("/api/hotspot/start", s.auth(s.handleHotspotChange(true)))
	mux.HandleFunc("/api/hotspot/stop", s.auth(s.handleHotspotChange(false)))
	return mux
}

// auth は Basic 認証を要求する。比較は時間差が出ない方法で行う。
func (s *server) auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		userOK := subtle.ConstantTimeCompare([]byte(user), []byte(s.cfg.Username)) == 1
		passOK := subtle.ConstantTimeCompare([]byte(pass), []byte(s.cfg.Password)) == 1
		if !ok || !userOK || !passOK {
			w.Header().Set("WWW-Authenticate", `Basic realm="Send Files"`)
			writeError(w, http.StatusUnauthorized, "認証が必要です")
			return
		}
		next(w, r)
	}
}

func (s *server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	data := struct {
		Version string
		Addrs   []string
		MaxMB   int64
	}{appVersion, localAddrs(s.cfg.Port), s.cfg.MaxUploadMB}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.tmpl.Execute(w, data); err != nil {
		log.Printf("画面の描画に失敗しました: %v", err)
	}
}

func (s *server) handleFiles(w http.ResponseWriter, r *http.Request) {
	files, err := listFiles(s.uploadDir)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, files)
}

// handleUpload は複数ファイルを受け取り、1ファイルずつストリーミングで保存する（メモリに全体を載せない）。
func (s *server) handleUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "POST で送信してください")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, s.cfg.MaxUploadMB<<20)

	mr, err := r.MultipartReader()
	if err != nil {
		writeError(w, http.StatusBadRequest, "送信データの形式が正しくありません")
		return
	}

	saved := []string{}
	for {
		part, err := mr.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			writeUploadError(w, err)
			return
		}
		if part.FormName() != "files" || part.FileName() == "" {
			_ = part.Close()
			continue
		}
		name, err := s.saveStream(part.FileName(), part)
		_ = part.Close()
		if err != nil {
			writeUploadError(w, err)
			return
		}
		saved = append(saved, name)
	}
	writeJSON(w, http.StatusOK, map[string][]string{"saved": saved})
}

func (s *server) saveStream(filename string, src io.Reader) (string, error) {
	name := sanitizeName(filename)
	if name == "" {
		return "", errors.New("ファイル名が不正です")
	}
	f, finalName, err := createUnique(s.uploadDir, name)
	if err != nil {
		return "", err
	}
	_, copyErr := io.Copy(f, src)
	closeErr := f.Close()
	if copyErr != nil || closeErr != nil {
		_ = os.Remove(filepath.Join(s.uploadDir, finalName))
		if copyErr != nil {
			return "", copyErr
		}
		return "", closeErr
	}
	return finalName, nil
}

func writeUploadError(w http.ResponseWriter, err error) {
	var mbe *http.MaxBytesError
	if errors.As(err, &mbe) {
		writeError(w, http.StatusRequestEntityTooLarge, "ファイルが大きすぎます（config.json の max_upload_mb で上限を変更できます）")
		return
	}
	writeError(w, http.StatusInternalServerError, "保存に失敗しました: "+err.Error())
}

// handleText は貼り付けられたテキストを .txt ファイルとして保存する。
func (s *server) handleText(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "POST で送信してください")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	if err := r.ParseForm(); err != nil {
		writeError(w, http.StatusBadRequest, "テキストが大きすぎるか、形式が正しくありません")
		return
	}
	text := r.PostForm.Get("text")
	if strings.TrimSpace(text) == "" {
		writeError(w, http.StatusBadRequest, "テキストが空です")
		return
	}

	name := "text-" + time.Now().Format("20060102-150405") + ".txt"
	f, finalName, err := createUnique(s.uploadDir, name)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	_, werr := f.WriteString(text)
	cerr := f.Close()
	if werr != nil || cerr != nil {
		_ = os.Remove(filepath.Join(s.uploadDir, finalName))
		writeError(w, http.StatusInternalServerError, "保存に失敗しました")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"saved": finalName})
}

func (s *server) handleDownload(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/download/")
	if !validName(name) {
		http.NotFound(w, r)
		return
	}
	f, err := os.Open(filepath.Join(s.uploadDir, name))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()

	st, err := f.Stat()
	if err != nil || st.IsDir() {
		http.NotFound(w, r)
		return
	}

	// 送られたファイルを画面上で実行させないよう、常にダウンロード扱いにする
	cd := mime.FormatMediaType("attachment", map[string]string{"filename": name})
	if cd == "" {
		cd = "attachment"
	}
	w.Header().Set("Content-Disposition", cd)
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	http.ServeContent(w, r, name, st.ModTime(), f)
}

func (s *server) handleDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "POST で送信してください")
		return
	}
	if err := r.ParseForm(); err != nil {
		writeError(w, http.StatusBadRequest, "送信データの形式が正しくありません")
		return
	}
	name := r.PostForm.Get("name")
	if !validName(name) {
		writeError(w, http.StatusBadRequest, "ファイル名が不正です")
		return
	}
	if err := os.Remove(filepath.Join(s.uploadDir, name)); err != nil {
		if os.IsNotExist(err) {
			writeError(w, http.StatusNotFound, "ファイルが見つかりません")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *server) handleHotspotStatus(w http.ResponseWriter, r *http.Request) {
	s.hsMu.Lock()
	defer s.hsMu.Unlock()
	st, err := s.hotspot.Status()
	if err != nil {
		st.Message = err.Error()
	}
	writeJSON(w, http.StatusOK, st)
}

func (s *server) handleHotspotChange(start bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "POST で送信してください")
			return
		}
		s.hsMu.Lock()
		defer s.hsMu.Unlock()

		var err error
		if start {
			err = s.hotspot.Start()
		} else {
			err = s.hotspot.Stop()
		}
		st, _ := s.hotspot.Status()
		if err != nil {
			log.Printf("アクセスポイントの操作に失敗しました: %v", err)
			writeJSON(w, http.StatusInternalServerError, map[string]any{
				"error":     err.Error(),
				"supported": st.Supported,
				"active":    st.Active,
			})
			return
		}
		writeJSON(w, http.StatusOK, st)
	}
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}
