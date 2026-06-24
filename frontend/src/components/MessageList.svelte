<script lang="ts">
  import { allMessages } from '../stores/messages';
  import { getDeviceId } from '../lib/types';
  import MessageItem from './MessageItem.svelte';
  import type { Device, Message } from '../lib/types';
  import { onMount, onDestroy } from 'svelte';

  let { currentDevice }: { currentDevice: Device } = $props();

  let myDeviceId = getDeviceId();
  let messages: Message[] = $state([]);

  function conversationId(a: string, b: string): string {
    return a < b ? `${a}:${b}` : `${b}:${a}`;
  }

  const unsub = allMessages.subscribe(all => {
    const convId = conversationId(myDeviceId, currentDevice.id);
    messages = all.filter((m: Message) => {
      if (m.type !== 'text') return false;
      return conversationId(m.from, m.to) === convId;
    });
  });

  onDestroy(unsub);

  let container: HTMLDivElement;
  $effect(() => {
    if (container && messages.length > 0) {
      requestAnimationFrame(() => {
        container.scrollTop = container.scrollHeight;
      });
    }
  });
</script>

<div bind:this={container} class="h-full overflow-y-auto select-text">
  {#each messages as msg (msg.id)}
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
