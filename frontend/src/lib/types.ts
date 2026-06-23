/** 设备信息 */
export interface Device {
  id: string;
  name: string;
  ip: string;
  port: number;
  online: boolean;
}

/** 消息结构（与 Go 后端 WSMessage 一致） */
export interface Message {
  id: string;
  type: 'text' | 'hello' | 'handshake' | 'device_online' | 'device_offline';
  from: string;
  to: string;
  content: string;
  timestamp: number;
}

/** 在线设备事件 payload */
export interface DeviceOnlinePayload {
  name: string;
  ip: string;
  port: number;
}

/** 生成本机持久化设备 ID */
export function getDeviceId(): string {
  let id = localStorage.getItem('sharelan_device_id');
  if (!id) {
    id = crypto.randomUUID();
    localStorage.setItem('sharelan_device_id', id);
  }
  return id;
}

/** 生成本机设备名 */
export function getDeviceName(): string {
  return '本机';
}
