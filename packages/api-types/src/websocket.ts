// Input events sent from frontend to backend
export type InputEvent = MouseMoveEvent | MouseButtonEvent | KeyboardEvent | ScrollEvent;

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

// System messages
export interface SystemMessage {
  type: 'system.connected' | 'system.disconnected' | 'system.error';
  message: string;
  timestamp: string;
}
