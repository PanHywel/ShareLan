<script lang="ts">
  import { devices } from '../stores/devices';
  import { activeDeviceId } from '../stores/activeChat';
  import DeviceItem from './DeviceItem.svelte';

  let { onSelect, localIP = '' }: { onSelect: (id: string) => void; localIP?: string } = $props();
</script>

<div class="h-full flex flex-col bg-gray-50">
  <!-- 标题 -->
  <div class="p-3 border-b border-gray-200">
    <h1 class="text-base font-semibold text-gray-800">ShareLan</h1>
    {#if localIP}
      <p class="text-xs text-gray-400 mt-0.5">{localIP}</p>
    {/if}
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
</div>
