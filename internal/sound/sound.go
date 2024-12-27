package sound

import (
	"errors"
	"io"
	"os"
	"time"

	sound_config "github.com/AlexTransit/vender/internal/sound/config"
	"github.com/AlexTransit/vender/log2"
	"github.com/hajimehoshi/ebiten/v2/audio"
	"github.com/hajimehoshi/ebiten/v2/audio/mp3"
)

const sampleRate = 24000

type Sound struct {
	config        *sound_config.Config
	log           *log2.Log
	audioContext  *audio.Context
	audioPlayer   *audio.Player
	keyBeep       soundStream
	moneyIn       soundStream
	currentVolume float64
}
type soundStream struct {
	Stream []byte
}

var s Sound

func Init(conf *sound_config.Config, log *log2.Log, startingVMC bool) {
	s.config = conf
	if conf.Disabled {
		return
	}
	s.log = log
	SetDefaultVolume()
	audioContext := audio.NewContext(sampleRate)
	s.audioContext = audioContext
	if startingVMC {
		go func() {
			// PlayVmcStarting()
			s.keyBeep.prepare("keyBeep", s.config.KeyBeep)
			s.moneyIn.prepare("money in", s.config.MoneyIn)
		}()
	}
}

func PlayKeyBeep() { playStream(&s.keyBeep) }
func PlayMoneyIn() { playStream(&s.moneyIn) }
func SetVolume(v int16) {
	s.currentVolume = float64(v) / 10
}

// set volume from config
// value is fixed point. 10 = 100%
func SetDefaultVolume() {
	s.currentVolume = float64(s.config.DefaultVolume) / 10
}

func PlayFile(file string) error {
	PlayFileNoWait(file)
	waitingEndPlay()
	return nil
}

func PlayFileNoWait(file string) error {
	if err := playMP3controlled(file); err != nil {
		s.log.Errorf(" play file (%v)", err)
		return err
	}
	return nil
}

func Stop() {
	if s.audioPlayer == nil {
		return
	}
	if s.audioPlayer.IsPlaying() {
		s.audioPlayer.Pause()
		s.audioPlayer.Close()
	}
}

func playMP3controlled(file string) (err error) {
	if s.config == nil || s.config.Disabled {
		return nil
	}
	if file == "" {
		return errors.New("play imposible")
	}
	Stop()
	f, err := os.Open(s.config.Folder + file)
	if err != nil {
		return
	}
	str, err := mp3.DecodeWithoutResampling(f)
	if err != nil {
		return
	}
	s.audioPlayer, err = s.audioContext.NewPlayer(str)
	s.audioPlayer.SetVolume(s.currentVolume)
	if err != nil {
		return
	}
	go func() {
		s.audioPlayer.Play()
		waitingEndPlay()
		f.Close()
	}()
	s.log.Infof("play %s", file)
	return nil
}

func waitingEndPlay() {
	for {
		if s.audioPlayer == nil || !s.audioPlayer.IsPlaying() {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func (ss *soundStream) prepare(name string, file string) {
	var err error
	ss.Stream, err = loadSteram(file)
	if err != nil {
		s.log.Errorf("load sound %s (%+v)", name, err)
	}
}

func loadSteram(file string) ([]byte, error) {
	f, err := os.Open(s.config.Folder + file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	bs, err := mp3.DecodeWithSampleRate(sampleRate, f)
	if err != nil {
		return nil, err
	}
	soundStream, err := io.ReadAll(bs)
	return soundStream, err
}

func playStream(ss *soundStream) *audio.Player {
	if s.config.Disabled {
		return nil
	}
	p := s.audioContext.NewPlayerFromBytes(ss.Stream)
	p.SetVolume(s.currentVolume)
	p.Play()
	return p
}
