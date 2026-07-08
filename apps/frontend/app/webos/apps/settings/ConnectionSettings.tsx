'use client';

import { useState } from 'react';
import type { ConnectionConfig, StreamPreset, EncodingMode, EncoderPreset, VideoCodec, AudioCodec, GamingMode, SessionMode, ResourceClass, InputDeviceKind, StreamTransport } from '../../system/types';
import { defaultConnectionConfig } from '../../system/types';

type SectionId = 'session' | 'performance' | 'video' | 'audio' | 'input' | 'security' | 'gaming' | 'network';

interface ToggleProps {
  label: string;
  desc?: string;
  value: boolean;
  onChange: (v: boolean) => void;
}

function Toggle({ label, desc, value, onChange }: ToggleProps) {
  return (
    <label style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', padding: '8px 0', cursor: 'pointer' }}>
      <div>
        <div style={{ fontSize: 13, color: '#e6edf3' }}>{label}</div>
        {desc && <div style={{ fontSize: 11, color: '#8b949e', marginTop: 2 }}>{desc}</div>}
      </div>
      <button
        type="button"
        onClick={() => onChange(!value)}
        style={{
          width: 40, height: 22, borderRadius: 11, border: 'none', cursor: 'pointer', position: 'relative',
          background: value ? '#238636' : '#30363d', transition: 'background 0.2s', flexShrink: 0,
        }}
      >
        <span style={{
          position: 'absolute', top: 3, width: 16, height: 16, borderRadius: 8, background: '#fff',
          left: value ? 21 : 3, transition: 'left 0.2s', display: 'block',
        }} />
      </button>
    </label>
  );
}

interface SelectProps<T extends string> {
  label: string;
  desc?: string;
  value: T;
  options: { value: T; label: string; desc?: string }[];
  onChange: (v: T) => void;
}

function Select<T extends string>({ label, desc, value, options, onChange }: SelectProps<T>) {
  return (
    <div style={{ padding: '8px 0' }}>
      <div style={{ fontSize: 13, color: '#e6edf3', marginBottom: 2 }}>{label}</div>
      {desc && <div style={{ fontSize: 11, color: '#8b949e', marginBottom: 6 }}>{desc}</div>}
      <div style={{ display: 'flex', flexWrap: 'wrap', gap: 4 }}>
        {options.map((opt) => (
          <button
            key={opt.value}
            type="button"
            onClick={() => onChange(opt.value)}
            style={{
              padding: '5px 12px', borderRadius: 6, border: '1px solid', fontSize: 12, cursor: 'pointer',
              background: value === opt.value ? '#1f6feb' : '#161b22',
              borderColor: value === opt.value ? '#1f6feb' : '#30363d',
              color: value === opt.value ? '#fff' : '#8b949e',
              transition: 'all 0.15s',
            }}
            title={opt.desc}
          >
            {opt.label}
          </button>
        ))}
      </div>
    </div>
  );
}

interface SliderProps {
  label: string;
  desc?: string;
  value: number;
  min: number;
  max: number;
  step?: number;
  suffix?: string;
  onChange: (v: number) => void;
}

function Slider({ label, desc, value, min, max, step, suffix, onChange }: SliderProps) {
  return (
    <div style={{ padding: '8px 0' }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 4 }}>
        <div>
          <span style={{ fontSize: 13, color: '#e6edf3' }}>{label}</span>
          {desc && <span style={{ fontSize: 11, color: '#8b949e', marginLeft: 8 }}>{desc}</span>}
        </div>
        <span style={{ fontSize: 13, color: '#58a6ff' }}>{value}{suffix}</span>
      </div>
      <input
        type="range"
        min={min}
        max={max}
        step={step ?? 1}
        value={value}
        onChange={(e) => onChange(Number(e.target.value))}
        style={{ width: '100%', accentColor: '#1f6feb', cursor: 'pointer' }}
      />
    </div>
  );
}

function ChipSelect({ label, desc, values, options, onChange }: {
  label: string; desc?: string; values: string[]; options: { value: string; label: string }[]; onChange: (v: string[]) => void;
}) {
  const toggle = (v: string) => {
    onChange(values.includes(v) ? values.filter((x) => x !== v) : [...values, v]);
  };
  return (
    <div style={{ padding: '8px 0' }}>
      <div style={{ fontSize: 13, color: '#e6edf3', marginBottom: 2 }}>{label}</div>
      {desc && <div style={{ fontSize: 11, color: '#8b949e', marginBottom: 6 }}>{desc}</div>}
      <div style={{ display: 'flex', flexWrap: 'wrap', gap: 4 }}>
        {options.map((opt) => (
          <button
            key={opt.value}
            type="button"
            onClick={() => toggle(opt.value)}
            style={{
              padding: '5px 12px', borderRadius: 6, border: '1px solid', fontSize: 12, cursor: 'pointer',
              background: values.includes(opt.value) ? '#1f6feb' : '#161b22',
              borderColor: values.includes(opt.value) ? '#1f6feb' : '#30363d',
              color: values.includes(opt.value) ? '#fff' : '#8b949e',
              transition: 'all 0.15s',
            }}
          >
            {opt.label}
          </button>
        ))}
      </div>
    </div>
  );
}

const sections: { id: SectionId; label: string; icon: string }[] = [
  { id: 'session', label: 'Session', icon: '🖥️' },
  { id: 'performance', label: 'Performance', icon: '⚡' },
  { id: 'video', label: 'Video', icon: '🎬' },
  { id: 'audio', label: 'Audio', icon: '🔊' },
  { id: 'input', label: 'Input', icon: '🎮' },
  { id: 'security', label: 'Security', icon: '🔒' },
  { id: 'gaming', label: 'Gaming', icon: '🎯' },
  { id: 'network', label: 'Network', icon: '🌐' },
];

interface Props {
  config: ConnectionConfig;
  onChange: (config: ConnectionConfig) => void;
}

export default function ConnectionSettings({ config, onChange }: Props) {
  const [activeSection, setActiveSection] = useState<SectionId>('session');
  const [showPresets, setShowPresets] = useState(false);

  const update = <K extends keyof ConnectionConfig>(key: K, value: ConnectionConfig[K]) => {
    onChange({ ...config, [key]: value });
  };

  const applyPreset = (preset: 'safe' | 'fast' | 'gaming' | 'lowest-latency') => {
    switch (preset) {
      case 'safe':
        onChange({ ...defaultConnectionConfig(), preset: 'safe', sessionMode: config.sessionMode });
        break;
      case 'fast':
        onChange({ ...defaultConnectionConfig(), preset: 'fast', encodingMode: 'speed', encoderPreset: 'ultrafast', adaptiveResolution: true, packetLossRecovery: true, latencyTargetMs: 20, fps: 120, highRefreshRate: true });
        break;
      case 'gaming':
        onChange({ ...defaultConnectionConfig(), preset: 'fast', sessionMode: 'gaming', encodingMode: 'speed', encoderPreset: 'ultrafast', videoCodec: 'h265', latencyTargetMs: 15, fps: 144, refreshRate: 144, gpuPreferred: true, controllerRumble: true, networkOptimization: true, highRefreshRate: true });
        break;
      case 'lowest-latency':
        onChange({ ...defaultConnectionConfig(), preset: 'fast', encodingMode: 'speed', encoderPreset: 'ultrafast', videoCodec: 'h264', adaptiveBitrate: false, adaptiveResolution: false, packetLossRecovery: false, latencyTargetMs: 5, fps: 60, audioEnabled: false });
        break;
    }
    setShowPresets(false);
  };

  const navStyle = (id: SectionId): React.CSSProperties => ({
    padding: '8px 16px', borderRadius: 6, border: 'none', cursor: 'pointer', fontSize: 12, textAlign: 'left',
    background: activeSection === id ? '#1f6feb' : 'transparent', color: activeSection === id ? '#fff' : '#8b949e',
    transition: 'all 0.15s', display: 'flex', alignItems: 'center', gap: 8, width: '100%',
  });

  return (
    <div style={{ display: 'flex', height: '100%', color: '#e6edf3', fontFamily: 'system-ui, sans-serif', background: '#0d1117' }}>
      <nav style={{ width: 160, padding: 12, borderRight: '1px solid #21262d', display: 'flex', flexDirection: 'column', gap: 2, flexShrink: 0, overflow: 'auto' }}>
        {sections.map((s) => (
          <button key={s.id} type="button" style={navStyle(s.id)} onClick={() => setActiveSection(s.id)}>
            <span>{s.icon}</span> {s.label}
          </button>
        ))}
        <div style={{ flex: 1 }} />
        <button
          type="button"
          onClick={() => setShowPresets(!showPresets)}
          style={{ padding: '7px 12px', borderRadius: 6, border: '1px solid #30363d', cursor: 'pointer', fontSize: 11, background: '#161b22', color: '#8b949e', marginTop: 8 }}
        >
          {showPresets ? 'Hide Presets' : 'Quick Presets'}
        </button>
      </nav>

      <div style={{ flex: 1, padding: 16, overflow: 'auto' }}>
        {showPresets && (
          <div style={{ marginBottom: 16, background: '#161b22', borderRadius: 8, padding: 12, border: '1px solid #30363d' }}>
            <div style={{ fontSize: 13, fontWeight: 600, marginBottom: 8, color: '#e6edf3' }}>Quick Presets</div>
            <div style={{ display: 'flex', gap: 8 }}>
              {([
                ['🛡️', 'Safe Mode', 'safe' as const, 'Compatibility first, conservative bitrate, strong recovery'],
                ['🚀', 'Fast Mode', 'fast' as const, 'Low latency, high bitrate, hardware encoding'],
                ['🎮', 'Gaming', 'gaming' as const, 'Lowest latency, GPU preferred, controller support'],
                ['⚡', 'Lowest Latency', 'lowest-latency' as const, 'Minimal latency, no audio, ultra-fast encode'],
              ] as const).map(([icon, name, id, desc]) => (
                <button key={id} type="button" onClick={() => applyPreset(id)}
                  style={{ flex: 1, padding: 12, borderRadius: 8, border: '1px solid #30363d', cursor: 'pointer', background: '#0d1117', color: '#e6edf3', textAlign: 'left' }}>
                  <div style={{ fontSize: 20, marginBottom: 4 }}>{icon}</div>
                  <div style={{ fontSize: 13, fontWeight: 600 }}>{name}</div>
                  <div style={{ fontSize: 11, color: '#8b949e', marginTop: 4 }}>{desc}</div>
                </button>
              ))}
            </div>
          </div>
        )}

        {activeSection === 'session' && (
          <div>
            <SectionHeader title="Session Mode" desc="Choose how your remote desktop behaves" />
            <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 8, marginBottom: 16 }}>
              {([
                ['🖥️', 'Desktop', 'desktop' as const, 'General cloud desktop access, apps, browsing'],
                ['💻', 'Development', 'development' as const, 'Browser IDEs, terminals, SSH, app preview'],
                ['🎮', 'Gaming', 'gaming' as const, 'Fullscreen game streaming, low latency, controller'],
                ['🔐', 'Remote Support', 'remote-support' as const, 'Temporary access, approval, audit logs'],
              ] as const).map(([icon, name, value, desc]) => (
                <button key={value} type="button" onClick={() => update('sessionMode', value)}
                  style={{
                    padding: 16, borderRadius: 8, border: '1px solid', cursor: 'pointer', textAlign: 'left', transition: 'all 0.15s',
                    background: config.sessionMode === value ? '#1f6feb22' : '#161b22',
                    borderColor: config.sessionMode === value ? '#1f6feb' : '#30363d',
                  }}>
                  <div style={{ fontSize: 24, marginBottom: 4 }}>{icon}</div>
                  <div style={{ fontSize: 14, fontWeight: 600, color: config.sessionMode === value ? '#58a6ff' : '#e6edf3' }}>{name}</div>
                  <div style={{ fontSize: 11, color: '#8b949e', marginTop: 4 }}>{desc}</div>
                </button>
              ))}
            </div>

            <SectionHeader title="Resource Class" desc="Hardware allocation for your session" />
            <Select<ResourceClass>
              label=""
              value={config.resourceClass}
              options={[
                { value: 'basic', label: 'Basic', desc: '1 vCPU, 2GB RAM — light tasks' },
                { value: 'standard', label: 'Standard', desc: '2 vCPU, 4GB RAM — general use' },
                { value: 'gaming', label: 'Gaming', desc: '4 vCPU, 8GB RAM + GPU — gaming/graphics' },
                { value: 'workstation', label: 'Workstation', desc: '8 vCPU, 16GB RAM + GPU — heavy workloads' },
              ]}
              onChange={(v) => update('resourceClass', v)}
            />

            <Toggle label="GPU Preferred" desc="Request GPU if available (fallback to CPU otherwise)" value={config.gpuPreferred} onChange={(v) => update('gpuPreferred', v)} />
            <Toggle label="GPU Required" desc="Session fails if no GPU host is available" value={config.gpuRequired} onChange={(v) => update('gpuRequired', v)} />
            <Slider label="Max Duration" desc="Session auto-ends after this" value={config.maxDurationMinutes} min={5} max={480} step={5} suffix="m" onChange={(v) => update('maxDurationMinutes', v)} />
            <Slider label="Idle Timeout" desc="Auto-end if inactive" value={config.idleTimeoutMinutes} min={1} max={120} suffix="m" onChange={(v) => update('idleTimeoutMinutes', v)} />
          </div>
        )}

        {activeSection === 'performance' && (
          <div>
            <SectionHeader title="Performance Tuning" desc="Balance between speed and quality" />
            <Select<StreamPreset>
              label="Stream Preset" desc="Safe = compatibility, Fast = low latency"
              value={config.preset}
              options={[
                { value: 'safe', label: 'Safe', desc: 'Conservative, H.264, strong recovery' },
                { value: 'fast', label: 'Fast', desc: 'Low latency, H.264/H.265, high bitrate' },
              ]}
              onChange={(v) => update('preset', v)}
            />
            <Select<EncodingMode>
              label="Encoding Mode"
              value={config.encodingMode}
              options={[
                { value: 'speed', label: 'Speed', desc: 'Ultra-fast encoding, lower quality' },
                { value: 'balanced', label: 'Balanced', desc: 'Good quality/speed tradeoff' },
                { value: 'quality', label: 'Quality', desc: 'Best quality, higher latency' },
              ]}
              onChange={(v) => update('encodingMode', v)}
            />
            <Select<EncoderPreset>
              label="Encoder Preset" desc="FFmpeg/NVENC preset (ultrafast = lowest latency)"
              value={config.encoderPreset}
              options={[
                { value: 'ultrafast', label: 'Ultrafast' },
                { value: 'superfast', label: 'Superfast' },
                { value: 'veryfast', label: 'Veryfast' },
                { value: 'faster', label: 'Faster' },
                { value: 'fast', label: 'Fast' },
                { value: 'medium', label: 'Medium' },
                { value: 'slow', label: 'Slow' },
                { value: 'slower', label: 'Slower' },
                { value: 'veryslow', label: 'Veryslow' },
                { value: 'placebo', label: 'Placebo' },
              ]}
              onChange={(v) => update('encoderPreset', v)}
            />
            <Toggle label="Hardware Acceleration" desc="Use GPU encoder (NVENC/AMF/VAAPI)" value={config.encoderHardwareAccel} onChange={(v) => update('encoderHardwareAccel', v)} />
            <Toggle label="Adaptive Bitrate" desc="Auto-adjust bitrate based on network" value={config.adaptiveBitrate} onChange={(v) => update('adaptiveBitrate', v)} />
            <Toggle label="Adaptive Resolution" desc="Auto-adjust resolution when congested" value={config.adaptiveResolution} onChange={(v) => update('adaptiveResolution', v)} />
            <Toggle label="Packet Loss Recovery" desc="FEC/retransmission for lossy networks" value={config.packetLossRecovery} onChange={(v) => update('packetLossRecovery', v)} />
          </div>
        )}

        {activeSection === 'video' && (
          <div>
            <SectionHeader title="Video Settings" desc="Codec, resolution, and framerate" />
            <Select<VideoCodec>
              label="Video Codec"
              value={config.videoCodec}
              options={[
                { value: 'h264', label: 'H.264', desc: 'Best compatibility, good quality' },
                { value: 'h265', label: 'H.265/HEVC', desc: 'Better quality at same bitrate' },
                { value: 'vp8', label: 'VP8', desc: 'Open codec, good fallback' },
                { value: 'vp9', label: 'VP9', desc: 'Open codec, high quality, slower encode' },
                { value: 'av1', label: 'AV1', desc: 'Best compression, requires GPU encode' },
                { value: 'h263', label: 'H.263', desc: 'Legacy, minimal latency' },
              ]}
              onChange={(v) => update('videoCodec', v)}
            />
            <Select<StreamTransport>
              label="Stream Transport"
              value={config.transport}
              options={[
                { value: 'quic', label: 'QUIC', desc: 'Ultra-fast 0-RTT, connection migration, multiplexed (experimental)' },
                { value: 'webtransport', label: 'WebTransport', desc: 'QUIC-based, multiplexed streams, native browser API (experimental)' },
                { value: 'webrtc', label: 'WebRTC', desc: 'Low latency, UDP preferred, best for real-time' },
                { value: 'websocket-fallback', label: 'WebSocket', desc: 'TCP fallback, through firewalls' },
              ]}
              onChange={(v) => update('transport', v)}
            />
            <Select<string>
              label="Resolution"
              value={`${config.resolutionWidth}x${config.resolutionHeight}`}
              options={[
                { value: '1280x720', label: '720p' },
                { value: '1920x1080', label: '1080p' },
                { value: '2560x1440', label: '1440p' },
                { value: '3840x2160', label: '4K' },
              ]}
              onChange={(v) => {
                const [w, h] = v.split('x').map(Number);
                update('resolutionWidth', w);
                update('resolutionHeight', h);
              }}
            />
            <Slider label="Frame Rate" value={config.fps} min={15} max={240} step={15} suffix=" FPS" onChange={(v) => update('fps', v)} />
            <Slider label="Refresh Rate" value={config.refreshRate} min={30} max={240} step={30} suffix=" Hz" onChange={(v) => update('refreshRate', v)} />
            <Slider label="Latency Target" desc="Lower = more responsive, higher = smoother" value={config.latencyTargetMs} min={5} max={200} step={5} suffix="ms" onChange={(v) => update('latencyTargetMs', v)} />
            <Toggle label="High Refresh Rate" desc="Enable 120Hz+ support" value={config.highRefreshRate} onChange={(v) => update('highRefreshRate', v)} />
            <Toggle label="Fullscreen Mode" desc="Start session in fullscreen" value={config.fullscreen} onChange={(v) => update('fullscreen', v)} />
            <Toggle label="Multi Monitor" desc="Enable multiple virtual displays" value={config.multiMonitor} onChange={(v) => update('multiMonitor', v)} />
          </div>
        )}

        {activeSection === 'audio' && (
          <div>
            <SectionHeader title="Audio Settings" desc="Audio codec and processing" />
            <Toggle label="Audio Enabled" desc="Stream audio from remote desktop" value={config.audioEnabled} onChange={(v) => update('audioEnabled', v)} />
            <Select<AudioCodec>
              label="Audio Codec"
              value={config.audioCodec}
              options={[
                { value: 'opus', label: 'Opus', desc: 'Best quality, low latency, open' },
                { value: 'aac', label: 'AAC', desc: 'Wide compatibility, good quality' },
              ]}
              onChange={(v) => update('audioCodec', v)}
            />
            <Toggle label="Echo Cancellation (AEC)" desc="Remove echo from microphone" value={config.audioAEC} onChange={(v) => update('audioAEC', v)} />
            <Toggle label="Noise Suppression (NS)" desc="Filter background noise" value={config.audioNS} onChange={(v) => update('audioNS', v)} />
            <Toggle label="Auto Gain Control (AGC)" desc="Normalize volume levels" value={config.audioAGC} onChange={(v) => update('audioAGC', v)} />
          </div>
        )}

        {activeSection === 'input' && (
          <div>
            <SectionHeader title="Input Devices" desc="Enable input methods for your session" />
            <ChipSelect
              label="Supported Inputs"
              desc="Select which input devices to enable"
              values={config.supportedInputs}
              options={[
                { value: 'keyboard', label: '⌨️ Keyboard' },
                { value: 'mouse', label: '🖱️ Mouse' },
                { value: 'touch', label: '👆 Touch' },
                { value: 'xbox-controller', label: '🎮 Xbox' },
                { value: 'playstation-controller', label: '🎮 PlayStation' },
                { value: 'generic-gamepad', label: '🎮 Gamepad' },
                { value: 'racing-wheel', label: '🏎️ Racing Wheel' },
                { value: 'hotas', label: '✈️ HOTAS' },
                { value: 'vr-controller', label: '🥽 VR Controller' },
              ]}
              onChange={(v) => update('supportedInputs', v as InputDeviceKind[])}
            />
          </div>
        )}

        {activeSection === 'security' && (
          <div>
            <SectionHeader title="Security & Permissions" desc="Control access and data sharing" />
            <Toggle label="Require User Approval" desc="Remote support: approve control requests" value={config.requireUserApproval} onChange={(v) => update('requireUserApproval', v)} />
            <Toggle label="Clipboard Sync" desc="Enable clipboard sharing" value={config.clipboardSync} onChange={(v) => update('clipboardSync', v)} />
            {config.clipboardSync && (
              <div style={{ paddingLeft: 20 }}>
                <Toggle label="Clipboard Read" desc="Allow reading clipboard from remote" value={config.clipboardRead} onChange={(v) => update('clipboardRead', v)} />
                <Toggle label="Clipboard Write" desc="Allow writing to remote clipboard" value={config.clipboardWrite} onChange={(v) => update('clipboardWrite', v)} />
              </div>
            )}
            <Toggle label="File Transfer" desc="Enable file transfer between local and remote" value={config.fileTransfer} onChange={(v) => update('fileTransfer', v)} />
            {config.fileTransfer && (
              <div style={{ paddingLeft: 20 }}>
                <Toggle label="File Upload" desc="Allow uploading files to remote" value={config.fileUpload} onChange={(v) => update('fileUpload', v)} />
                <Toggle label="File Download" desc="Allow downloading from remote" value={config.fileDownload} onChange={(v) => update('fileDownload', v)} />
              </div>
            )}
            <Toggle label="Session Recording" desc="Record session for audit" value={config.sessionRecording} onChange={(v) => update('sessionRecording', v)} />
          </div>
        )}

        {activeSection === 'gaming' && (
          <div>
            <SectionHeader title="Gaming Optimizations" desc="Fine-tune for game streaming" />
            <Select<GamingMode>
              label="Gaming Mode"
              value={config.gamingMode}
              options={[
                { value: 'standard', label: 'Standard', desc: '60 FPS, 20ms latency target' },
                { value: 'competitive', label: 'Competitive', desc: '144 FPS, 10ms latency, network optimization' },
                { value: 'casual', label: 'Casual', desc: '30 FPS, 50ms latency, lower bandwidth' },
                { value: 'vr', label: 'VR', desc: '90 FPS, 5ms latency, motion controls' },
              ]}
              onChange={(v) => update('gamingMode', v)}
            />
            <Toggle label="Controller Rumble" desc="Enable vibration feedback" value={config.controllerRumble} onChange={(v) => update('controllerRumble', v)} />
            <Toggle label="Motion Controls" desc="Gyroscope/accelerometer input" value={config.motionControls} onChange={(v) => update('motionControls', v)} />
            <Toggle label="HDR" desc="High dynamic range video" value={config.hdr} onChange={(v) => update('hdr', v)} />
            <Toggle label="Ray Tracing" desc="Request RT-capable GPU" value={config.rayTracing} onChange={(v) => update('rayTracing', v)} />
            <Toggle label="VSync" desc="Synchronize frame output" value={config.vsync} onChange={(v) => update('vsync', v)} />
            <Toggle label="Network Optimization" desc="Prioritize game traffic, reduce jitter" value={config.networkOptimization} onChange={(v) => update('networkOptimization', v)} />
          </div>
        )}

        {activeSection === 'network' && (
          <div>
            <SectionHeader title="Network & Transport" desc="Connection protocol and transport tuning" />
            <Slider label="Latency Target" value={config.latencyTargetMs} min={5} max={200} step={5} suffix="ms" onChange={(v) => update('latencyTargetMs', v)} />
            <Toggle label="Adaptive Bitrate" desc="Auto-adjust bitrate to network conditions" value={config.adaptiveBitrate} onChange={(v) => update('adaptiveBitrate', v)} />
            <Toggle label="Adaptive Resolution" desc="Downscale resolution when congested" value={config.adaptiveResolution} onChange={(v) => update('adaptiveResolution', v)} />
            <Toggle label="Packet Loss Recovery" desc="FEC and retransmission for reliability" value={config.packetLossRecovery} onChange={(v) => update('packetLossRecovery', v)} />
            <div style={{ height: 1, background: '#21262d', margin: '12px 0' }} />
            <SectionHeader title="Advanced Networking" desc="Connection fusion, QUIC, and mesh routing" />
            <Toggle label="Connection Fusion" desc="Merge all streams into single QUIC/WebTransport connection for lower overhead" value={config.connectionFusion} onChange={(v) => update('connectionFusion', v)} />
            <Toggle label="QUIC Connection Migration" desc="Seamless network handoff (WiFi ↔ cellular) without session drop" value={config.quicMigration} onChange={(v) => update('quicMigration', v)} />
            <Toggle label="Mesh Routing" desc="Route through lowest-latency global relay mesh node" value={config.meshRouting} onChange={(v) => update('meshRouting', v)} />
            <Toggle label="Stream Prioritization" desc="Prioritize audio/input over video for best responsiveness" value={config.streamPrioritization} onChange={(v) => update('streamPrioritization', v)} />
            <Toggle label="Network Optimization" desc="DSCP tagging, jitter buffer tuning, BBR congestion control" value={config.networkOptimization} onChange={(v) => update('networkOptimization', v)} />
          </div>
        )}

        <div style={{ height: 40 }} />
      </div>
    </div>
  );
}

function SectionHeader({ title, desc }: { title: string; desc?: string }) {
  return (
    <div style={{ marginBottom: 12 }}>
      <div style={{ fontSize: 16, fontWeight: 600, color: '#e6edf3' }}>{title}</div>
      {desc && <div style={{ fontSize: 12, color: '#8b949e', marginTop: 2 }}>{desc}</div>}
      <div style={{ height: 1, background: '#21262d', marginTop: 8 }} />
    </div>
  );
}
