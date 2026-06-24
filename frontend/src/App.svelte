<script lang="ts">
  import { onMount } from 'svelte';
  import Sidebar from './components/Sidebar.svelte';
  import ChatPanel from './components/ChatPanel.svelte';
  import { WSClient } from './lib/ws';
  import { addMessage } from './stores/messages';
  import { upsertDevice, setDeviceOnline } from './stores/devices';
  import { activeDeviceId } from './stores/activeChat';
  import { getDeviceId, getDeviceName } from './lib/types';
  import type { Message, DeviceOnlinePayload } from './lib/types';

  let wsClient: WSClient | null = $state(null);
  let connected = $state(false);
  let myDeviceId = $state(getDeviceId());
  let myDeviceName = $state(getDeviceName());
  let localIP = $state('');

  onMount(() => {
    const port = window.location.port || '17888';
    const url = `ws://127.0.0.1:${port}/ws`;

    wsClient = new WSClient(
      url,
      myDeviceId,
      myDeviceName,
      handleMessage,
      (status) => { connected = status; }
    );

    return () => {
      wsClient?.destroy();
    };
  });

  function handleMessage(msg: Message) {
    switch (msg.type) {
      case 'text':
        addMessage(msg);
        break;
      case 'server_info':
        try {
          const info = JSON.parse(msg.content);
          localIP = info.ip || msg.content;
          if (info.id) {
            myDeviceId = info.id;
            localStorage.setItem('sharelan_device_id', info.id);
          }
          if (info.name) myDeviceName = info.name;
        } catch {
          localIP = msg.content;
        }
        break;
      case 'device_online':
        handleDeviceOnline(msg);
        break;
      case 'device_offline':
        handleDeviceOffline(msg);
        break;
    }
  }

  function handleDeviceOnline(msg: Message) {
    try {
      const data: DeviceOnlinePayload = JSON.parse(msg.content);
      upsertDevice({
        id: msg.from,
        name: data.name,
        ip: data.ip,
        port: data.port,
        online: true,
      });
    } catch (e) {
      console.error('设备上线消息解析失败:', e);
    }
  }

  function handleDeviceOffline(msg: Message) {
    setDeviceOnline(msg.content, false);
  }

  function sendMessage(content: string) {
    const targetId = $activeDeviceId;
    console.log('[App.sendMessage] targetId=' + targetId + ' wsClient=' + (wsClient ? 'ok' : 'null') + ' myDeviceId=' + myDeviceId);
    if (!targetId || !wsClient) return;

    const msg: Message = {
      id: crypto.randomUUID(),
      type: 'text',
      from: myDeviceId,
      to: targetId,
      content,
      timestamp: Date.now(),
    };

    console.log('[App.sendMessage] 发送: to=' + msg.to.slice(0,8) + ' content=' + content);
    wsClient.send(msg);
    addMessage(msg);
    console.log('[App.sendMessage] 发送完成');
  }

  function handleDeviceSelect(id: string) {
    activeDeviceId.set(id);
  }
</script>

<div class="flex h-screen bg-white">
  <!-- 侧边栏 -->
  <div class="w-60 flex-shrink-0 border-r border-gray-200">
    <Sidebar onSelect={handleDeviceSelect} {localIP} />
  </div>
  <!-- 聊天区域 -->
  <div class="flex-1 flex flex-col">
    {#if connected}
      <ChatPanel {sendMessage} />
    {:else}
      <div class="flex-1 flex items-center justify-center text-gray-400">
        <div class="text-center">
          <p class="text-lg">正在连接后端服务...</p>
          <p class="text-sm mt-2">请确保 ShareLan 后端已启动</p>
        </div>
      </div>
    {/if}
  </div>
</div>
