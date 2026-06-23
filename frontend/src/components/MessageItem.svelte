<script lang="ts">
  import type { Message } from '../lib/types';

  let { message, isMine = false }: { message: Message; isMine?: boolean } = $props();

  let timeStr = $derived.by(() => {
    const d = new Date(message.timestamp);
    return d.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' });
  });
</script>

<div class="flex {isMine ? 'justify-end' : 'justify-start'} mb-3">
  <div
    class="max-w-[70%] min-w-[80px] px-3 py-2 rounded-lg {isMine
      ? 'bg-blue-500 text-white rounded-br-sm'
      : 'bg-gray-100 text-gray-800 rounded-bl-sm'}"
  >
    <p class="whitespace-pre-wrap break-words select-text text-sm leading-relaxed">{message.content}</p>
    <div class="flex items-center gap-1 mt-1 {isMine ? 'justify-end' : 'justify-start'}">
      <span class="text-[10px] opacity-60">{timeStr}</span>
    </div>
  </div>
</div>
