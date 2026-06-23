package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/grandcat/zeroconf"
)

const mdnsServiceType = "_sharelan._tcp"
const mdnsDomain = "local."

var hostname string

func init() {
	var err error
	hostname, err = os.Hostname()
	if err != nil {
		hostname = "unknown"
	}
	hostname = strings.TrimSuffix(hostname, ".local")
}

// DeviceInfo 描述一个发现的设备
type DeviceInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	IP   string `json:"ip"`
	Port int    `json:"port"`
}

// MDNSService 管理 mDNS 广播和发现
type MDNSService struct {
	server  *zeroconf.Server
	devices map[string]*DeviceInfo
	mu      sync.RWMutex
	onFound func(DeviceInfo)
	onLost  func(string)
	ctx     context.Context
	cancel  context.CancelFunc
	selfID  string // 本机设备 ID，用于过滤自己
}

// startMDNS 启动 mDNS 广播和发现
func startMDNS(deviceID string, port int,
	onFound func(DeviceInfo),
	onLost func(string),
) (*MDNSService, error) {

	ctx, cancel := context.WithCancel(context.Background())

	s := &MDNSService{
		devices: make(map[string]*DeviceInfo),
		onFound: onFound,
		onLost:  onLost,
		ctx:     ctx,
		cancel:  cancel,
		selfID:  deviceID,
	}

	// 注册 mDNS 服务
	server, err := zeroconf.Register(
		deviceID,
		mdnsServiceType,
		mdnsDomain,
		port,
		[]string{
			"id=" + deviceID,
			"name=" + hostname,
			fmt.Sprintf("port=%d", port),
		},
		nil,
	)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("mDNS 注册失败: %w", err)
	}
	s.server = server

	localIP := getLocalIP()
	log.Printf("mDNS 广播已启动: %s (%s) 端口 %d", hostname, localIP, port)

	// 启动设备发现
	go s.discover()

	return s, nil
}

// reAnnounce 端口变化时重新广播（含 debounce）
func (s *MDNSService) reAnnounce(deviceID string, port int) {
	if s.server != nil {
		s.server.Shutdown()
	}

	server, err := zeroconf.Register(
		deviceID,
		mdnsServiceType,
		mdnsDomain,
		port,
		[]string{
			"id=" + deviceID,
			"name=" + hostname,
			fmt.Sprintf("port=%d", port),
		},
		nil,
	)
	if err != nil {
		log.Printf("mDNS reannounce 失败: %v", err)
		return
	}
	s.server = server
	log.Printf("mDNS 已重新广播，新端口: %d", port)
}

func (s *MDNSService) discover() {
	resolver, err := zeroconf.NewResolver(nil)
	if err != nil {
		log.Fatalf("mDNS 解析器创建失败: %v", err)
	}

	entries := make(chan *zeroconf.ServiceEntry)
	go func() {
		for entry := range entries {
			s.handleEntry(entry)
		}
	}()

	err = resolver.Browse(s.ctx, mdnsServiceType, mdnsDomain, entries)
	if err != nil {
		log.Printf("mDNS 浏览失败: %v", err)
	}
}

func (s *MDNSService) handleEntry(entry *zeroconf.ServiceEntry) {
	id := ""
	name := ""
	port := entry.Port

	for _, txt := range entry.Text {
		parts := strings.SplitN(txt, "=", 2)
		if len(parts) != 2 {
			continue
		}
		switch parts[0] {
		case "id":
			id = parts[1]
		case "name":
			name = parts[1]
		}
	}

	if id == "" || len(entry.AddrIPv4) == 0 {
		return
	}

	// 不添加自己（跟本机 deviceID 比较）
	if id == s.selfID {
		return
	}

	log.Printf("mDNS 收到设备: instance=%s id=%s name=%s ip=%v port=%d",
		entry.Instance, id, name, entry.AddrIPv4, entry.Port)

	ip := pickLANIP(entry.AddrIPv4)
	if ip == "" {
		return // 没有找到有效的局域网 IP
	}

	s.mu.Lock()
	existing, exists := s.devices[id]
	if !exists || existing.Port != port || existing.IP != ip {
		s.devices[id] = &DeviceInfo{ID: id, Name: name, IP: ip, Port: port}
		s.mu.Unlock()
		if s.onFound != nil {
			s.onFound(DeviceInfo{ID: id, Name: name, IP: ip, Port: port})
		}
	} else {
		s.mu.Unlock()
	}
}

// Stop 停止 mDNS 服务
func (s *MDNSService) Stop() {
	s.cancel()
	if s.server != nil {
		s.server.Shutdown()
	}
}

// 定期清理超时设备（mDNS 本身不提供心跳，备用兜底）
func (s *MDNSService) startCleanupLoop(timeout time.Duration) {
	ticker := time.NewTicker(timeout)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			// 实际下线检测依赖 WS 断开事件 + ping/pong
			// mDNS 只负责发现，不负责心跳
		}
	}
}

// pickLANIP 从多个 IP 中选取最合适的局域网 IP
// 优先顺序：192.168.x.x > 10.x.x.x > 172.16-31.x.x > 其他私有 IP
func pickLANIP(ips []net.IP) string {
	// 先看有没有 192.168.x.x
	for _, ip := range ips {
		if ip.To4() != nil && ip[0] == 192 && ip[1] == 168 {
			return ip.String()
		}
	}
	// 再看 10.x.x.x
	for _, ip := range ips {
		if ip.To4() != nil && ip[0] == 10 {
			return ip.String()
		}
	}
	// 再看 172.16-31.x.x
	for _, ip := range ips {
		if ip.To4() != nil && ip[0] == 172 && ip[1] >= 16 && ip[1] <= 31 {
			return ip.String()
		}
	}
	// 最后取第一个非回环 IPv4
	for _, ip := range ips {
		if ip.To4() != nil && !ip.IsLoopback() {
			return ip.String()
		}
	}
	return ""
}

func getLocalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "127.0.0.1"
	}
	var ips []net.IP
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				ips = append(ips, ipnet.IP)
			}
		}
	}
	// 复用 pickLANIP 优先选局域网 IP
	if ip := pickLANIP(ips); ip != "" {
		return ip
	}
	if len(ips) > 0 {
		return ips[0].String()
	}
	return "127.0.0.1"
}
