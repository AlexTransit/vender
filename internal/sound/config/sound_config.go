package sound_config

// sound volume use fixed point. 12 = 1.2
type Config struct {
	Disabled      bool   `hcl:"disabled,optional"`
	Folder        string `hcl:"folder,optional"`
	KeyBeep       string `hcl:"keyBeep,optional"`
	KeyBeepVolume int    `hcl:"keyBeepVolume,optional"`
	MoneyIn       string `hcl:"moneyIn,optional"`
	MoneyInVolume int    `hcl:"moneyInVolume,optional"`
}
