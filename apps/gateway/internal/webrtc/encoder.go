package webrtc

import "fmt"

type EncodingMode string

const (
	EncodingModeSpeed    EncodingMode = "speed"
	EncodingModeQuality  EncodingMode = "quality"
	EncodingModeBalanced EncodingMode = "balanced"
)

type EncoderPreset string

const (
	EncoderPresetUltrafast EncoderPreset = "ultrafast"
	EncoderPresetSuperfast EncoderPreset = "superfast"
	EncoderPresetVeryfast  EncoderPreset = "veryfast"
	EncoderPresetFaster    EncoderPreset = "faster"
	EncoderPresetFast      EncoderPreset = "fast"
	EncoderPresetMedium    EncoderPreset = "medium"
	EncoderPresetSlow      EncoderPreset = "slow"
	EncoderPresetSlower    EncoderPreset = "slower"
	EncoderPresetVeryslow  EncoderPreset = "veryslow"
	EncoderPresetPlacebo   EncoderPreset = "placebo"
)

type EncoderConfig struct {
	Mode          EncodingMode `json:"mode"`
	Preset        EncoderPreset `json:"preset"`
	CodecProfile  string       `json:"codecProfile,omitempty"`
	CodecLevel    string       `json:"codecLevel,omitempty"`
	PixelFormat   string       `json:"pixelFormat,omitempty"`
	GopSize       int          `json:"gopSize,omitempty"`
	BFrames       int          `json:"bFrames,omitempty"`
	RefFrames     int          `json:"refFrames,omitempty"`
	MaxBitrateKbps int         `json:"maxBitrateKbps,omitempty"`
	CRF           int          `json:"crf,omitempty"`
	QP            int          `json:"qp,omitempty"`
	HardwareAccel bool         `json:"hardwareAccel"`
}

type EncoderProfile struct {
	Codec      string
	Config     EncoderConfig
	Priority   int
}

var codecPriority = map[string]int{
	"h265": 6,
	"h264": 5,
	"av1":  4,
	"vp9":  3,
	"vp8":  2,
	"h263": 1,
}

func DefaultEncoderConfig(mode EncodingMode, hardwareAccel bool) EncoderConfig {
	cfg := EncoderConfig{
		Mode:          mode,
		HardwareAccel: hardwareAccel,
	}

	switch mode {
	case EncodingModeSpeed:
		cfg.Preset = EncoderPresetVeryfast
		cfg.PixelFormat = "yuv420p"
		cfg.GopSize = 120
		cfg.BFrames = 0
		cfg.RefFrames = 2
		cfg.CRF = 28
		cfg.QP = 28
	case EncodingModeQuality:
		cfg.Preset = EncoderPresetSlow
		cfg.PixelFormat = "yuv420p"
		cfg.GopSize = 60
		cfg.BFrames = 3
		cfg.RefFrames = 5
		cfg.CRF = 18
		cfg.QP = 18
	default:
		cfg.Preset = EncoderPresetMedium
		cfg.PixelFormat = "yuv420p"
		cfg.GopSize = 90
		cfg.BFrames = 2
		cfg.RefFrames = 3
		cfg.CRF = 23
		cfg.QP = 23
	}

	if hardwareAccel {
		switch mode {
		case EncodingModeSpeed:
			cfg.Preset = EncoderPresetFast
		case EncodingModeQuality:
			cfg.Preset = EncoderPresetMedium
		default:
			cfg.Preset = EncoderPresetFast
		}
	}

	return cfg
}

func EncoderConfigForCodec(codec string, mode EncodingMode, hardwareAccel bool) EncoderConfig {
	cfg := DefaultEncoderConfig(mode, hardwareAccel)
	cfg.CodecProfile = codecProfileFor(codec, mode)
	cfg.CodecLevel = codecLevelFor(codec)
	return cfg
}

func codecProfileFor(codec string, mode EncodingMode) string {
	switch codec {
	case "h264":
		if mode == EncodingModeQuality {
			return "high"
		}
		return "main"
	case "h265":
		if mode == EncodingModeQuality {
			return "main10"
		}
		return "main"
	case "h263":
		return "baseline"
	default:
		return ""
	}
}

func codecLevelFor(codec string) string {
	switch codec {
	case "h264":
		return "4.1"
	case "h265":
		return "5.1"
	case "h263":
		return "45"
	default:
		return ""
	}
}

func BuildFFmpegEncoderArgs(codec string, cfg EncoderConfig) []string {
	args := []string{}

	switch codec {
	case "h264":
		if cfg.HardwareAccel {
			args = append(args, "-c:v", "h264_nvenc")
		} else {
			args = append(args, "-c:v", "libx264")
		}
	case "h265":
		if cfg.HardwareAccel {
			args = append(args, "-c:v", "hevc_nvenc")
		} else {
			args = append(args, "-c:v", "libx265")
		}
	case "h263":
		if cfg.HardwareAccel {
			args = append(args, "-c:v", "h263_vaapi")
		} else {
			args = append(args, "-c:v", "libx264")
			args = append(args, "-profile:v", "baseline")
			args = append(args, "-x264-params", "annexb=1")
		}
	case "av1":
		if cfg.HardwareAccel {
			args = append(args, "-c:v", "av1_nvenc")
		} else {
			args = append(args, "-c:v", "libaom-av1")
			args = append(args, "-strict", "experimental")
			if cfg.Preset == EncoderPresetUltrafast {
				args = append(args, "-cpu-used", "8")
			}
		}
	case "vp8":
		args = append(args, "-c:v", "libvpx")
	case "vp9":
		args = append(args, "-c:v", "libvpx-vp9")
	}

	if cfg.Preset != "" {
		switch codec {
		case "h264":
			if !cfg.HardwareAccel {
				args = append(args, "-preset", string(cfg.Preset))
			} else {
				nvencPreset := "p2"
				switch cfg.Mode {
				case EncodingModeSpeed:
					nvencPreset = "p1"
				case EncodingModeQuality:
					nvencPreset = "p7"
				}
				args = append(args, "-preset", nvencPreset)
			}
		case "h265":
			if !cfg.HardwareAccel {
				args = append(args, "-preset", string(cfg.Preset))
			} else {
				nvencPreset := "p2"
				switch cfg.Mode {
				case EncodingModeSpeed:
					nvencPreset = "p1"
				case EncodingModeQuality:
					nvencPreset = "p7"
				}
				args = append(args, "-preset", nvencPreset)
			}
		case "vp9":
			if cfg.Preset == EncoderPresetUltrafast || cfg.Preset == EncoderPresetSuperfast {
				args = append(args, "-deadline", "realtime")
				args = append(args, "-cpu-used", "5")
			} else if cfg.Preset == EncoderPresetVeryfast || cfg.Preset == EncoderPresetFaster {
				args = append(args, "-deadline", "realtime")
				args = append(args, "-cpu-used", "2")
			} else {
				args = append(args, "-deadline", "good")
				args = append(args, "-cpu-used", "0")
			}
		case "vp8":
			args = append(args, "-deadline", "realtime")
			args = append(args, "-cpu-used", "5")
		}
	}

	if cfg.CRF > 0 {
		switch codec {
		case "h264", "h265":
			args = append(args, "-crf", fmt.Sprintf("%d", cfg.CRF))
		case "vp9":
			args = append(args, "-crf", fmt.Sprintf("%d", cfg.CRF))
		case "av1":
			args = append(args, "-crf", fmt.Sprintf("%d", cfg.CRF))
		}
	}

	if cfg.QP > 0 {
		switch codec {
		case "h264", "h265":
			if cfg.HardwareAccel {
				args = append(args, "-qp", fmt.Sprintf("%d", cfg.QP))
			}
		}
	}

	if cfg.PixelFormat != "" {
		args = append(args, "-pix_fmt", cfg.PixelFormat)
	}

	if cfg.GopSize > 0 {
		switch codec {
		case "h264", "h265":
			args = append(args, "-g", fmt.Sprintf("%d", cfg.GopSize))
			args = append(args, "-keyint_min", fmt.Sprintf("%d", cfg.GopSize))
		case "vp8", "vp9":
			args = append(args, "-g", fmt.Sprintf("%d", cfg.GopSize))
		}
	}

	if cfg.BFrames >= 0 {
		args = append(args, "-bf", fmt.Sprintf("%d", cfg.BFrames))
	}

	if cfg.RefFrames > 0 {
		switch codec {
		case "h264":
			args = append(args, "-refs", fmt.Sprintf("%d", cfg.RefFrames))
		}
	}

	if cfg.CodecProfile != "" {
		switch codec {
		case "h264":
			args = append(args, "-profile:v", cfg.CodecProfile)
		case "h265":
			if cfg.HardwareAccel {
				args = append(args, "-profile:v", cfg.CodecProfile)
			} else {
				switch cfg.CodecProfile {
				case "main":
					args = append(args, "-x265-params", "profile=main")
				case "main10":
					args = append(args, "-x265-params", "profile=main10")
				}
			}
		}
	}

	if cfg.CodecLevel != "" {
		args = append(args, "-level", cfg.CodecLevel)
	}

	if cfg.MaxBitrateKbps > 0 {
		args = append(args, "-maxrate", fmt.Sprintf("%dk", cfg.MaxBitrateKbps))
		args = append(args, "-bufsize", fmt.Sprintf("%dk", cfg.MaxBitrateKbps*2))
	}

	return args
}

func SelectEncoderProfile(codec string, mode EncodingMode, hardwareAccel bool) (EncoderProfile, error) {
	priority, exists := codecPriority[codec]
	if !exists {
		return EncoderProfile{}, fmt.Errorf("unknown codec: %s", codec)
	}

	config := EncoderConfigForCodec(codec, mode, hardwareAccel)

	return EncoderProfile{
		Codec:    codec,
		Config:   config,
		Priority: priority,
	}, nil
}
