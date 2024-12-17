package watchdog_config

type Config struct {
	Disabled bool   `hcl:"disabled,optional"`
	Folder   string `hcl:"folder,optional"`
}
