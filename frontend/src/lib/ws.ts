import type { Message } from './types';

type MessageHandler = (msg: Message) => void;
type StatusHandler = (connected: boolean) => void;

export class WSClient {
  private ws: WebSocket | null = null;
  private url: string;
  private deviceId: string;
  private deviceName: string;
  private onMessage: MessageHandler;
  private onStatus: StatusHandler;
  private reconnectDelay = 1000;
  private readonly maxReconnectDelay = 30000;
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null;
  private destroyed = false;
  private debugTimer: ReturnType<typeof setInterval> | null = null;

  constructor(
    url: string,
    deviceId: string,
    deviceName: string,
    onMessage: MessageHandler,
    onStatus: StatusHandler
  ) {
    this.url = url;
    this.deviceId = deviceId;
    this.deviceName = deviceName;
    this.onMessage = onMessage;
    this.onStatus = onStatus;
    this.connect();
  }

  private connect() {
    if (this.destroyed) return;

    this.ws = new WebSocket(this.url);

    this.ws.onopen = () => {
      this.onStatus(true);
      this.reconnectDelay = 1000;
      // 连接后发送 hello 标识自己
      this.send({
        id: crypto.randomUUID(),
        type: 'hello',
        from: this.deviceId,
        to: '',
        content: this.deviceName,
        timestamp: Date.now(),
      });
      // 启动控制台日志上传（每 2s）
      this.startDebugUpload();
    };

    this.ws.onclose = () => {
      this.onStatus(false);
      this.scheduleReconnect();
    };

    this.ws.onerror = () => {
      this.ws?.close();
    };

    this.ws.onmessage = (event) => {
      try {
        const msg: Message = JSON.parse(event.data);
        this.onMessage(msg);
      } catch (e) {
        console.error('消息解析失败:', e);
      }
    };
  }

  private scheduleReconnect() {
    if (this.destroyed) return;
    if (this.reconnectTimer) return;

    this.reconnectTimer = setTimeout(() => {
      this.reconnectTimer = null;
      this.connect();
    }, this.reconnectDelay);

    this.reconnectDelay = Math.min(
      this.reconnectDelay * 2,
      this.maxReconnectDelay
    );
  }

  send(msg: Message) {
    if (this.ws?.readyState === WebSocket.OPEN) {
      const data = JSON.stringify(msg);
      console.log('[WS.send] 发送 type=' + msg.type + ' to=' + (msg.to||'').slice(0,8) + ' readyState=' + this.ws.readyState);
      this.ws.send(data);
    } else {
      console.warn('[WS.send] WebSocket 未就绪: readyState=' + (this.ws ? this.ws.readyState : 'null'));
    }
  }

  private startDebugUpload() {
    if (this.debugTimer) clearInterval(this.debugTimer);
    this.debugTimer = setInterval(() => {
      const getLogs = (window as any).__getConsoleLogs;
      if (typeof getLogs === 'function') {
        const lines = getLogs();
        if (lines && lines.length > 0) {
          this.send({
            id: crypto.randomUUID(),
            type: 'debug_log' as any,
            from: this.deviceId,
            to: '',
            content: lines.join('\n'),
            timestamp: Date.now(),
          });
        }
      }
    }, 2000);
  }

  private stopDebugUpload() {
    if (this.debugTimer) {
      clearInterval(this.debugTimer);
      this.debugTimer = null;
    }
  }

  destroy() {
    this.destroyed = true;
    this.stopDebugUpload();
    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer);
      this.reconnectTimer = null;
    }
    this.ws?.close();
    this.ws = null;
  }
}
