package main

import (
	"fmt"
	"net"
	"strings"
)

// localAddrs は接続用の URL を返す。アクセスポイントの IP を先頭に並べる。
func localAddrs(port int) []string {
	var preferred, others []string

	addrs, err := net.InterfaceAddrs()
	if err == nil {
		for _, a := range addrs {
			ipnet, ok := a.(*net.IPNet)
			if !ok || ipnet.IP.IsLoopback() {
				continue
			}
			ip4 := ipnet.IP.To4()
			if ip4 == nil {
				continue
			}
			u := fmt.Sprintf("http://%s:%d", ip4.String(), port)
			if isHotspotIP(ip4.String()) {
				preferred = append(preferred, u)
			} else {
				others = append(others, u)
			}
		}
	}

	list := append(preferred, others...)
	if len(list) == 0 {
		list = []string{fmt.Sprintf("http://localhost:%d", port)}
	}
	return list
}

// Windows のモバイルホットスポットは 192.168.137.1、Linux（NetworkManager）は 10.42.0.1 が多い。
func isHotspotIP(ip string) bool {
	return strings.HasPrefix(ip, "192.168.137.") || strings.HasPrefix(ip, "10.42.0.")
}
