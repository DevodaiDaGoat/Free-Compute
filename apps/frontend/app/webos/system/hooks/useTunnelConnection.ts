'use client';

import { useCallback, useEffect, useRef, useState } from 'react';

export type TunnelStatus = 'idle' | 'connecting' | 'connected' | 'reconnecting' | 'closed' | 'error';

export type ChannelType =
  | 0x00  // control / signaling JSON
  | 0x01  // video  RTP  (binary)
  | 0x02  // audio  RTP  (binary)
  | 0x03  // input  events (binary)
  | 0x04  // clipboard (utf-8)
  | 0x05  // file-transfer
  | 0x06  // ssh-data

const CH_CONTROL:   ChannelType = 0x00;
const CH_VIDEO:     ChannelType = 0x01;
const CH_AUDIO:     ChannelType = 0x02;
const CH_INPUT:     ChannelType = 0x03;
const CH_CLIPBOARD: ChannelType = 0x04;
const CH_SSH:       ChannelType = 0x06;

export { CH_CONTROL, CH_VIDEO, CH_AUDIO, CH_INPUT, CH_CLIPBOARD, CH_SSH };

type FrameHandler = (payload: ArrayBuffer) => void;

interface TunnelOptions {
  url: string;
  sessionId: string;
  token?: string;
  maxReconnectMs?: number;
  onStatusChange?: (s: TunnelStatus) => void;
}

const FRAME_HEADER_SIZE = 5;  // 1 byte channel + 4 bytes length (big-endian uint32)

function encodeFrame(ch: number, payload: ArrayBuffer | string): ArrayBuffer {
  const data = typeof payload === 'string'
    ? new TextEncoder().encode(payload).buffer
    : payload;
  const buf = new ArrayBuffer(FRAME_HEADER_SIZE + data.byteLength);
  const view = new DataView(buf);
  view.setUint8(0, ch);
  view.setUint32(1, data.byteLength, false);
  new Uint8Array(buf, FRAME_HEADER_SIZE).set(new Uint8Array(data));
  return buf;
}

function encodeInputEvent(ev: object): ArrayBuffer {
  return encodeFrame(CH_INPUT, JSON.stringify(ev));
}

function encodeClipboard(text: string): ArrayBuffer {
  return encodeFrame(CH_CLIPBOARD, text);
}

export interface TunnelConnection {
  status: TunnelStatus;
  rttMs: number;
  send: (ch: ChannelType, payload: ArrayBuffer | string) => void;
  sendInput: (ev: object) => void;
  sendClipboard: (text: string) => void;
  sendSSH: (data: ArrayBuffer) => void;
  onFrame: (ch: ChannelType, handler: FrameHandler) => () => void;
  close: () => void;
}

export function useTunnelConnection(opts: TunnelOptions | null): TunnelConnection {
  const [status, setStatus] = useState<TunnelStatus>('idle');
  const [rttMs, setRttMs] = useState(0);

  const wsRef = useRef<WebSocket | null>(null);
  const handlersRef = useRef<Map<number, Set<FrameHandler>>>(new Map());
  const reconnectTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const reconnectDelayRef = useRef(500);
  const closedRef = useRef(false);
  const pingTimerRef = useRef<ReturnType<typeof setInterval> | null>(null);
  const lastPingRef = useRef(0);
  const inputBatchRef = useRef<object[]>([]);
  const inputFlushRef = useRef<ReturnType<typeof requestAnimationFrame> | null>(null);
  const optsRef = useRef<TunnelOptions | null>(opts);

  useEffect(() => {
    optsRef.current = opts;
  }, [opts]);

  const setStatusBoth = useCallback((s: TunnelStatus) => {
    setStatus(s);
    optsRef.current?.onStatusChange?.(s);
  }, []);

  const dispatchFrame = useCallback((ch: number, payload: ArrayBuffer) => {
    const handlers = handlersRef.current.get(ch);
    if (handlers) {
      for (const h of handlers) {
        try { h(payload); } catch { /* ignore handler errors */ }
      }
    }
  }, []);

  const connect = useCallback(() => {
    const current = optsRef.current;
    if (!current || closedRef.current) return;
    setStatusBoth('connecting');

    const ws = new WebSocket(current.url);
    ws.binaryType = 'arraybuffer';
    wsRef.current = ws;

    ws.onopen = () => {
      if (closedRef.current) { ws.close(); return; }
      reconnectDelayRef.current = 500;
      setStatusBoth('connected');

      const now = optsRef.current;
      if (now?.token) {
        ws.send(encodeFrame(CH_CONTROL, JSON.stringify({
          type: 'auth',
          token: now.token,
          sessionId: now.sessionId,
        })));
      }

      pingTimerRef.current = setInterval(() => {
        if (ws.readyState !== WebSocket.OPEN) return;
        lastPingRef.current = performance.now();
        ws.send(encodeFrame(CH_CONTROL, JSON.stringify({ type: 'ping', t: lastPingRef.current })));
      }, 5000);
    };

    ws.onmessage = (ev: MessageEvent) => {
      if (!(ev.data instanceof ArrayBuffer)) return;
      const data = ev.data;
      if (data.byteLength < FRAME_HEADER_SIZE) return;
      const view = new DataView(data);
      const ch = view.getUint8(0);
      const len = view.getUint32(1, false);
      const payload = data.slice(FRAME_HEADER_SIZE, FRAME_HEADER_SIZE + len);

      if (ch === CH_CONTROL) {
        try {
          const msg = JSON.parse(new TextDecoder().decode(payload));
          if (msg.type === 'pong' && lastPingRef.current) {
            setRttMs(Math.round(performance.now() - lastPingRef.current));
          }
        } catch { /* ignore malformed control */ }
      } else {
        dispatchFrame(ch, payload);
      }
    };

    ws.onerror = () => {
      if (!closedRef.current) setStatusBoth('error');
    };

    ws.onclose = () => {
      if (pingTimerRef.current) { clearInterval(pingTimerRef.current); pingTimerRef.current = null; }
      if (closedRef.current) return;
      setStatusBoth('reconnecting');
      const delay = reconnectDelayRef.current;
      const maxDelay = optsRef.current?.maxReconnectMs ?? 16000;
      reconnectDelayRef.current = Math.min(delay * 2, maxDelay);
      reconnectTimerRef.current = setTimeout(connect, delay);
    };
  }, [setStatusBoth, dispatchFrame]);

  useEffect(() => {
    if (!opts) return;
    closedRef.current = false;
    reconnectDelayRef.current = 500;
    connect();
    return () => {
      closedRef.current = true;
      if (reconnectTimerRef.current) { clearTimeout(reconnectTimerRef.current); reconnectTimerRef.current = null; }
      if (pingTimerRef.current) { clearInterval(pingTimerRef.current); pingTimerRef.current = null; }
      if (inputFlushRef.current) { cancelAnimationFrame(inputFlushRef.current); inputFlushRef.current = null; }
      try { wsRef.current?.close(); } catch { /* ignore */ }
      wsRef.current = null;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [opts?.url]);

  const send = useCallback((ch: ChannelType, payload: ArrayBuffer | string) => {
    const ws = wsRef.current;
    if (!ws || ws.readyState !== WebSocket.OPEN) return;
    ws.send(encodeFrame(ch, payload));
  }, []);

  const sendInput = useCallback((ev: object) => {
    inputBatchRef.current.push(ev);
    if (!inputFlushRef.current) {
      inputFlushRef.current = requestAnimationFrame(() => {
        inputFlushRef.current = null;
        const batch = inputBatchRef.current.splice(0);
        if (!batch.length) return;
        const ws = wsRef.current;
        if (!ws || ws.readyState !== WebSocket.OPEN) return;
        const payload = batch.length === 1 ? batch[0] : { type: 'batch', events: batch };
        ws.send(encodeInputEvent(payload));
      });
    }
  }, []);

  const sendClipboard = useCallback((text: string) => {
    const ws = wsRef.current;
    if (!ws || ws.readyState !== WebSocket.OPEN) return;
    ws.send(encodeClipboard(text));
  }, []);

  const sendSSH = useCallback((data: ArrayBuffer) => {
    const ws = wsRef.current;
    if (!ws || ws.readyState !== WebSocket.OPEN) return;
    ws.send(encodeFrame(CH_SSH, data));
  }, []);

  const onFrame = useCallback((ch: ChannelType, handler: FrameHandler): (() => void) => {
    if (!handlersRef.current.has(ch)) {
      handlersRef.current.set(ch, new Set());
    }
    handlersRef.current.get(ch)!.add(handler);
    return () => {
      handlersRef.current.get(ch)?.delete(handler);
    };
  }, []);

  const close = useCallback(() => {
    closedRef.current = true;
    if (reconnectTimerRef.current) clearTimeout(reconnectTimerRef.current);
    if (pingTimerRef.current) clearInterval(pingTimerRef.current);
    wsRef.current?.close();
    setStatusBoth('closed');
  }, [setStatusBoth]);

  return { status, rttMs, send, sendInput, sendClipboard, sendSSH, onFrame, close };
}
