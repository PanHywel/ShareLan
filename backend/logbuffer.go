package main

import (
	"bytes"
	"fmt"
	"io"
	"sync"
	"time"
)

// logBuffer 全局日志缓冲区（在 main 或 test_main 中初始化）
var logBuffer *LogBuffer

// LogBuffer 环形日志缓冲区，同时写入 stderr 和内存
type LogBuffer struct {
	buf    []byte
	max    int
	mu     sync.RWMutex
	writer io.Writer
}

// NewLogBuffer 创建日志缓冲区，maxBytes 达到后循环覆盖
func NewLogBuffer(maxBytes int, writer io.Writer) *LogBuffer {
	return &LogBuffer{
		buf:    make([]byte, 0, maxBytes),
		max:    maxBytes,
		writer: writer,
	}
}

func (lb *LogBuffer) Write(p []byte) (n int, err error) {
	// 同时写入原始目标（stderr）
	lb.writer.Write(p)

	// 写入环形缓冲
	line := []byte(fmt.Sprintf("[%s] %s", time.Now().Format("15:04:05.000"), string(p)))

	lb.mu.Lock()
	defer lb.mu.Unlock()

	lb.buf = append(lb.buf, line...)
	if len(lb.buf) > lb.max {
		// 丢掉最早的一半
		half := len(lb.buf) / 2
		copy(lb.buf, lb.buf[half:])
		lb.buf = lb.buf[:len(lb.buf)-half]
	}
	return len(p), nil
}

// Recent 返回最近的日志文本
func (lb *LogBuffer) Recent() []byte {
	lb.mu.RLock()
	defer lb.mu.RUnlock()
	return bytes.Clone(lb.buf)
}

// Clear 清空日志
func (lb *LogBuffer) Clear() {
	lb.mu.Lock()
	defer lb.mu.Unlock()
	lb.buf = lb.buf[:0]
}
