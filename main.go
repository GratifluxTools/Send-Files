package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"time"
)

func main() {
	dir := dataDir()
	cfgPath := filepath.Join(dir, "config.json")

	cfg, created, err := loadConfig(cfgPath)
	if err != nil {
		log.Fatalf("設定ファイルの読み込みに失敗しました: %v", err)
	}

	uploadDir := cfg.UploadDir
	if !filepath.IsAbs(uploadDir) {
		uploadDir = filepath.Join(dir, uploadDir)
	}
	if err := os.MkdirAll(uploadDir, 0o755); err != nil {
		log.Fatalf("保存先フォルダを作成できません: %v", err)
	}

	hs := newHotspot(cfg)
	srv := newServer(cfg, uploadDir, hs)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	// 設定で自動開始する場合。終了時には、このアプリが開始したものだけ停止する。
	startedHotspot := false
	if cfg.AutoStartHotspot {
		if err := hs.Start(); err != nil {
			log.Printf("アクセスポイントを自動開始できませんでした: %v", err)
		} else {
			startedHotspot = true
		}
	}

	printBanner(cfg, created, cfgPath, uploadDir)

	httpSrv := &http.Server{
		Addr:              fmt.Sprintf("0.0.0.0:%d", cfg.Port),
		Handler:           srv.routes(),
		ReadHeaderTimeout: 10 * time.Second,
	}
	errCh := make(chan error, 1)
	go func() { errCh <- httpSrv.ListenAndServe() }()

	select {
	case err := <-errCh:
		if !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("サーバーを起動できません（ポート %d が使用中の可能性があります）: %v", cfg.Port, err)
		}
	case <-ctx.Done():
		fmt.Println("\n終了しています…")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = httpSrv.Shutdown(shutdownCtx)

	if startedHotspot {
		if err := hs.Stop(); err != nil {
			log.Printf("アクセスポイントの停止に失敗しました: %v", err)
		}
	}
}

func printBanner(cfg *Config, created bool, cfgPath, uploadDir string) {
	fmt.Println("==================================================")
	fmt.Printf("Send Files v%s\n", appVersion)
	if created {
		fmt.Printf("設定ファイルを作成しました: %s\n", cfgPath)
	}
	fmt.Printf("ユーザー名: %s\n", cfg.Username)
	fmt.Printf("パスワード: %s\n", cfg.Password)
	fmt.Printf("保存先フォルダ: %s\n", uploadDir)
	fmt.Println("iPad などのブラウザで次の URL を開いてください:")
	for _, a := range localAddrs(cfg.Port) {
		fmt.Println("  " + a)
	}
	fmt.Println("（アクセスポイントを使う場合は、PC側で ON にしてから接続してください）")
	fmt.Println("==================================================")
}
