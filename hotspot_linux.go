//go:build linux

package main

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// NetworkManager の接続プロファイル名。このアプリが作るホットスポット専用。
const linuxConnName = "SendFiles-Hotspot"

// linuxHotspot は NetworkManager（nmcli）でホットスポットを操作する。
// 必要なもの: NetworkManager、AP モードに対応した Wi-Fi アダプター。
type linuxHotspot struct {
	cfg *Config
}

func newHotspot(cfg *Config) Hotspot { return linuxHotspot{cfg: cfg} }

// nmcli は NetworkManager のコマンドを実行する。
// パスワードを含む引数はエラーメッセージに出さない。
func nmcli(args ...string) (string, error) {
	out, err := exec.Command("nmcli", args...).CombinedOutput()
	text := strings.TrimSpace(string(out))
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return "", errors.New("nmcli が見つかりません（NetworkManager をインストールしてください）")
		}
		return text, fmt.Errorf("nmcli の実行に失敗しました: %s", text)
	}
	return text, nil
}

func wifiDevice() (string, error) {
	out, err := nmcli("-t", "-f", "DEVICE,TYPE", "device")
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(out, "\n") {
		fields := strings.SplitN(line, ":", 2)
		if len(fields) == 2 && fields[1] == "wifi" {
			return fields[0], nil
		}
	}
	return "", errors.New("Wi-Fi デバイスが見つかりません")
}

func (h linuxHotspot) isActive() (bool, error) {
	out, err := nmcli("-t", "-f", "NAME", "connection", "show", "--active")
	if err != nil {
		return false, err
	}
	for _, line := range strings.Split(out, "\n") {
		if strings.TrimSpace(line) == linuxConnName {
			return true, nil
		}
	}
	return false, nil
}

func (h linuxHotspot) Status() (HotspotStatus, error) {
	st := HotspotStatus{
		Supported: true,
		SSID:      h.cfg.HotspotSSID,
		Password:  h.cfg.HotspotPassword,
	}
	active, err := h.isActive()
	if err != nil {
		st.Message = err.Error()
		return st, nil
	}
	st.Active = active
	return st, nil
}

func (h linuxHotspot) Start() error {
	if len(h.cfg.HotspotPassword) < 8 {
		return errors.New("config.json の hotspot_password は 8 文字以上にしてください")
	}
	dev, err := wifiDevice()
	if err != nil {
		return err
	}
	// 前回の設定が残らないよう、既存のプロファイルは作り直す（存在しなくても無視する）
	_, _ = nmcli("connection", "delete", linuxConnName)
	_, err = nmcli("device", "wifi", "hotspot",
		"ifname", dev,
		"con-name", linuxConnName,
		"ssid", h.cfg.HotspotSSID,
		"password", h.cfg.HotspotPassword,
	)
	return err
}

func (h linuxHotspot) Stop() error {
	active, err := h.isActive()
	if err != nil {
		return err
	}
	if !active {
		return nil
	}
	_, err = nmcli("connection", "down", linuxConnName)
	return err
}
