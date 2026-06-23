import { writable, derived } from 'svelte/store';
import type { Message } from '../lib/types';

export const allMessages = writable<Message[]>([]);

/** 按 conversation_id 分组的消息 */
export const messagesByConversation = derived(allMessages, ($msgs) => {
  const map = new Map<string, Message[]>();
  for (const msg of $msgs) {
    if (msg.type !== 'text') continue;
    const convId = conversationId(msg.from, msg.to);
    const list = map.get(convId) || [];
    list.push(msg);
    map.set(convId, list);
  }
  return map;
});

export function addMessage(msg: Message) {
  allMessages.update(list => [...list, msg]);
}

export function loadMessages(msgs: Message[]) {
  allMessages.set(msgs);
}

function conversationId(a: string, b: string): string {
  return a < b ? `${a}:${b}` : `${b}:${a}`;
}
