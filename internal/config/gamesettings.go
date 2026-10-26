package config

type GameSettings struct {
	WineVersion          WineVersion        `json:"wine_version"`
	WinePrefix           string             `json:"wine_prefix"`
	TargetExe            string             `json:"target_exe"`
	LauncherArgs         string             `json:"launcher_args"`
	Environment          []EnvVar           `json:"environment"`
	Wrappers             []Wrapper          `json:"wrappers"`
	UseGameMode          bool               `json:"use_gamemode"`
	ShowMangoHud         bool               `json:"show_mangohud"`
	Gamescope            *GamescopeSettings `json:"gamescope"`
	EnableEsync          bool               `json:"enable_esync"`
	EnableFsync          bool               `json:"enable_fsync"`
	EnableFSR            bool               `json:"enable_fsr"`
	MaxSharpness         int                `json:"max_sharpness"`
	AutoInstallDxvk      bool               `json:"auto_install_dxvk"`
	AutoInstallVkd3d     bool               `json:"auto_install_vkd3d"`
	AutoInstallDxvkNvapi bool               `json:"auto_install_dxvk_nvapi"`
	EacRuntime           bool               `json:"eac_runtime"`
	BattlEyeRuntime      bool               `json:"battleye_runtime"`
	EnableEOSOverlay     bool               `json:"enable_eos_overlay"`
	NvidiaPrime          bool               `json:"nvidia_prime"`
	ShowFPS              bool               `json:"show_fps"`
	EnableWineWayland    bool               `json:"enable_wine_wayland"`
	EnableHDR            bool               `json:"enable_hdr"`
	EnableWoW64          bool               `json:"enable_wow64"`
	PreferSystemLibs     bool               `json:"prefer_system_libs"`
	OfflineMode          bool               `json:"offline_mode"`
	DisableUMU           bool               `json:"disable_umu"`
	Language             string             `json:"language"`
	SavesPath            string             `json:"saves_path,omitempty"`
	BeforeScript         string             `json:"before_script"`
	AfterScript          string             `json:"after_script"`
}

type WineVersion struct {
	Bin  string `json:"bin"`
	Name string `json:"name"`
	Type string `json:"type"`
}

type EnvVar struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type Wrapper struct {
	Exe  string `json:"exe"`
	Args string `json:"args"`
}

type GamescopeSettings struct {
	EnableLimiter     bool   `json:"enable_limiter"`
	EnableUpscaling   bool   `json:"enable_upscaling"`
	GameWidth         string `json:"game_width"`
	GameHeight        string `json:"game_height"`
	UpscaleWidth      string `json:"upscale_width"`
	UpscaleHeight     string `json:"upscale_height"`
	UpscaleMethod     string `json:"upscale_method"`
	WindowType        string `json:"window_type"`
	FpsLimiter        string `json:"fps_limiter"`
	AdditionalOptions string `json:"additional_options"`
}
