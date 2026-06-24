<script lang="ts">
  import { activeDeviceId } from '../stores/activeChat';

  let { sendMessage }: { sendMessage: (content: string) => void } = $props();

  let text = $state('');
  let textareaEl: HTMLTextAreaElement | undefined = $state();

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      send();
    }
  }

  function handleInput(e: Event) {
    text = (e.target as HTMLTextAreaElement).value;
  }

  function send() {
    const content = text.trim();
    if (!content || !$activeDeviceId) return;
    sendMessage(content);
    text = '';
  }

  $effect(() => {
    // text 变化后同步到 textarea 的 value
    if (textareaEl) {
      textareaEl.value = text;
    }
  });
</script>

<div class="flex items-end gap-2 p-3 bg-white">
  <textarea
    bind:this={textareaEl}
    oninput={handleInput}
    onkeydown={handleKeydown}
    placeholder="输入消息... (Enter 发送, Shift+Enter 换行)"
    rows="3"
    class="flex-1 resize-none rounded-lg border border-gray-300 px-3 py-2 text-sm
      focus:outline-none focus:border-blue-400 focus:ring-1 focus:ring-blue-400
      placeholder:text-gray-400"
  ></textarea>
  <button
    onclick={send}
    disabled={!text.trim() || !$activeDeviceId}
    class="px-4 py-2 rounded-lg bg-blue-500 text-white text-sm font-medium
      hover:bg-blue-600 active:bg-blue-700
      disabled:bg-gray-300 disabled:cursor-not-allowed
      transition-colors"
  >
    发送
  </button>
</div>
