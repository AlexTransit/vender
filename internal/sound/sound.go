package sound

import (
	"io"
	"os"

	"github.com/AlexTransit/vender/log2"
	"github.com/hajimehoshi/ebiten/v2/audio"
	"github.com/hajimehoshi/ebiten/v2/audio/mp3"
)

const sampleRate = 48000

type Sound struct {
	sound         *Config
	log           *log2.Log
	audioContext  *audio.Context
	audioPlayer   *audio.Player
	keyBeepStream []byte
	moneyInStream []byte
	trash         []byte
}

type Config struct {
	Disabled bool   `hcl:"sound_disabled"`
	KeyBeep  string `hcl:"sound_key"`
	Starting string `hcl:"sound_starting"`
	Started  string `hcl:"sound_started"`
	MoneyIn  string `hcl:"sound_money_in"`
	Trash    string `hcl:"sound_trash"`
}

var s Sound

func Init(conf *Config, log *log2.Log) {
	s.sound = conf

	if conf.Disabled {
		return
	}
	s.log = log
	audioContext := audio.NewContext(sampleRate)
	s.audioContext = audioContext
	s.playfile(s.sound.Starting)
	s.keyBeepStream = s.loadMp3Steram(s.sound.KeyBeep)
	s.moneyInStream = s.loadMp3Steram(s.sound.MoneyIn)
	s.trash = s.loadMp3Steram(s.sound.Trash)
	s.log.Info("sound module started")
}

func (s *Sound) playfile(file string) {
	f, _ := os.Open(file)
	bs, _ := mp3.DecodeWithoutResampling(f)
	s.audioPlayer, _ = s.audioContext.NewPlayer(bs)
	s.audioPlayer.Play()
}

func (s *Sound) loadMp3Steram(file string) []byte {
	f, err := os.Open(file)
	if err != nil {
		s.log.Errorf("error open file: %v (%v)", file, err)
		return nil
	}
	defer f.Close()
	bs, err := mp3.DecodeWithoutResampling(f)
	if err != nil {
		s.log.Errorf("error decode sound:%v (%v)", file, err)
		return nil
	}
	soundStream, _ := io.ReadAll(bs)
	return soundStream
}

func KeyBeep() {
	p := s.audioContext.NewPlayerFromBytes(s.keyBeepStream)
	p.SetVolume(1.5)
	p.Play()
}

func MoneyIn() {
	p := s.audioContext.NewPlayerFromBytes(s.moneyInStream)
	p.SetVolume(1.5)
	p.Play()
}

func Started() {
	if s.audioPlayer != nil && s.audioPlayer.IsPlaying() {
		s.audioPlayer.Close()
	}
	p := s.audioContext.NewPlayerFromBytes(s.loadMp3Steram(s.sound.Started))
	p.Play()
}

func Trash() {
	p := s.audioContext.NewPlayerFromBytes(s.trash)
	p.SetVolume(1.5)
	p.Play()
}
