//go:build windows

package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"syscall"
	"unicode/utf16"
)

// windowsHotspot は Windows のモバイルホットスポットを、PowerShell から
// WinRT API（NetworkOperatorTetheringManager）で操作する。管理者権限は不要。
// SSID とパスワードは、Windows の「モバイル ホットスポット」の設定をそのまま使う。
type windowsHotspot struct{}

func newHotspot(_ *Config) Hotspot { return windowsHotspot{} }

type psResult struct {
	State  int    `json:"state"` // 2 = On
	SSID   string `json:"ssid"`
	Pass   string `json:"pass"`
	Status string `json:"status"`
	Error  string `json:"error"`
}

// psPrelude は WinRT の非同期 API を PowerShell で待つための準備。
// Windows PowerShell 5.1 で動く定番の書き方。
const psPrelude = `
$ErrorActionPreference = 'Stop'
Add-Type -AssemblyName System.Runtime.WindowsRuntime
$asTaskGeneric = ([System.WindowsRuntimeSystemExtensions].GetMethods() | Where-Object { $_.Name -eq 'AsTask' -and $_.GetParameters().Count -eq 1 -and $_.GetParameters()[0].ParameterType.Name -match '^IAsyncOperation\W' })[0]
function Await($op, $t) {
  $task = $asTaskGeneric.MakeGenericMethod($t).Invoke($null, @($op))
  if (-not $task.Wait(30000)) { throw 'timeout' }
  return $task.Result
}
[void][Windows.Networking.Connectivity.NetworkInformation, Windows.Networking.Connectivity, ContentType = WindowsRuntime]
[void][Windows.Networking.NetworkOperators.NetworkOperatorTetheringManager, Windows.Networking.NetworkOperators, ContentType = WindowsRuntime]
$conn = [Windows.Networking.Connectivity.NetworkInformation]::GetInternetConnectionProfile()
if ($null -eq $conn) { throw 'no internet connection profile' }
$tm = [Windows.Networking.NetworkOperators.NetworkOperatorTetheringManager]::CreateFromConnectionProfile($conn)
`

var psActions = map[string]string{
	"status": `
$r = @{ state = [int]$tm.TetheringOperationalState; ssid = ''; pass = ''; status = ''; error = '' }
$cfg = $tm.GetCurrentAccessPointConfiguration()
$r.ssid = [string]$cfg.Ssid
$r.pass = [string]$cfg.Passphrase
$r | ConvertTo-Json -Compress`,
	"start": `
$res = Await ($tm.StartTetheringAsync()) ([Windows.Networking.NetworkOperators.NetworkOperatorTetheringOperationResult])
@{ state = [int]$tm.TetheringOperationalState; ssid = ''; pass = ''; status = [string]$res.Status; error = '' } | ConvertTo-Json -Compress`,
	"stop": `
$res = Await ($tm.StopTetheringAsync()) ([Windows.Networking.NetworkOperators.NetworkOperatorTetheringOperationResult])
@{ state = [int]$tm.TetheringOperationalState; ssid = ''; pass = ''; status = [string]$res.Status; error = '' } | ConvertTo-Json -Compress`,
}

// runPS は PowerShell スクリプトを実行し、最後に出力された JSON を読む。
// -EncodedCommand を使うため、引用符の扱いで壊れることがない。
func runPS(action string) (psResult, error) {
	body, ok := psActions[action]
	if !ok {
		return psResult{}, fmt.Errorf("不明な操作です: %s", action)
	}
	script := "try {" + psPrelude + body + "\n} catch {\n@{ error = $_.Exception.Message } | ConvertTo-Json -Compress\n}\n"

	cmd := exec.Command("powershell.exe",
		"-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass",
		"-EncodedCommand", encodeCommand(script),
	)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return psResult{}, fmt.Errorf("PowerShell の実行に失敗しました: %v %s", err, strings.TrimSpace(stderr.String()))
	}

	var res psResult
	line := lastJSONLine(stdout.String())
	if err := json.Unmarshal([]byte(line), &res); err != nil {
		return psResult{}, fmt.Errorf("PowerShell の出力を読み取れません: %q", stdout.String())
	}
	return res, nil
}

func encodeCommand(script string) string {
	units := utf16.Encode([]rune(script))
	buf := make([]byte, 0, len(units)*2)
	for _, u := range units {
		buf = append(buf, byte(u), byte(u>>8))
	}
	return base64.StdEncoding.EncodeToString(buf)
}

func lastJSONLine(out string) string {
	lines := strings.Split(strings.TrimSpace(out), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if strings.HasPrefix(line, "{") {
			return line
		}
	}
	return ""
}

func (windowsHotspot) Status() (HotspotStatus, error) {
	res, err := runPS("status")
	if err != nil {
		return HotspotStatus{Supported: true, Message: err.Error()}, nil
	}
	if res.Error != "" {
		return HotspotStatus{Supported: true, Message: res.Error}, nil
	}
	return HotspotStatus{
		Supported: true,
		Active:    res.State == 2,
		SSID:      res.SSID,
		Password:  res.Pass,
	}, nil
}

func (windowsHotspot) Start() error { return windowsChange("start") }

func (windowsHotspot) Stop() error { return windowsChange("stop") }

func windowsChange(action string) error {
	res, err := runPS(action)
	if err != nil {
		return err
	}
	if res.Error != "" {
		return fmt.Errorf("ホットスポットの操作に失敗しました: %s", res.Error)
	}
	if res.Status != "Success" {
		return fmt.Errorf("ホットスポットの操作に失敗しました（結果: %s）", res.Status)
	}
	return nil
}
