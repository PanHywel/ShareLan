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
      this.ws.send(JSON.stringify(msg));
    }
  }

  destroy() {
    this.destroyed = true;
    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer);
      this.reconnectTimer = null;
    }
    this.ws?.close();
    this.ws = null;
  }
}
