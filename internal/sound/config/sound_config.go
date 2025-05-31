package sound_config

// sound volume use fixed point. 12 = 1.2
type Config struct {
	TTS_exec      []string `hcl:"tts_exec,optional"`
	Folder        string   `hcl:"folder,optional"`
	KeyBeep       string   `hcl:"keyBeep,optional"`
	MoneyIn       string   `hcl:"moneyIn,optional"`
	DefaultVolume int16    `hcl:"default_volume,optional"`
	Disabled      bool     `hcl:"disabled,optional"`
}
