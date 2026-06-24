<script lang="ts">
  import { activeDeviceId } from '../stores/activeChat';

  let { sendMessage }: { sendMessage: (content: string) => void } = $props();

  let ta: HTMLTextAreaElement;

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      send();
    }
  }

  function send() {
    if (!ta || !$activeDeviceId) return;
    const content = ta.value.trim();
    if (!content) return;
    sendMessage(content);
    ta.value = '';
    // 手动触发 input 事件更新按钮状态
    ta.dispatchEvent(new Event('input'));
  }

  // 按钮状态通过 input 事件实时更新
  let canSend = $state(false);
  function onInput() {
    canSend = ta ? ta.value.trim().length > 0 && !!$activeDeviceId : false;
  }
</script>

<div class="flex items-end gap-2 p-3 bg-white">
  <textarea
    bind:this={ta}
    onkeydown={handleKeydown}
    oninput={onInput}
    placeholder="输入消息... (Enter 发送, Shift+Enter 换行)"
    rows="3"
    class="flex-1 resize-none rounded-lg border border-gray-300 px-3 py-2 text-sm
      focus:outline-none focus:border-blue-400 focus:ring-1 focus:ring-blue-400
      placeholder:text-gray-400"
  ></textarea>
  <button
    onclick={send}
    disabled={!canSend}
    class="px-4 py-2 rounded-lg bg-blue-500 text-white text-sm font-medium
      hover:bg-blue-600 active:bg-blue-700
      disabled:bg-gray-300 disabled:cursor-not-allowed
      transition-colors"
  >
    发送
  </button>
</div>
