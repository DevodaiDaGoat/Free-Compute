import type { NetworkQualitySnapshot } from './remote';

// Input events sent from frontend to backend
export type InputEvent =
  | MouseMoveEvent
  | MouseButtonEvent
  | KeyboardEvent
  | ScrollEvent
  | TouchInputEvent
  | GamepadInputEvent;

export interface MouseMoveEvent {
  type: 'input.mouse.move';
  x: number;
  y: number;
}

export interface MouseButtonEvent {
  type: 'input.mouse.down' | 'input.mouse.up' | 'input.mouse.click' | 'input.mouse.dblclick';
  x: number;
  y: number;
  button: 'left' | 'right' | 'middle';
}

export interface KeyboardEvent {
  type: 'input.keyboard.press' | 'input.keyboard.release';
  key: string;
  code: string; // Physical key position
  ctrlKey: boolean;
  shiftKey: boolean;
  altKey: boolean;
  metaKey: boolean;
  repeat: boolean;
}

export interface ScrollEvent {
  type: 'input.scroll';
  x: number;
  y: number;
  deltaX: number;
  deltaY: number;
}

export interface TouchPoint {
  id: number;
  x: number;
  y: number;
  pressure?: number;
}

export interface TouchInputEvent {
  type: 'input.touch.start' | 'input.touch.move' | 'input.touch.end' | 'input.touch.cancel';
  touches: TouchPoint[];
}

export interface GamepadButtonState {
  index: number;
  pressed: boolean;
  value: number;
}

export interface GamepadInputEvent {
  type: 'input.gamepad';
  gamepadId: string;
  vendor?: 'xbox' | 'playstation' | 'generic';
  axes: number[];
  buttons: GamepadButtonState[];
  timestamp: number;
}

export interface ClipboardMessage {
  type: 'clipboard.read' | 'clipboard.write';
  mimeType: string;
  data: string;
}

export interface FileTransferMessage {
  type:
    | 'file.upload.request'
    | 'file.upload.chunk'
    | 'file.download.request'
    | 'file.download.chunk'
    | 'file.transfer.complete'
    | 'file.transfer.error';
  transferId: string;
  filename?: string;
  sizeBytes?: number;
  chunkIndex?: number;
  data?: string;
  message?: string;
}

export interface SessionControlMessage {
  type:
    | 'session.approve-control'
    | 'session.revoke-control'
    | 'session.extend'
    | 'session.recording.start'
    | 'session.recording.stop'
    | 'session.fullscreen.enter'
    | 'session.fullscreen.exit';
  sessionId: string;
  requestedDurationSeconds?: number;
}

export interface NetworkQualityMessage {
  type: 'network.quality';
  sessionId: string;
  quality: NetworkQualitySnapshot;
}

// System messages
export interface SystemMessage {
  type: 'system.connected' | 'system.disconnected' | 'system.error';
  message: string;
  timestamp: string;
}

export type ControlChannelMessage =
  | InputEvent
  | ClipboardMessage
  | FileTransferMessage
  | SessionControlMessage
  | NetworkQualityMessage
  | SystemMessage;
