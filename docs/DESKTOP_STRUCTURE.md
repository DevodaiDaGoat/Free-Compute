# Desktop/WebOS Structure

```
desktop/
├── src/
│   ├── boot/
│   │   ├── BootSequence.tsx       # BIOS → Loading → Login
│   │   ├── BIOSScreen.tsx
│   │   ├── LoadingScreen.tsx
│   │   └── LoginScreen.tsx
│   │
│   ├── desktop/
│   │   ├── Desktop.tsx             # Main desktop container
│   │   ├── Wallpaper.tsx           # Background & theme
│   │   ├── NotificationCenter.tsx  # Notifications
│   │   ├── ContextMenu.tsx         # Right-click menu
│   │   └── VirtualDesktops.tsx
│   │
│   ├── taskbar/
│   │   ├── Taskbar.tsx             # Bottom bar
│   │   ├── StartMenu.tsx           # App launcher
│   │   ├── Clock.tsx               # System clock
│   │   ├── SystemTray.tsx          # Icons/indicators
│   │   └── QuickSettings.tsx
│   │
│   ├── window-manager/
│   │   ├── WindowManager.tsx       # Manages all windows
│   │   ├── Window.tsx              # Window component
│   │   ├── WindowControls.tsx      # Min/Max/Close buttons
│   │   ├── WindowDragHandler.tsx   # Drag logic
│   │   └── WindowResizer.tsx       # Resize logic
│   │
│   ├── apps/
│   │   ├── index.ts                # App registry
│   │   │
│   │   ├── browser/
│   │   │   ├── Browser.tsx
│   │   │   ├── AddressBar.tsx
│   │   │   ├── Tabs.tsx
│   │   │   └── WebView.tsx
│   │   │
│   │   ├── terminal/
│   │   │   ├── Terminal.tsx
│   │   │   ├── TerminalInput.tsx
│   │   │   └── OutputRenderer.tsx
│   │   │
│   │   ├── files/
│   │   │   ├── FileManager.tsx
│   │   │   ├── FileList.tsx
│   │   │   ├── FilePreview.tsx
│   │   │   └── FolderTree.tsx
│   │   │
│   │   ├── settings/
│   │   │   ├── Settings.tsx
│   │   │   ├── Display.tsx
│   │   │   ├── Audio.tsx
│   │   │   └── About.tsx
│   │   │
│   │   ├── store/
│   │   │   ├── Store.tsx
│   │   │   ├── AppGrid.tsx
│   │   │   └── AppDetail.tsx
│   │   │
│   │   ├── task-manager/
│   │   │   ├── TaskManager.tsx
│   │   │   ├── ProcessList.tsx
│   │   │   └── ResourceMonitor.tsx
│   │   │
│   │   └── calculator/
│   │       └── Calculator.tsx
│   │
│   ├── system/
│   │   ├── types.ts                # Shared types
│   │   ├── hooks.ts                # System hooks
│   │   ├── utils.ts                # Utilities
│   │   ├── constants.ts            # Constants
│   │   │
│   │   ├── api/
│   │   │   ├── websocket.ts        # WebRTC/WebSocket connection
│   │   │   ├── input-handler.ts    # Mouse/keyboard
│   │   │   └── file-transfer.ts    # File I/O
│   │   │
│   │   └── stores/
│   │       ├── desktopStore.ts     # Desktop state
│   │       ├── windowStore.ts      # Window state
│   │       ├── appStore.ts         # App state
│   │       └── systemStore.ts      # System metrics
│   │
│   ├── App.tsx                     # Root component
│   └── index.css                   # Global styles
│
├── public/
│   ├── wallpapers/
│   ├── icons/
│   └── sounds/
│
├── next.config.js                  # Next.js config
├── tailwind.config.ts              # Tailwind
├── tsconfig.json
└── package.json
```

## Key Components

### `boot/BootSequence.tsx`
Orchestrates boot animation, loading, and login flow.

### `desktop/Desktop.tsx`
Main container with wallpaper, window manager, taskbar, notifications.

### `window-manager/WindowManager.tsx`
Manages window positioning, z-order, focus, minimize/maximize.

### `apps/[app]/[App].tsx`
Each app is a React component that receives window context.

### `system/api/websocket.ts`
Establishes WebRTC connection to backend, streams desktop frames, handles input.

## Component Hierarchy

```
App
├── BootSequence (if not logged in)
│   ├── BIOSScreen
│   ├── LoadingScreen
│   └── LoginScreen
│
└── Desktop (if logged in)
    ├── Wallpaper
    ├── WindowManager
    │   └── Windows[]
    │       ├── Browser
    │       ├── Terminal
    │       ├── Files
    │       └── ...
    ├── Taskbar
    │   ├── StartMenu
    │   ├── RunningApps
    │   ├── Clock
    │   └── SystemTray
    └── NotificationCenter
```

## Window System

### Window Data Structure
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

### Operations
- Create/destroy windows
- Drag to move
- Resize from edges/corners
- Minimize/maximize/restore
- Alt+Tab to switch
- Click to focus (z-order)

## Input Handling

All user input (mouse, keyboard) is captured and sent to backend via WebSocket:

```typescript
// Mouse move
{
  type: 'input.mouse.move',
  x: 100,
  y: 200
}

// Keyboard
{
  type: 'input.keyboard.press',
  key: 'A',
  ctrlKey: true
}
```

## Streaming

Desktop frames received from backend as WebRTC video stream:
- VP9 or H.264 codec
- Adaptive bitrate based on connection
- Target: 60 FPS, <100ms latency

## Theming

CSS variables for dark/light themes:
```css
--bg-primary: #0a0a0a;
--bg-secondary: #1a1a1a;
--text-primary: #ffffff;
--accent: #18e2ff;
```

## State Management

Zustand stores for:
- Desktop state (theme, wallpaper, settings)
- Window positions and focus
- App state and data
- System metrics (CPU, RAM, disk)
