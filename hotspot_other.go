//go:build !windows && !linux

package main

import "errors"

var errUnsupportedOS = errors.New("このOSではアクセスポイントの切り替えに対応していません")

type unsupportedHotspot struct{}

func newHotspot(_ *Config) Hotspot { return unsupportedHotspot{} }

func (unsupportedHotspot) Status() (HotspotStatus, error) {
	return HotspotStatus{Supported: false, Message: errUnsupportedOS.Error()}, nil
}

func (unsupportedHotspot) Start() error { return errUnsupportedOS }

func (unsupportedHotspot) Stop() error { return errUnsupportedOS }
