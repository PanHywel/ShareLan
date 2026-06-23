package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

// 连接三态模型
type connState int

const (
	stateConnecting  connState = iota // 刚刚建立 TCP 连接
	stateHandshaking                  // 正在 handshake 确认
	stateReady                        // handshake 完成，可转发消息
)

const (
	pingInterval  = 30 * time.Second
	pongTimeout   = 10 * time.Second
	maxRetryDelay = 30 * time.Second
	initialRetry  = 3 * time.Second
)

// peerConn 表示到一个远端设备的 WebSocket 连接
type peerConn struct {
	deviceID string
	conn     *websocket.Conn
	state    connState
	mu       sync.Mutex
}

// Hub 管理所有 WebSocket 连接
type Hub struct {
	db         *sql.DB
	deviceID   string
	deviceName string
	localPort  int

	peers   map[string]*peerConn // deviceID -> conn
	peersMu sync.RWMutex

	// 本机前端连接（只保留一个）
	localConn   *websocket.Conn
	localConnMu sync.Mutex
}

// WSMessage 统一消息结构
type WSMessage struct {
	ID        string `json:"id"`
	Type      string `json:"type"`
	From      string `json:"from"`
	To        string `json:"to"`
	Content   string `json:"content,omitempty"`
	Timestamp int64  `json:"timestamp"`
}

func newHub(db *sql.DB, deviceID, deviceName string, localPort int) *Hub {
	return &Hub{
		db:         db,
		deviceID:   deviceID,
		deviceName: deviceName,
		localPort:  localPort,
		peers:      make(map[string]*peerConn),
	}
}

// ─── 前端连接处理 ──────────────────────────────────────────

// ServeWS 处理 WebSocket 升级请求（来自前端或远端设备）
func (h *Hub) ServeWS(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true,
	})
	if err != nil {
		log.Printf("WebSocket 接受失败: %v", err)
		return
	}

	// 读取第一条消息来判断来源
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var msg WSMessage
	if err := wsjson.Read(ctx, conn, &msg); err != nil {
		conn.Close(websocket.StatusNormalClosure, "读取首条消息失败")
		return
	}

	switch msg.Type {
	case "hello":
		// 本机前端连入
		h.localConnMu.Lock()
		if h.localConn != nil {
			h.localConn.Close(websocket.StatusNormalClosure, "新连接接入")
		}
		h.localConn = conn
		h.localConnMu.Unlock()
		log.Printf("前端 WebSocket 已连接 (设备: %s)", msg.From)
		// 推送服务信息（本机 IP）
		go h.pushServerInfo(conn)
		// 推历史消息
		go h.pushHistory(conn, msg.From)
		go h.handleLocalConnection(conn)

	case "handshake":
		// 远端设备连入
		deviceID := msg.From
		log.Printf("远端设备连入: %s (%s)", deviceID, msg.Content)
		h.HandleIncomingConnection(deviceID, conn, &msg)
	}
}

// handleLocalConnection 处理前端发来的消息
func (h *Hub) handleLocalConnection(conn *websocket.Conn) {
	defer func() {
		h.localConnMu.Lock()
		if h.localConn == conn {
			h.localConn = nil
		}
		h.localConnMu.Unlock()
		conn.Close(websocket.StatusNormalClosure, "连接关闭")
		log.Println("前端 WebSocket 已断开")
	}()

	for {
		var msg WSMessage
		if err := wsjson.Read(context.Background(), conn, &msg); err != nil {
			if websocket.CloseStatus(err) == websocket.StatusNormalClosure {
				return
			}
			log.Printf("读取前端消息失败: %v", err)
			return
		}

		if msg.Type == "text" {
			h.handleMessage(&msg)
		}
	}
}


// pushServerInfo 推送本机服务信息到前端
func (h *Hub) pushServerInfo(conn *websocket.Conn) {
	localIP := getLocalIP()
	msg := WSMessage{
		ID:        newUUID(),
		Type:      "server_info",
		Content:   localIP,
		Timestamp: time.Now().UnixMilli(),
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	wsjson.Write(ctx, conn, &msg)
}
// pushHistory 向前端推送历史消息（MVP 简化：按 conversation_id 查询并推送）
func (h *Hub) pushHistory(conn *websocket.Conn, myDeviceID string) {
	log.Printf("历史消息推送待实现（按 conversation_id 查询）")
}

// ─── 设备事件推送 ──────────────────────────────────────────

// pushDeviceOnline 推送设备上线事件到前端
func (h *Hub) pushDeviceOnline(deviceID, name, ip string, port int) {
	h.localConnMu.Lock()
	conn := h.localConn
	h.localConnMu.Unlock()
	if conn == nil {
		return
	}

	devInfo := map[string]interface{}{
		"name": name,
		"ip":   ip,
		"port": port,
	}
	devJSON, _ := json.Marshal(devInfo)

	msg := WSMessage{
		ID:        newUUID(),
		Type:      "device_online",
		From:      deviceID,
		Content:   string(devJSON),
		Timestamp: time.Now().UnixMilli(),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	wsjson.Write(ctx, conn, &msg)
}

// pushDeviceOffline 推送设备下线事件到前端
func (h *Hub) pushDeviceOffline(deviceID string) {
	h.localConnMu.Lock()
	conn := h.localConn
	h.localConnMu.Unlock()
	if conn == nil {
		return
	}

	msg := WSMessage{
		ID:        newUUID(),
		Type:      "device_offline",
		Content:   deviceID,
		Timestamp: time.Now().UnixMilli(),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	wsjson.Write(ctx, conn, &msg)
}

// ─── 消息处理 ──────────────────────────────────────────────

// handleMessage 处理收到的消息并转发
func (h *Hub) handleMessage(msg *WSMessage) {
	if msg.ID == "" {
		msg.ID = newUUID()
	}
	if msg.From == "" {
		msg.From = h.deviceID
	}
	if msg.Timestamp == 0 {
		msg.Timestamp = time.Now().UnixMilli()
	}

	cid := conversationID(msg.From, msg.To)

	// 存入 SQLite
	dbMsg := &Message{
		ID:             msg.ID,
		Type:           msg.Type,
		FromDevice:     msg.From,
		ToDevice:       msg.To,
		ConversationID: cid,
		Content:        msg.Content,
		CreatedAt:      msg.Timestamp,
	}
	if err := saveMessage(h.db, dbMsg); err != nil {
		log.Printf("保存消息失败: %v", err)
	}

	// 回显到本机前端
	h.pushToLocal(msg)

	// 如果不是发给自己的，转发给目标设备
	if msg.To != "" && msg.To != h.deviceID {
		h.forwardToPeer(msg)
	}
}

// pushToLocal 推送消息到本机前端
func (h *Hub) pushToLocal(msg *WSMessage) {
	h.localConnMu.Lock()
	conn := h.localConn
	h.localConnMu.Unlock()
	if conn == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := wsjson.Write(ctx, conn, msg); err != nil {
		log.Printf("推送到前端失败: %v", err)
	}
}

// forwardToPeer 转发消息给目标设备
func (h *Hub) forwardToPeer(msg *WSMessage) {
	if msg.To == "" {
		return
	}

	h.peersMu.RLock()
	pc, ok := h.peers[msg.To]
	h.peersMu.RUnlock()

	if !ok || pc == nil {
		log.Printf("目标设备 %s 不在线，消息 %s 无法转发", msg.To, msg.ID)
		return
	}

	pc.mu.Lock()
	ready := pc.state == stateReady
	conn := pc.conn
	pc.mu.Unlock()

	if !ready || conn == nil {
		log.Printf("目标设备 %s 连接未就绪", msg.To)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := wsjson.Write(ctx, conn, msg); err != nil {
		log.Printf("转发消息到 %s 失败: %v", msg.To, err)
	}
}

// ─── 远端连接管理 ──────────────────────────────────────────

// ConnectToPeer 发起对远端设备的 WebSocket 连接
func (h *Hub) ConnectToPeer(deviceID, ip string, port int) {
	h.peersMu.RLock()
	existing, ok := h.peers[deviceID]
	h.peersMu.RUnlock()
	if ok && existing != nil {
		existing.mu.Lock()
		state := existing.state
		existing.mu.Unlock()
		if state == stateReady || state == stateHandshaking {
			return
		}
	}

	go h.connectWithRetry(deviceID, ip, port)
}

func (h *Hub) connectWithRetry(deviceID, ip string, port int) {
	delay := initialRetry

	for {
		conn, err := h.tryConnect(deviceID, ip, port)
		if err == nil {
			h.startHandshake(deviceID, conn, true)
			return
		}

		log.Printf("连接 %s@%s:%d 失败: %v，%v 后重试", deviceID, ip, port, err, delay)
		time.Sleep(delay)

		delay = delay * 2
		if delay > maxRetryDelay {
			delay = maxRetryDelay
		}
	}
}

func (h *Hub) tryConnect(deviceID, ip string, port int) (*websocket.Conn, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	url := fmt.Sprintf("ws://%s:%d/ws", ip, port)
	conn, _, err := websocket.Dial(ctx, url, nil)
	if err != nil {
		return nil, err
	}

	// 存储连接，state = connecting
	pc := &peerConn{
		deviceID: deviceID,
		conn:     conn,
		state:    stateConnecting,
	}

	h.peersMu.Lock()
	existing, exists := h.peers[deviceID]
	if exists && existing.state >= stateHandshaking {
		h.peersMu.Unlock()
		conn.Close(websocket.StatusNormalClosure, "重复连接")
		return nil, fmt.Errorf("已有更高状态的连接")
	}
	h.peers[deviceID] = pc
	h.peersMu.Unlock()

	// 启动读取 goroutine
	go h.readPeer(deviceID, conn)

	return conn, nil
}

// HandleIncomingConnection 处理远端设备的主动连入
func (h *Hub) HandleIncomingConnection(deviceID string, conn *websocket.Conn, handshakeMsg *WSMessage) {
	h.peersMu.Lock()
	existing, exists := h.peers[deviceID]
	if exists && existing.state >= stateHandshaking {
		h.peersMu.Unlock()
		conn.Close(websocket.StatusNormalClosure, "已有连接")
		return
	}

	pc := &peerConn{
		deviceID: deviceID,
		conn:     conn,
		state:    stateConnecting,
	}
	h.peers[deviceID] = pc
	h.peersMu.Unlock()

	// 处理收到的 handshake 消息
	h.handleHandshake(deviceID, handshakeMsg)

	// 启动读取 goroutine
	go h.readPeer(deviceID, conn)
}

// ─── Handshake 去重逻辑 ────────────────────────────────────

// startHandshake 发送 handshake 消息
func (h *Hub) startHandshake(deviceID string, conn *websocket.Conn, initiator bool) {
	pc := h.getPeerConn(deviceID)
	if pc == nil {
		return
	}

	pc.mu.Lock()
	pc.state = stateHandshaking
	pc.mu.Unlock()

	hs := WSMessage{
		Type:      "handshake",
		From:      h.deviceID,
		To:        deviceID,
		Content:   h.deviceName,
		Timestamp: time.Now().UnixMilli(),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := wsjson.Write(ctx, conn, &hs); err != nil {
		log.Printf("发送 handshake 到 %s 失败: %v", deviceID, err)
		h.removePeer(deviceID)
		return
	}

	// 主动发起方：等对端响应后比较 device_id
	if initiator {
		time.Sleep(500 * time.Millisecond)

		pc.mu.Lock()
		if pc.state == stateHandshaking {
			if h.deviceID > deviceID {
				log.Printf("device_id %s > %s，关闭主动连接", h.deviceID, deviceID)
				pc.state = stateConnecting
				pc.mu.Unlock()
				conn.Close(websocket.StatusNormalClosure, "device_id 更大，让连接留给对端")
				h.removePeer(deviceID)
				return
			}
			pc.state = stateReady
			pc.mu.Unlock()
			log.Printf("与 %s 的 WebSocket 连接已就绪 (主动发起)", deviceID)
		} else {
			pc.mu.Unlock()
		}
	}
}

// handleHandshake 处理收到的 handshake
func (h *Hub) handleHandshake(fromDevice string, msg *WSMessage) {
	pc := h.getPeerConn(fromDevice)
	if pc == nil {
		return
	}

	pc.mu.Lock()
	defer pc.mu.Unlock()

	if pc.state == stateReady {
		return
	}

	if h.deviceID < msg.From {
		pc.state = stateReady
		log.Printf("与 %s (%s) 的 WebSocket 连接已就绪 (handshake 确认)", fromDevice, msg.Content)
	} else {
		log.Printf("device_id %s > %s，由对端保留连接", h.deviceID, msg.From)
		pc.conn.Close(websocket.StatusNormalClosure, "对端保留连接")
		pc.state = stateConnecting
	}
}

// ─── 读取循环 ──────────────────────────────────────────────

// readPeer 持续读取远端设备的消息
func (h *Hub) readPeer(deviceID string, conn *websocket.Conn) {
	defer func() {
		h.removePeer(deviceID)
		// 触发重连
		h.reconnectPeer(deviceID)
	}()

	for {
		var msg WSMessage
		if err := wsjson.Read(context.Background(), conn, &msg); err != nil {
			if websocket.CloseStatus(err) == websocket.StatusNormalClosure {
				return
			}
			log.Printf("读取 %s 消息失败: %v", deviceID, err)
			return
		}

		switch msg.Type {
		case "handshake":
			h.handleHandshake(deviceID, &msg)
		case "text":
			h.handleMessage(&msg)
		}
	}
}

// ─── 辅助方法 ──────────────────────────────────────────────

func (h *Hub) removePeer(deviceID string) {
	h.peersMu.Lock()
	delete(h.peers, deviceID)
	h.peersMu.Unlock()
}

func (h *Hub) reconnectPeer(deviceID string) {
	// 重连由外部通过 mDNS 重新发现触发
	log.Printf("设备 %s 连接断开，等待 mDNS 重新发现", deviceID)
}

func (h *Hub) getPeerConn(deviceID string) *peerConn {
	h.peersMu.RLock()
	defer h.peersMu.RUnlock()
	return h.peers[deviceID]
}

// ─── 心跳 ──────────────────────────────────────────────────

// startPingLoop 定时发送心跳
func (h *Hub) startPingLoop() {
	ticker := time.NewTicker(pingInterval)
	defer ticker.Stop()

	for range ticker.C {
		h.peersMu.RLock()
		for deviceID, pc := range h.peers {
			if pc.state != stateReady {
				continue
			}
			go func(id string, conn *websocket.Conn) {
				ctx, cancel := context.WithTimeout(context.Background(), pongTimeout)
				defer cancel()
				if err := conn.Ping(ctx); err != nil {
					log.Printf("%s ping 失败: %v", id, err)
					h.removePeer(id)
				}
			}(deviceID, pc.conn)
		}
		h.peersMu.RUnlock()
	}
}
