import { writable } from 'svelte/store';

export const activeDeviceId = writable<string | null>(null);
