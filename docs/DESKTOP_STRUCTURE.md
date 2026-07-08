# Desktop/WebOS Structure

## Location

The WebOS desktop lives in `apps/frontend/app/webos/` (not a top-level `desktop/` directory).

## File Tree

```
webos/
├── apps
│   ├── admin
│   │   └── Admin.tsx
│   ├── browser
│   │   └── Browser.tsx
│   ├── calculator
│   │   └── Calculator.tsx
│   ├── files
│   │   └── Files.tsx
│   ├── settings
│   │   ├── ConnectionSettings.tsx
│   │   └── Settings.tsx
│   ├── store
│   ├── task-manager
│   └── terminal
│       └── Terminal.tsx
├── boot
│   └── BootSequence.tsx
├── desktop
│   └── Desktop.tsx
├── page.tsx
├── system
│   ├── api
│   │   └── websocket.ts
│   ├── stores
│   └── types.ts
├── taskbar
│   └── Taskbar.tsx
└── window-manager
    ├── Window.tsx
    └── WindowManager.tsx


## State Management

Zustand stores for:
- Desktop state (theme, wallpaper, settings)
- Window positions and focus
- App state and data
- System metrics (CPU, RAM, disk)

## Window Data Structure

```typescript
interface Window {
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
```

## Input Handling

All user input (mouse, keyboard) is captured and sent to backend via WebSocket.

## Streaming

Desktop frames received from backend as WebRTC video stream:
- VP9 or H.264 codec
- Adaptive bitrate based on connection
- Target: 60 FPS, <100ms latency
