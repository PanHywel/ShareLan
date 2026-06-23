<script lang="ts">
  import type { Device } from '../lib/types';

  let { device, isActive, onclick }: {
    device: Device;
    isActive: boolean;
    onclick: () => void;
  } = $props();
</script>

<button
  onclick={onclick}
  class="w-full flex items-center gap-2.5 px-3 py-2.5 text-left text-sm transition-colors
    {isActive
      ? 'bg-blue-50 border-l-[3px] border-blue-500 text-gray-900'
      : 'border-l-[3px] border-transparent text-gray-700 hover:bg-gray-100'}"
>
  <span class="w-2.5 h-2.5 rounded-full flex-shrink-0 {device.online ? 'bg-green-500' : 'bg-gray-300'}"></span>
  <span class="truncate font-medium">{device.name}</span>
</button>
