<script lang="ts">
  import { devices } from '../stores/devices';
  import { activeDeviceId } from '../stores/activeChat';
  import DeviceItem from './DeviceItem.svelte';

  let { onSelect, localIP = '' }: { onSelect: (id: string) => void; localIP?: string } = $props();

  let debugMode = $state(true);
  let logText = $state('');

  async function fetchLogs() {
    try {
      const r = await fetch('/debug/logs');
      logText = await r.text();
    } catch (e) {
      logText = '获取日志失败: ' + e;
    }
  }

  async function copyLogs() {
    await fetchLogs();
    await navigator.clipboard.writeText(logText);
    alert('日志已复制到剪贴板，粘贴到对话中即可');
  }
</script>

<div class="h-full flex flex-col bg-gray-50">
  <!-- 标题 -->
  <div class="p-3 border-b border-gray-200">
    <h1 class="text-base font-semibold text-gray-800">ShareLan</h1>
    <p class="text-xs text-gray-400 mt-0.5">
      {#if localIP}
        {localIP}
      {:else}
        局域网通讯工具
      {/if}
    </p>
    <p class="text-[10px] text-gray-300 mt-0.5">调试模式</p>
  </div>

  <!-- 设备列表 -->
  <div class="flex-1 overflow-y-auto">
    <div class="px-3 py-2 text-xs text-gray-400 uppercase tracking-wide font-medium">
      在线设备
    </div>
    {#each $devices as device (device.id)}
      <DeviceItem
        {device}
        isActive={$activeDeviceId === device.id}
        onclick={() => onSelect(device.id)}
      />
    {:else}
      <div class="px-4 py-8 text-sm text-gray-400 text-center">
        <p>暂无在线设备</p>
        <p class="text-xs mt-1">等待其他设备加入局域网...</p>
      </div>
    {/each}
  </div>

  <!-- 调试工具 -->
  {#if debugMode}
    <div class="border-t border-gray-200 p-2 space-y-1">
      <button
        onclick={copyLogs}
        class="w-full text-xs px-2 py-1.5 rounded bg-gray-200 hover:bg-gray-300 text-gray-700 transition-colors"
      >
        📋 复制日志
      </button>
    </div>
  {/if}
</div>
