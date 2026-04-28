package watchdog_config

type Config struct {
	// RU: Если true, то WatchDog для systemd отключен. Если отключить WatchDog, то при зависании сервиса он не будет перезапущен.
	// EN: If true, the WatchDog for systemd is disabled. If you disable the WatchDog, then when the service hangs, it will not be restarted.
	Disabled bool `hcl:"disabled,optional"`
	// RU: когда автомат закончил готовить то создается папка на RamDrive. если в момент приготовления было отключение питания, то папки нет и нужно сделать инициальзацию с выдачей стакана.
	// EN: when the automaton finished preparing, a folder is created on RamDrive. if there was a power outage during preparation, then there is no folder and initialization with cup dispensing is required.
	Folder string `hcl:"folder,optional"`
}
