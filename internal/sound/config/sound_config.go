package sound_config

// sound volume use fixed point. 12 = 1.2
type Config struct {
	// RU: Если true, то звук отключен.
	// EN: If true, the sound is disabled.
	Disabled bool `hcl:"disabled,optional"`
	// RU: Громкость звука в процентах. 100 - это 100%, 50 - это 50% и т.д.
	// EN: Sound volume in percent. 100 is 100%, 50 is 50%, etc.
	DefaultVolume int16 `hcl:"default_volume,optional"`
	// RU: Директрория с аудио файлами. Файлы должны быть в формате MP3, 16 бит, 22050 Гц, моно.
	// EN: Directory with audio files. Files must be in MP3 format, 16 bit, 22050 Hz, mono.
	Folder string `hcl:"folder,optional"`
	// RU: файл для звука при нажатии кнопки.
	// EN: File for sound when a button is pressed.
	KeyBeep string `hcl:"keyBeep,optional"`
	// RU: файл для звука при внесении денег.
	// EN: File for sound when money is inserted.
	MoneyIn string `hcl:"moneyIn,optional"`
	// RU: Команда для генерации TTS звука. текст для озвучивания.
	// EN: Command for generating TTS sound. Text To Sound.
	// Example: ["/home/vmc/vender-db/audio/tts/piper", "--model", "/home/vmc/vender-db/audio/tts/ruslan/voice.onnx", "--config", "/home/vmc/vender-db/audio/tts/ruslan/voice.json"]
	TTSExec []string `hcl:"tts_exec,optional"`
}
