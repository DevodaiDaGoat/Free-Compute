import type { ProxyRoute, ProxyRouteRequest } from './proxy';

export type SessionMode = 'desktop' | 'development' | 'gaming' | 'remote-support';

export type SessionType = 'desktop' | 'gaming' | 'remote-support' | 'host';

export type ResourceClass = 'basic' | 'standard' | 'gaming' | 'workstation';

export type StreamPreset = 'safe' | 'fast';

export type StreamTransport = 'webrtc' | 'websocket-fallback';

export type VideoCodec = 'h264' | 'h265' | 'av1' | 'vp8' | 'vp9';

export type AudioCodec = 'opus' | 'aac';

export type RemoteSessionState =
  | 'requested'
  | 'queued'
  | 'provisioning'
  | 'waiting-for-approval'
  | 'connecting'
  | 'active'
  | 'reconnecting'
  | 'ended'
  | 'expired'
  | 'failed';

export type RecordingMode = 'disabled' | 'optional' | 'required';

export type InputDeviceKind =
  | 'keyboard'
  | 'mouse'
  | 'touch'
  | 'xbox-controller'
  | 'playstation-controller'
  | 'generic-gamepad'
  | 'racing-wheel'
  | 'hotas'
  | 'vr-controller';

export interface StreamBitrateLimits {
  minKbps: number;
  startKbps: number;
  maxKbps: number;
}

export interface StreamResolution {
  width: number;
  height: number;
  refreshRateHz: number;
}

export interface StreamProfile {
  preset: StreamPreset;
  transport: StreamTransport;
  preferredVideoCodecs: VideoCodec[];
  activeVideoCodec?: VideoCodec;
  preferredAudioCodecs: AudioCodec[];
  activeAudioCodec?: AudioCodec;
  bitrate: StreamBitrateLimits;
  resolution: StreamResolution;
  adaptiveBitrate: boolean;
  adaptiveResolution: boolean;
  packetLossRecovery: boolean;
  audioEnabled: boolean;
  latencyTargetMs: number;
  keyframeIntervalMs?: number;
}

export interface NetworkQualitySnapshot {
  rttMs: number;
  jitterMs: number;
  packetLossPercent: number;
  downstreamMbps: number;
  upstreamMbps: number;
  score: number;
  sampledAt: string;
}

export interface RemoteSessionCapabilities {
  clipboardSync: boolean;
  fileTransfer: boolean;
  multiMonitor: boolean;
  audioForwarding: boolean;
  sessionRecording: RecordingMode;
  fullscreen: boolean;
  highRefreshRate: boolean;
  controllerRumble: boolean;
  supportedInputs: InputDeviceKind[];
}

export interface RemoteSessionPermissions {
  requiresUserApproval: boolean;
  allowRemoteControl: boolean;
  allowClipboardRead: boolean;
  allowClipboardWrite: boolean;
  allowFileUpload: boolean;
  allowFileDownload: boolean;
  allowAudioForwarding: boolean;
  allowSessionRecording: boolean;
  maxDurationSeconds: number;
  idleTimeoutSeconds: number;
  approvedByUserId?: string;
  approvedAt?: string;
}

export interface TemporaryAccessLink {
  id: string;
  sessionId: string;
  createdByUserId: string;
  url: string;
  oneTimeUse: boolean;
  permissions: RemoteSessionPermissions;
  createdAt: string;
  expiresAt: string;
  revokedAt?: string;
}

export interface RemoteSession {
  id: string;
  userId: string;
  hostId?: string;
  vmId?: string;
  type: SessionType;
  mode: SessionMode;
  resourceClass: ResourceClass;
  state: RemoteSessionState;
  stream: StreamProfile;
  capabilities: RemoteSessionCapabilities;
  permissions: RemoteSessionPermissions;
  network?: NetworkQualitySnapshot;
  proxyRoutes?: ProxyRoute[];
  createdAt: string;
  updatedAt: string;
  expiresAt?: string;
  endedAt?: string;
}

export interface CreateRemoteSessionRequest {
  type: SessionType;
  mode: SessionMode;
  resourceClass: ResourceClass;
  vmId?: string;
  hostId?: string;
  region?: string;
  gpuPreferred?: boolean;
  gpuRequired?: boolean;
  streamPreset: StreamPreset;
  requestedResolution?: StreamResolution;
  requestedInputs?: InputDeviceKind[];
  requestedCapabilities?: Partial<RemoteSessionCapabilities>;
  permissions?: Partial<RemoteSessionPermissions>;
  proxyRoutes?: ProxyRouteRequest[];
}

export interface CreateRemoteSessionResponse {
  session: RemoteSession;
  signalingUrl: string;
  connectionToken: string;
  estimatedReadySeconds: number;
}

export interface UpdateRemoteSessionPermissionsRequest {
  permissions: Partial<RemoteSessionPermissions>;
}

export interface EndRemoteSessionRequest {
  reason: 'user-requested' | 'timeout' | 'host-maintenance' | 'security' | 'error';
}

export interface RemoteSupportInviteRequest {
  targetDeviceId?: string;
  hostId?: string;
  vmId?: string;
  maxDurationSeconds: number;
  idleTimeoutSeconds: number;
  allowRemoteControl: boolean;
  allowFileTransfer: boolean;
  allowClipboardSync: boolean;
  requireUserApproval: boolean;
}

export interface RemoteSupportInviteResponse {
  session: RemoteSession;
  accessLink: TemporaryAccessLink;
}

export interface SessionAuditLogEntry {
  id: string;
  sessionId: string;
  actorUserId?: string;
  action:
    | 'created'
    | 'approved'
    | 'connected'
    | 'control-granted'
    | 'control-revoked'
    | 'file-uploaded'
    | 'file-downloaded'
    | 'clipboard-synced'
    | 'recording-started'
    | 'recording-stopped'
    | 'proxy-route-opened'
    | 'proxy-route-closed'
    | 'ended';
  ipAddress?: string;
  userAgent?: string;
  metadata?: Record<string, string>;
  createdAt: string;
}

export interface WebRtcSignalMessage {
  sessionId: string;
  type: 'offer' | 'answer' | 'ice-candidate' | 'renegotiate' | 'close';
  payload: unknown;
  sentAt: string;
}
