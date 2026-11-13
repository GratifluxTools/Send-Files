package main

import (
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"
)

type fileInfo struct {
	Name     string `json:"name"`
	Size     int64  `json:"size"`
	Modified int64  `json:"modified"` // Unix 秒
}

// Windows で使えない文字は「_」に置き換える（iPad 側で付いた名前でも PC で保存できるように）。
var reservedChars = strings.NewReplacer(`<`, "_", `>`, "_", `:`, "_", `"`, "_", `|`, "_", `?`, "_", `*`, "_")

// sanitizeName は送信元の名前から、安全な保存用のファイル名を作る。空なら不正。
func sanitizeName(name string) string {
	name = strings.ReplaceAll(name, "\\", "/")
	name = path.Base(name) // フォルダ部分を取り除く（../ などの対策）
	name = strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, name)
	name = strings.TrimSpace(reservedChars.Replace(name))
	if name == "" || name == "." || name == ".." || name == "/" {
		return ""
	}
	return truncateName(name, 200)
}

// truncateName は拡張子を残したまま、バイト数の上限内に収める。
func truncateName(name string, max int) string {
	if len(name) <= max {
		return name
	}
	ext := path.Ext(name)
	if len(ext) > 20 {
		ext = ""
	}
	base := name[:len(name)-len(ext)]
	limit := max - len(ext)
	for len(base) > limit {
		_, size := utf8.DecodeLastRuneInString(base)
		base = base[:len(base)-size]
	}
	return base + ext
}

// validName は、URL などから受け取った名前が保存フォルダ直下のファイルとして安全かを調べる。
func validName(name string) bool {
	return name != "" && name != "." && name != ".." && len(name) <= 255 && !strings.ContainsAny(name, `/\`)
}

// createUnique は同じ名前があれば「名前 (2).ext」のように連番を付けて、新規ファイルを作る。
// O_EXCL で作成するため、同時アップロードでも上書きされない。
func createUnique(dir, name string) (*os.File, string, error) {
	ext := path.Ext(name)
	base := strings.TrimSuffix(name, ext)
	for i := 1; i <= 1000; i++ {
		candidate := name
		if i > 1 {
			candidate = fmt.Sprintf("%s (%d)%s", base, i, ext)
		}
		f, err := os.OpenFile(filepath.Join(dir, candidate), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if err == nil {
			return f, candidate, nil
		}
		if !os.IsExist(err) {
			return nil, "", err
		}
	}
	return nil, "", errors.New("同じ名前のファイルが多すぎます")
}

// listFiles は保存フォルダ内のファイルを新しい順に返す。
func listFiles(dir string) ([]fileInfo, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	out := make([]fileInfo, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		out = append(out, fileInfo{Name: e.Name(), Size: info.Size(), Modified: info.ModTime().Unix()})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Modified > out[j].Modified })
	return out, nil
}
