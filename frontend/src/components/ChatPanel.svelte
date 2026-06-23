<script lang="ts">
  import { activeDeviceId } from '../stores/activeChat';
  import { devices } from '../stores/devices';
  import MessageList from './MessageList.svelte';
  import MessageInput from './MessageInput.svelte';

  let { sendMessage }: { sendMessage: (content: string) => void } = $props();

  let currentDevice = $derived.by(() => {
    return $devices.find(d => d.id === $activeDeviceId) ?? null;
  });
</script>

{#if currentDevice}
  <div class="flex flex-col h-full">
    <!-- 聊天头部 -->
    <div class="px-4 py-3 border-b border-gray-200 bg-gray-50 flex items-center gap-2">
      <span class="w-2 h-2 rounded-full bg-green-500"></span>
      <h2 class="text-sm font-medium text-gray-800">{currentDevice.name}</h2>
    </div>
    <!-- 消息列表 -->
    <div class="flex-1 overflow-y-auto px-4 py-3">
      <MessageList {currentDevice} />
    </div>
    <!-- 输入区域 -->
    <div class="border-t border-gray-200">
      <MessageInput {sendMessage} />
    </div>
  </div>
{:else}
  <div class="flex-1 flex items-center justify-center text-gray-400">
    <div class="text-center">
      <p class="text-base">选择一个设备开始聊天</p>
      <p class="text-xs mt-1">点击左侧在线设备列表中的设备</p>
    </div>
  </div>
{/if}
