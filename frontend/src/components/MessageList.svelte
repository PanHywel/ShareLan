<script lang="ts">
  import { allMessages } from '../stores/messages';
  import { getDeviceId } from '../lib/types';
  import MessageItem from './MessageItem.svelte';
  import type { Device, Message } from '../lib/types';

  let { currentDevice }: { currentDevice: Device } = $props();

  let myDeviceId = getDeviceId();

  let filteredMessages = $derived.by(() => {
    const convId = conversationId(myDeviceId, currentDevice.id);
    return $allMessages.filter((m: Message) => {
      if (m.type !== 'text') return false;
      return conversationId(m.from, m.to) === convId;
    });
  });

  function conversationId(a: string, b: string): string {
    return a < b ? `${a}:${b}` : `${b}:${a}`;
  }

  let container: HTMLDivElement;
  // 调试消息数量
  let totalMsgs = $derived($allMessages.length);
  let filteredCount = $derived(filteredMessages.length);
  console.log('[MessageList] total=' + totalMsgs + ' filtered=' + filteredCount + ' myId=' + myDeviceId.slice(0,8) + ' convId=' + conversationId(myDeviceId, currentDevice.id).slice(0,20));

  $effect(() => {
    // 新消息到达时滚动到底部
    if (container) {
      requestAnimationFrame(() => {
        container.scrollTop = container.scrollHeight;
      });
    }
  });
</script>

<div bind:this={container} class="h-full overflow-y-auto select-text">
  {#each filteredMessages as msg (msg.id)}
    <MessageItem message={msg} isMine={msg.from === myDeviceId} />
  {:else}
    <div class="flex items-center justify-center h-full text-gray-400 text-sm">
      <div class="text-center">
        <p>暂无消息</p>
        <p class="text-xs mt-1">发送第一条消息开始对话</p>
      </div>
    </div>
  {/each}
</div>
