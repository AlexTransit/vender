package sound_config

// sound volume use fixed point. 12 = 1.2
type Config struct {
	Disabled      bool   `hcl:"disabled,optional"`
	DefaultVolume int16  `hcl:"default_volume,optional"`
	Folder        string `hcl:"folder,optional"`
	KeyBeep       string `hcl:"keyBeep,optional"`
	MoneyIn       string `hcl:"moneyIn,optional"`
}
