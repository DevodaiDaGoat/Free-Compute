import type { ReactNode } from 'react';

export interface AppWindow {
  id: string;
  title: string;
  app: string;
  x: number;
  y: number;
  width: number;
  height: number;
  zIndex: number;
  minimized: boolean;
  maximized: boolean;
  focused: boolean;
}

export interface DesktopApp {
  id: string;
  name: string;
  icon: ReactNode;
}

export type StreamPreset = 'safe' | 'fast';
export type EncodingMode = 'speed' | 'quality' | 'balanced';
export type EncoderPreset = 'ultrafast' | 'superfast' | 'veryfast' | 'faster' | 'fast' | 'medium' | 'slow' | 'slower' | 'veryslow' | 'placebo';
export interface ConnectionQualityMetrics {
  rttMs: number;
  jitterMs: number;
  packetLoss: number;
  availableBandwidthKbps: number;
  score: number;
  quality: 'excellent' | 'good' | 'degraded' | 'poor';
  timestamp: number;
}

export type StreamTransport = 'webrtc' | 'websocket-fallback' | 'quic' | 'webtransport';
export type VideoCodec = 'h263' | 'h264' | 'h265' | 'av1' | 'vp8' | 'vp9';
export type AudioCodec = 'opus' | 'aac';
export type GamingMode = 'standard' | 'competitive' | 'casual' | 'vr';
export type SessionMode = 'desktop' | 'development' | 'gaming' | 'remote-support';
export type ResourceClass = 'basic' | 'standard' | 'gaming' | 'workstation';
export type InputDeviceKind =
  | 'keyboard' | 'mouse' | 'touch'
  | 'xbox-controller' | 'playstation-controller' | 'generic-gamepad'
  | 'racing-wheel' | 'hotas' | 'vr-controller';

export interface ConnectionConfig {
  preset: StreamPreset;
  transport: StreamTransport;
  videoCodec: VideoCodec;
  audioCodec: AudioCodec;
  encodingMode: EncodingMode;
  encoderPreset: EncoderPreset;
  encoderHardwareAccel: boolean;
  resolutionWidth: number;
  resolutionHeight: number;
  refreshRate: number;
  fps: number;
  latencyTargetMs: number;
  adaptiveBitrate: boolean;
  adaptiveResolution: boolean;
  packetLossRecovery: boolean;
  audioEnabled: boolean;
  audioAEC: boolean;
  audioNS: boolean;
  audioAGC: boolean;
  clipboardSync: boolean;
  clipboardRead: boolean;
  clipboardWrite: boolean;
  fileTransfer: boolean;
  fileUpload: boolean;
  fileDownload: boolean;
  sessionRecording: boolean;
  controllerRumble: boolean;
  motionControls: boolean;
  hdr: boolean;
  rayTracing: boolean;
  vsync: boolean;
  networkOptimization: boolean;
  connectionFusion: boolean;
  quicMigration: boolean;
  meshRouting: boolean;
  streamPrioritization: boolean;
  gamingMode: GamingMode;
  gpuPreferred: boolean;
  gpuRequired: boolean;
  multiMonitor: boolean;
  fullscreen: boolean;
  highRefreshRate: boolean;
  requireUserApproval: boolean;
  maxDurationMinutes: number;
  idleTimeoutMinutes: number;
  sessionMode: SessionMode;
  resourceClass: ResourceClass;
  supportedInputs: InputDeviceKind[];
}

export interface Preferences {
  connection?: Partial<ConnectionConfig>;
}

export function defaultConnectionConfig(): ConnectionConfig {
  return {
    preset: 'safe',
    transport: 'webrtc',
    videoCodec: 'h264',
    audioCodec: 'opus',
    encodingMode: 'balanced',
    encoderPreset: 'fast',
    encoderHardwareAccel: true,
    resolutionWidth: 1920,
    resolutionHeight: 1080,
    refreshRate: 60,
    fps: 60,
    latencyTargetMs: 50,
    adaptiveBitrate: true,
    adaptiveResolution: true,
    packetLossRecovery: true,
    audioEnabled: true,
    audioAEC: true,
    audioNS: true,
    audioAGC: false,
    clipboardSync: false,
    clipboardRead: false,
    clipboardWrite: false,
    fileTransfer: false,
    fileUpload: false,
    fileDownload: false,
    sessionRecording: false,
    controllerRumble: true,
    motionControls: false,
    hdr: false,
    rayTracing: false,
    vsync: true,
    networkOptimization: false,
    connectionFusion: true,
    quicMigration: true,
    meshRouting: false,
    streamPrioritization: true,
    gamingMode: 'standard',
    gpuPreferred: false,
    gpuRequired: false,
    multiMonitor: false,
    fullscreen: false,
    highRefreshRate: false,
    requireUserApproval: false,
    maxDurationMinutes: 60,
    idleTimeoutMinutes: 10,
    sessionMode: 'desktop',
    resourceClass: 'standard',
    supportedInputs: ['keyboard', 'mouse'],
  };
}
