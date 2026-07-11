import type { ConnectionConfig } from '../types';

const GATEWAY_URL = process.env.NEXT_PUBLIC_GATEWAY_URL || 'http://localhost:8080';
const DEFAULT_TIMEOUT = 10_000;
const PREWARM_KEEPALIVE_MS = 120_000;

const warmupPool = new Map<string, WebSocket>();
let warmupTimer: ReturnType<typeof setInterval> | null = null;

class GatewayError extends Error {
  constructor(public status: number, message: string) {
    super(message);
    this.name = 'GatewayError';
  }
}

let activeController: AbortController | null = null;

function getSharedAbortSignal(): AbortSignal {
  if (!activeController || activeController.signal.aborted) {
    activeController = new AbortController();
  }
  return activeController.signal;
}

async function gatewayFetch<T>(url: string, opts: RequestInit = {}): Promise<T> {
  const controller = new AbortController();
  const timeout = setTimeout(() => controller.abort(), DEFAULT_TIMEOUT);

  const signal = opts.signal ? anySignal(opts.signal, controller.signal) : controller.signal;

  try {
    const resp = await fetch(url, {
      ...opts,
      signal,
      headers: {
        'Accept': 'application/json',
        ...(opts.headers as Record<string, string>),
      },
    });
    if (!resp.ok) throw new GatewayError(resp.status, `Gateway ${resp.status}: ${resp.statusText}`);
    return resp.json() as Promise<T>;
  } finally {
    clearTimeout(timeout);
  }
}

function anySignal(...signals: AbortSignal[]): AbortSignal {
  const controller = new AbortController();
  for (const signal of signals) {
    if (signal.aborted) {
      controller.abort(signal.reason);
      return controller.signal;
    }
    signal.addEventListener('abort', () => controller.abort(signal.reason), { once: true });
  }
  return controller.signal;
}

async function withRetry<T>(fn: () => Promise<T>, maxRetries = 2): Promise<T> {
  let lastErr: unknown;
  for (let attempt = 0; attempt <= maxRetries; attempt++) {
    try {
      return await fn();
    } catch (err) {
      lastErr = err;
      if (attempt < maxRetries) {
        await new Promise(r => setTimeout(r, Math.min(1000 * Math.pow(2, attempt), 4000)));
      }
    }
  }
  throw lastErr;
}

export function preconnectGateway(): void {
  const url = GATEWAY_URL.replace(/^http/, 'ws');
  if (warmupPool.has(url)) return;

  const ws = new WebSocket(url + '/prewarm');
  ws.onopen = () => {
    ws.send(new Uint8Array([0x01]));
  };
  ws.onclose = () => warmupPool.delete(url);
  ws.onerror = () => warmupPool.delete(url);

  warmupPool.set(url, ws);

  if (!warmupTimer) {
    warmupTimer = setInterval(() => {
      for (const [key, ws] of warmupPool) {
        if (ws.readyState === WebSocket.OPEN) {
          try { ws.send(new Uint8Array([0x01])); } catch { warmupPool.delete(key); }
        } else {
          warmupPool.delete(key);
        }
      }
    }, PREWARM_KEEPALIVE_MS);
  }
}

export function warmupConnection(): Promise<void> {
  return withRetry(async () => {
    const resp = await fetch(`${GATEWAY_URL}/healthz`, {
      method: 'HEAD',
      signal: AbortSignal.timeout(2000),
    });
    if (!resp.ok) throw new Error(`warmup failed: ${resp.status}`);
  });
}

export interface WebRTCSession {
  sessionId: string;
  clientId: string;
  videoCodec: string;
  audioCodec: string;
  signalingUrl: string;
  turnServers: string[];
  stunServers: string[];
  expiresAt: string;
}

export async function createWebRTCSession(config: ConnectionConfig): Promise<WebRTCSession> {
  const body = {
    videoCodecs: [config.videoCodec],
    audioCodecs: [config.audioCodec],
    preset: config.preset,
    encodingMode: config.encodingMode,
    resolution: { width: config.resolutionWidth, height: config.resolutionHeight, refreshRate: config.refreshRate },
    requestedFps: config.fps,
    latencyTarget: config.latencyTargetMs,
    gpuRequired: config.gpuRequired,
    gpuPreferred: config.gpuPreferred,
  };

  await warmupConnection();

  return withRetry(() =>
    gatewayFetch<WebRTCSession>(`${GATEWAY_URL}/webrtc/`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
      signal: getSharedAbortSignal(),
    })
  );
}

export async function createSession(config: ConnectionConfig) {
  const body = {
    type: config.sessionMode,
    mode: config.sessionMode,
    resourceClass: config.resourceClass,
    streamPreset: config.preset,
    gpuPreferred: config.gpuPreferred,
    gpuRequired: config.gpuRequired,
    requestedInputs: config.supportedInputs,
    requestedCapabilities: {
      clipboardSync: config.clipboardSync,
      fileTransfer: config.fileTransfer,
      multiMonitor: config.multiMonitor,
      audioForwarding: config.audioEnabled,
      highRefreshRate: config.highRefreshRate,
      fullscreen: config.fullscreen,
      controllerRumble: config.controllerRumble,
      sessionRecording: config.sessionRecording ? 'optional' : 'disabled',
      supportedInputs: config.supportedInputs,
    },
    permissions: {
      requireUserApproval: config.requireUserApproval,
      allowClipboardRead: config.clipboardRead,
      allowClipboardWrite: config.clipboardWrite,
      allowFileUpload: config.fileUpload,
      allowFileDownload: config.fileDownload,
      allowAudioForwarding: config.audioEnabled,
      allowSessionRecording: config.sessionRecording,
      maxDurationSeconds: config.maxDurationMinutes * 60,
      idleTimeoutSeconds: config.idleTimeoutMinutes * 60,
    },
  };

  return withRetry(() =>
    gatewayFetch(`${GATEWAY_URL}/sessions/`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    })
  );
}

export async function getCapabilities() {
  return withRetry(() => gatewayFetch(`${GATEWAY_URL}/capabilities`));
}

export async function getTailscaleStatus() {
  return gatewayFetch(`${GATEWAY_URL}/tailscale/hosts`);
}
