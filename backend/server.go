package main

import (
	"embed"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"strings"
)

//go:embed all:dist
var frontendFS embed.FS

// startHTTPServer 启动 HTTP 服务，托管前端静态文件 + WebSocket 路由
func startHTTPServer(port int, hub *Hub) *http.Server {
	distFS, err := fs.Sub(frontendFS, "dist")
	if err != nil {
		log.Fatalf("无法加载前端文件: %v", err)
	}

	mux := http.NewServeMux()

	// WebSocket 路由
	mux.HandleFunc("/ws", hub.ServeWS)

	// 静态文件 + SPA fallback
	fileServer := http.FileServer(http.FS(distFS))
	mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		if _, err := fs.Stat(distFS, path); err == nil {
			fileServer.ServeHTTP(w, r)
			return
		}
		// SPA fallback — 所有非文件路径返回 index.html
		index, err := fs.ReadFile(distFS, "index.html")
		if err != nil {
			http.Error(w, "Not Found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(index)
	}))

	addr := fmt.Sprintf("127.0.0.1:%d", port)
	server := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	go func() {
		log.Printf("HTTP 服务已启动: http://%s", addr)
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			log.Fatalf("HTTP 服务异常: %v", err)
		}
	}()

	return server
}

// bindPort 绑定端口，从 startPort 开始尝试，被占用则 +1
func bindPort(startPort int) int {
	for port := startPort; port < startPort+10; port++ {
		addr := fmt.Sprintf("127.0.0.1:%d", port)
		ln, err := net.Listen("tcp", addr)
		if err == nil {
			ln.Close()
			return port
		}
	}
	log.Fatalf("无法绑定端口 (尝试 %d-%d)", startPort, startPort+9)
	return 0
}
