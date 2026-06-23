package main

import (
	"crypto/rand"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// Message 统一消息结构
type Message struct {
	ID             string `json:"id"`
	Type           string `json:"type"`
	FromDevice     string `json:"from"`
	ToDevice       string `json:"to"`
	ConversationID string `json:"conversation_id"`
	Content        string `json:"content"`
	CreatedAt      int64  `json:"timestamp"`
}

// newUUID 使用 crypto/rand 生成 UUID v4（不依赖外部包）
func newUUID() string {
	b := make([]byte, 16)
	rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// initDB 初始化 SQLite 数据库
func initDB() (*sql.DB, error) {
	dir := filepath.Join(os.Getenv("HOME"), ".sharelan")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("创建数据目录失败: %w", err)
	}

	dbPath := filepath.Join(dir, "messages.db")
	db, err := sql.Open("sqlite", dbPath+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("打开数据库失败: %w", err)
	}

	if err := createTables(db); err != nil {
		return nil, err
	}

	return db, nil
}

func createTables(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS messages (
			id              TEXT PRIMARY KEY,
			type            TEXT NOT NULL DEFAULT 'text',
			from_device     TEXT NOT NULL,
			to_device       TEXT NOT NULL,
			conversation_id TEXT NOT NULL,
			content         TEXT NOT NULL,
			created_at      INTEGER NOT NULL
		);
		CREATE INDEX IF NOT EXISTS idx_messages_conversation ON messages(conversation_id);
		CREATE INDEX IF NOT EXISTS idx_messages_created ON messages(created_at);
	`)
	return err
}

// saveMessage 保存消息到数据库
func saveMessage(db *sql.DB, msg *Message) error {
	_, err := db.Exec(
		`INSERT OR IGNORE INTO messages (id, type, from_device, to_device, conversation_id, content, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		msg.ID, msg.Type, msg.FromDevice, msg.ToDevice,
		msg.ConversationID, msg.Content, msg.CreatedAt,
	)
	return err
}

// getConversation 获取某个会话的历史消息
func getConversation(db *sql.DB, conversationID string, limit, offset int) ([]Message, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := db.Query(
		`SELECT id, type, from_device, to_device, conversation_id, content, created_at
		 FROM messages WHERE conversation_id = ?
		 ORDER BY created_at DESC LIMIT ? OFFSET ?`,
		conversationID, limit, offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []Message
	for rows.Next() {
		var m Message
		if err := rows.Scan(&m.ID, &m.Type, &m.FromDevice, &m.ToDevice,
			&m.ConversationID, &m.Content, &m.CreatedAt); err != nil {
			return nil, err
		}
		messages = append(messages, m)
	}
	// 反转为升序
	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}
	return messages, nil
}

// conversationID 生成统一的会话 ID
func conversationID(a, b string) string {
	if a < b {
		return a + ":" + b
	}
	return b + ":" + a
}
