package sound

import (
	"io"
	"os"
	"time"

	"github.com/AlexTransit/vender/log2"
	"github.com/hajimehoshi/ebiten/v2/audio"
	"github.com/hajimehoshi/ebiten/v2/audio/mp3"
)

const sampleRate = 32000

type Sound struct {
	sound         *Config
	log           *log2.Log
	audioContext  *audio.Context
	audioPlayer   *audio.Player
	keyBeepStream []byte
	moneyInStream []byte
	trashStream   []byte
}

type Config struct {
	Disabled bool   `hcl:"sound_disabled"`
	KeyBeep  string `hcl:"sound_key"`
	Starting string `hcl:"sound_starting"`
	Started  string `hcl:"sound_started"`
	MoneyIn  string `hcl:"sound_money_in"`
	Trash    string `hcl:"sound_trash"`
	Broken   string `hcl:"sound_broken"`
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
	f, _ := os.Open(s.sound.Starting)
	str, _ := mp3.DecodeWithoutResampling(f)
	s.audioPlayer, _ = s.audioContext.NewPlayer(str)
	s.audioPlayer.Play()
	go func() {
		s.keyBeepStream = s.loadMp3Steram(s.sound.KeyBeep)
		s.moneyInStream = s.loadMp3Steram(s.sound.MoneyIn)
		s.trashStream = s.loadMp3Steram(s.sound.Trash)
	}()
	s.log.Info("sound module started")
}

func KeyBeep() { s.playStream(&s.keyBeepStream) }
func MoneyIn() { s.playStream(&s.moneyInStream) }
func Trash()   { s.playStream(&s.trashStream) }
func Started() {
	if !s.stopPreviewPlay() {
		return
	}
	s.playFile(s.sound.Started)
}

// play file and wait finishing
func Broken() {
	if !s.stopPreviewPlay() {
		return
	}
	p := s.playFile(s.sound.Broken)
	for {
		if !p.IsPlaying() {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func (s *Sound) stopPreviewPlay() (stoped bool) {
	if s.audioPlayer == nil {
		return false
	}
	if s.audioPlayer.IsPlaying() {
		s.audioPlayer.Close()
	}
	return true
}

func (s *Sound) loadMp3Steram(file string) []byte {
	f, err := os.Open(file)
	if err != nil {
		s.log.Errorf("error open file: %v (%v)", file, err)
		return nil
	}
	defer f.Close()
	bs, err := mp3.DecodeWithSampleRate(sampleRate, f)
	if err != nil {
		s.log.Errorf("error decode sound:%v (%v)", file, err)
		return nil
	}
	soundStream, _ := io.ReadAll(bs)
	return soundStream
}

func (s *Sound) playStream(byteStream *[]byte) *audio.Player {
	p := s.audioContext.NewPlayerFromBytes(*byteStream)
	p.SetVolume(1.5)
	p.Play()
	return p
}

func (s *Sound) playFile(fileName string) *audio.Player {
	bs := s.loadMp3Steram(fileName)
	return s.playStream(&bs)
}
