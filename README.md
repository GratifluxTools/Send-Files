# Send Files

[日本語](#日本語) | [English](#english)

---

## 日本語

PC と iPad などの端末の間で、ブラウザを使ってファイルとテキストを送受信する小さな Web サーバーアプリです。
Wi-Fi がない場所でも、PC の**モバイルホットスポット**（Windows）や**アクセスポイント**（Linux）を使って接続できます。
インストールや Python は不要で、単一の実行ファイルで動きます。

対応環境：**Windows**（すべての機能）、**Linux**（NetworkManager が動いている環境のみ、アクセスポイント機能を含む）

### 機能
- **双方向**：PC → iPad、iPad → PC のどちらも可能
- **複数ファイル送信**：ドラッグ＆ドロップにも対応。送信の進捗バーつき
- **テキスト送信**：貼り付けたテキストを `.txt` ファイルとして保存
- **アクセスポイントの ON/OFF を画面から切り替え**（SSID・パスワードも表示）
- **自動更新**：共有ファイル一覧は 5 秒ごとに更新
- **同名ファイルは自動で連番**：`写真 (2).jpg` のように、上書きされません
- **削除**：一覧から削除できます（確認ダイアログあり）
- **ダウンロードは常に添付扱い**：送られたファイルがブラウザ上で実行されることはありません

### インストール

**Windows**
1. [Releases](https://github.com/OWNER/send-files/releases) から `send-files-windows.zip` をダウンロードして展開します。
2. `send-files.exe` を起動します。初回起動時に `config.json` が作られ、ユーザー名とパスワードが表示されます。
3. 初回起動時に Windows ファイアウォールの許可を求められたら、**プライベートネットワーク**で許可してください。

**Linux**
1. `send-files-linux.zip` をダウンロードして展開します。
2. 実行権限を付けて起動します：
   ```bash
   chmod +x send-files
   ./send-files
   ```

### 使い方
1. **PC でアクセスポイントを ON にする**
   - Windows：「設定」→「ネットワークとインターネット」→「モバイル ホットスポット」を ON にする、または画面の「開始」ボタンを押す
   - Linux：画面の「開始」ボタンを押す（または `config.json` の `auto_start_hotspot` を `true` にする）
2. **iPad を PC のアクセスポイントに接続する**（SSID とパスワードは画面に表示されます）
3. **iPad のブラウザで、画面に表示された URL を開く**（例：`http://192.168.137.1:8080`）
4. **ユーザー名とパスワードを入力する**（起動時にコンソールに表示されたもの）
5. 「ファイルを選択」またはドラッグ＆ドロップで送信。ファイル名をタップするとダウンロードされます。

アクセスポイントを使わず、同じ Wi-Fi に PC と iPad をつないで使うこともできます。その場合は PC の IP アドレスの URL を開いてください。

### 設定（config.json）

実行ファイルと同じフォルダに作られます。編集後はアプリを再起動してください。

| キー | 既定値 | 内容 |
|---|---|---|
| `port` | `8080` | 待ち受けポート |
| `username` / `password` | `admin` / 自動生成 | 画面にログインするための認証情報 |
| `max_upload_mb` | `4096` | 1 回の送信の上限（MB） |
| `upload_dir` | `uploads` | 保存先フォルダ（相対パスは実行ファイルの場所基準） |
| `hotspot_ssid` | `SendFiles` | Linux のアクセスポイント名（Windows は OS の設定を使用） |
| `hotspot_password` | 自動生成（12 文字） | Linux のアクセスポイントのパスワード（8 文字以上） |
| `auto_start_hotspot` | `false` | 起動時にアクセスポイントを自動で開始する。終了時には、このアプリが開始したものだけ停止します。 |

### 注意事項

- **通信は暗号化されません（HTTP）**。信頼できるネットワーク（PC が作ったアクセスポイントなど）でのみ使ってください。公共の Wi-Fi やインターネット越しには使わないでください。
- 既定のパスワードは初回起動時にランダムに生成されます。変更したい場合は `config.json` を編集してください。
- Windows でアクセスポイントに接続できない場合は、ファイアウォールの設定で「パブリック」ネットワークの受信が遮断されていないか確認してください。
- 旧バージョンから移行した場合、削除機能は `GET /delete/...` から `POST /api/delete` に変わりました。古いブックマークは使えません。

### トラブルシューティング
- **画面が開かない**：PC とiPadが同じネットワークにいるか、ファイアウォールでポートが許可されているか確認してください。
- **アクセスポイントの開始に失敗する（Windows）**：Wi-Fi アダプターが有効か、「モバイル ホットスポット」がこの PC で使えるか確認してください。
- **アクセスポイントの開始に失敗する（Linux）**：`nmcli` が使えるか、Wi-Fi アダプターが AP モードに対応しているか確認してください。`nmcli device wifi hotspot` を手動で実行するとエラーの詳細が分かります。
- **ポートが使用中と表示される**：`config.json` の `port` を別の番号に変えてください。

### ビルド
```bash
go test ./...
go build -o send-files .
```
リリースは GitHub Actions（`.github/workflows/build.yml`）で作られます。`Build` ワークフローを手動実行すると、テスト用の成果物が作られます。`v0.2.0` のようなタグで Release を公開すると、そのバージョンでビルドされ、zip が Release に添付されます。

### ファイル構成
| ファイル | 役割 |
|---|---|
| `main.go` | 起動処理、設定の読み込み、終了処理 |
| `config.go` | `config.json` の読み書き、初期パスワードの生成 |
| `server.go` | HTTP ルーティング、認証、アップロード・ダウンロード・削除 |
| `files.go` | ファイル名の安全化、同名ファイルの連番、一覧 |
| `network.go` | 接続用 URL の一覧 |
| `hotspot.go` | アクセスポイント操作の共通インターフェース |
| `hotspot_windows.go` | Windows のモバイルホットスポット（PowerShell + WinRT） |
| `hotspot_linux.go` | Linux のアクセスポイント（nmcli / NetworkManager） |
| `hotspot_other.go` | 上記以外の OS（非対応として表示） |
| `app_version.go` | バージョン（リリース時にタグから書き換え） |
| `template/index.html` | 画面（HTML / CSS / JavaScript） |

### ライセンス
（未設定：必要に応じて LICENSE を追加してください）

---

## English

A small web server that lets you send files and text between a PC and a phone or tablet (such as an iPad) through a browser.
You can connect even where there is no Wi-Fi, using the PC's **Mobile Hotspot** (Windows) or **access point** (Linux).
No installation or Python is needed. It runs as a single executable.

Supported: **Windows** (all features) and **Linux** (requires NetworkManager for the access point feature).

### Features
- Two-way transfer: PC → iPad and iPad → PC
- Multiple files at once, with drag & drop and a progress bar
- Text sending: pasted text is saved as a `.txt` file
- Turn the access point on/off from the page (the SSID and password are shown)
- The shared file list refreshes every 5 seconds
- Duplicate names get numbered (`photo (2).jpg`), so nothing is overwritten
- Downloads are always sent as attachments, so files never run in the browser

### Quick start
1. Turn on the access point on the PC (Windows: Mobile hotspot; Linux: press Start on the page).
2. Connect the iPad to the access point using the SSID and password shown on the page.
3. Open the URL shown on the page in the iPad's browser, for example `http://192.168.137.1:8080`.
4. Log in with the username and password printed in the console at first launch (also stored in `config.json`).

### Configuration
`config.json` is created next to the executable on first launch. Restart the app after editing it.
Keys: `port`, `username`, `password`, `max_upload_mb`, `upload_dir`, `hotspot_ssid`, `hotspot_password` (8+ characters, Linux only), `auto_start_hotspot`.

### Security notes
- Traffic is **not encrypted (plain HTTP)**. Use it only on a trusted network such as the access point this PC creates. Do not use it over public Wi-Fi or the internet.
- Passwords are generated randomly on first launch.

### Build
```bash
go test ./...
go build -o send-files .
```
Releases are built by GitHub Actions (`.github/workflows/build.yml`). Publishing a Release with a tag such as `v0.2.0` attaches the Windows and Linux zip files to it.
