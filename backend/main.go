package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/webview/webview_go"
)

// Config 本地配置
type Config struct {
	DeviceID string `json:"device_id"`
}

// loadOrGenerateDeviceID 加载或生成设备 ID（持久化到 ~/.sharelan/config.json）
func loadOrGenerateDeviceID() string {
	dir := filepath.Join(os.Getenv("HOME"), ".sharelan")
	os.MkdirAll(dir, 0755)
	cfgPath := filepath.Join(dir, "config.json")

	var cfg Config
	data, err := os.ReadFile(cfgPath)
	if err == nil {
		json.Unmarshal(data, &cfg)
		if cfg.DeviceID != "" {
			return cfg.DeviceID
		}
	}

	cfg.DeviceID = newUUID()
	data, _ = json.Marshal(cfg)
	os.WriteFile(cfgPath, data, 0644)
	return cfg.DeviceID
}

// logBuffer 全局日志缓冲区（临时调试用）
var logBuffer *LogBuffer

func main() {
	log.SetFlags(log.Ltime | log.Lshortfile)

	// 全局日志缓冲，最后 500KB，同时写入 stderr
	logBuffer = NewLogBuffer(500*1024, os.Stderr)
	log.SetOutput(logBuffer)

	// 1. 加载设备 ID
	deviceID := loadOrGenerateDeviceID()
	log.Printf("设备 ID: %s", deviceID)
	log.Printf("主机名: %s", hostname)

	// 2. 初始化数据库
	db, err := initDB()
	if err != nil {
		log.Fatalf("数据库初始化失败: %v", err)
	}
	defer db.Close()
	log.Println("数据库已初始化")

	// 3. 绑定端口
	port := bindPort(17888)
	log.Printf("端口已绑定: %d", port)

	// 4. 创建 Hub
	hub := newHub(db, deviceID, hostname, port)

	// 5. Hub 启动心跳
	go hub.startPingLoop()

	// 6. 启动 HTTP 服务
	httpServer := startHTTPServer(port, hub)
	defer httpServer.Close()

	// 7. 启动 mDNS
	mdns, err := startMDNS(deviceID, port,
		// onFound: 新设备发现时发起 WS 连接 + 推送到前端
		func(d DeviceInfo) {
			log.Printf("发现设备: %s (%s:%d)", d.Name, d.IP, d.Port)
			hub.pushDeviceOnline(d.ID, d.Name, d.IP, d.Port)
			hub.ConnectToPeer(d.ID, d.IP, d.Port)
		},
		// onLost: 设备下线
		func(deviceID string) {
			log.Printf("设备下线: %s", deviceID)
			hub.pushDeviceOffline(deviceID)
		},
	)
	if err != nil {
		log.Fatalf("mDNS 启动失败: %v", err)
	}
	defer mdns.Stop()
	log.Println("mDNS 服务已启动")

	// 8. 打开 WebView
	time.Sleep(200 * time.Millisecond)
	log.Printf("正在打开 WebView: http://127.0.0.1:%d", port)

	w := webview.New(true)
	defer w.Destroy()
	w.SetTitle("ShareLan")
	w.SetSize(900, 640, webview.HintNone)
	w.Navigate(fmt.Sprintf("http://127.0.0.1:%d", port))
	w.Run()
}
