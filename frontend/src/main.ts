import App from './App.svelte';
import { mount } from 'svelte';
import './app.css';

// 劫持 console.log/warn/error 并缓存
const _log = console.log;
const _warn = console.warn;
const _error = console.error;
const logs: string[] = [];

function capture(level: string, args: any[]) {
  const line = `[${level}] ${args.map(a => typeof a === 'object' ? JSON.stringify(a) : String(a)).join(' ')}`;
  logs.push(line);
  if (logs.length > 500) logs.splice(0, 100);

  // 也输出到原始 console
  if (level === 'ERROR') _error(...args);
  else if (level === 'WARN') _warn(...args);
  else _log(...args);
}

console.log = (...args: any[]) => capture('LOG', args);
console.warn = (...args: any[]) => capture('WARN', args);
console.error = (...args: any[]) => capture('ERROR', args);

// 全局接口供 WSClient 定期上传日志
(window as any).__getConsoleLogs = () => logs.splice(0); // 取出并清空
(window as any).__consoleLogs = logs;

const app = mount(App, {
  target: document.getElementById('app')!,
});

export default app;
