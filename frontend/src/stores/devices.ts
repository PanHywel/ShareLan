import { writable } from 'svelte/store';
import type { Device } from '../lib/types';

export const devices = writable<Device[]>([]);

export function upsertDevice(device: Device) {
  devices.update(list => {
    const idx = list.findIndex(d => d.id === device.id);
    if (idx >= 0) {
      list[idx] = device;
      return [...list];
    }
    return [...list, device];
  });
}

export function removeDevice(id: string) {
  devices.update(list => list.filter(d => d.id !== id));
}

export function setDeviceOnline(id: string, online: boolean) {
  devices.update(list => {
    const device = list.find(d => d.id === id);
    if (device) {
      device.online = online;
      return [...list];
    }
    return list;
  });
}
