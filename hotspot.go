package main

// HotspotStatus はアクセスポイントの状態。画面表示と API の応答に使う。
type HotspotStatus struct {
	Supported bool   `json:"supported"`
	Active    bool   `json:"active"`
	SSID      string `json:"ssid"`
	Password  string `json:"password"`
	Message   string `json:"message"`
}

// Hotspot は OS ごとのアクセスポイント操作を抽象化する。
// 実装は hotspot_windows.go / hotspot_linux.go / hotspot_other.go にある。
type Hotspot interface {
	Status() (HotspotStatus, error)
	Start() error
	Stop() error
}
