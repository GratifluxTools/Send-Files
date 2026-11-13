package main

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/big"
	"os"
	"path/filepath"
)

// Config は config.json の内容。初回起動時に自動作成される。
type Config struct {
	Port             int    `json:"port"`
	Username         string `json:"username"`
	Password         string `json:"password"`
	MaxUploadMB      int64  `json:"max_upload_mb"`
	UploadDir        string `json:"upload_dir"`
	HotspotSSID      string `json:"hotspot_ssid"`     // Linux で使用（Windows は OS の設定を使う）
	HotspotPassword  string `json:"hotspot_password"` // Linux で使用（8文字以上）
	AutoStartHotspot bool   `json:"auto_start_hotspot"`
}

func defaultConfig() *Config {
	return &Config{
		Port:        8080,
		Username:    "admin",
		MaxUploadMB: 4096,
		UploadDir:   "uploads",
		HotspotSSID: "SendFiles",
	}
}

// dataDir は実行ファイルと同じフォルダを返す。
// ダブルクリックで起動しても、設定や保存先の場所がずれないようにするため。
func dataDir() string {
	exe, err := os.Executable()
	if err != nil {
		wd, _ := os.Getwd()
		return wd
	}
	return filepath.Dir(exe)
}

// loadConfig は設定を読み込む。ファイルがなければ作成する。
// 戻り値の bool は、このとき新規作成したかどうか。
func loadConfig(path string) (*Config, bool, error) {
	cfg := defaultConfig()

	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		cfg.Password = randomString(8)
		cfg.HotspotPassword = randomString(12)
		if err := cfg.save(path); err != nil {
			return nil, false, err
		}
		return cfg, true, nil
	}
	if err != nil {
		return nil, false, err
	}

	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, false, fmt.Errorf("%s の形式が正しくありません: %w", path, err)
	}

	changed := false
	if cfg.Password == "" {
		cfg.Password = randomString(8)
		changed = true
	}
	if cfg.HotspotPassword == "" {
		cfg.HotspotPassword = randomString(12)
		changed = true
	}
	if cfg.MaxUploadMB <= 0 {
		cfg.MaxUploadMB = defaultConfig().MaxUploadMB
		changed = true
	}
	if changed {
		if err := cfg.save(path); err != nil {
			return nil, false, err
		}
	}
	return cfg, false, nil
}

func (c *Config) save(path string) error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

// randomString は紛らわしい文字（0/o/1/l/i など）を除いた英数字をランダムに返す。
func randomString(n int) string {
	const chars = "abcdefghjkmnpqrstuvwxyz23456789"
	out := make([]byte, n)
	for i := range out {
		v, err := rand.Int(rand.Reader, big.NewInt(int64(len(chars))))
		if err != nil {
			log.Fatalf("乱数の生成に失敗しました: %v", err)
		}
		out[i] = chars[v.Int64()]
	}
	return string(out)
}
