package sound

import (
	"io"
	"os"
	"time"

	"github.com/AlexTransit/vender/helpers"
	"github.com/AlexTransit/vender/log2"
	"github.com/hajimehoshi/ebiten/v2/audio"
	"github.com/hajimehoshi/ebiten/v2/audio/mp3"
)

const sampleRate = 32000

type Sound struct {
	sound        *Config
	log          *log2.Log
	audioContext *audio.Context
	audioPlayer  *audio.Player
	keyBeep      soundStream
	moneyIn      soundStream
	trash        soundStream
}
type soundStream struct {
	Stream []byte
	volume float64
}

// sound volume use fixed point. 12 = 1.2
type Config struct {
	Disabled       bool
	KeyBeep        string
	KeyBeepVolume  int
	Starting       string
	StartingVolume int
	Started        string
	StartedVolume  int
	MoneyIn        string
	MoneyInVolume  int
	Trash          string
	TrashVolume    int
	Broken         string
	BrokenVolume   int
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
	f, err := os.Open(conf.Starting)
	if err != nil {
		s.log.Errorf("open starting (%v)", err)
	}
	str, _ := mp3.DecodeWithoutResampling(f)
	if err != nil {
		s.log.Errorf("decode (%v)", err)
	}
	s.audioPlayer, err = s.audioContext.NewPlayer(str)
	if err != nil {
		s.log.Errorf("new player (%v)", err)
	}

	s.audioPlayer.SetVolume(float64(helpers.ConfigDefaultInt(conf.StartingVolume, 10)) / 10)
	s.audioPlayer.Play()
	go func() {
		s.keyBeep.prepare("keyBeep", conf.KeyBeep, conf.KeyBeepVolume)
		s.moneyIn.prepare("money in", conf.MoneyIn, conf.MoneyInVolume)
		s.trash.prepare("trash", conf.Trash, conf.TrashVolume)
	}()
	s.log.Info("sound module started")
}

func KeyBeep() { playStream(&s.keyBeep) }
func MoneyIn() { playStream(&s.moneyIn) }
func Trash()   { playStream(&s.trash) }
func Started() {
	if !stopPreviewPlay() {
		return
	}
	p := playFile(s.sound.Started, s.sound.StartedVolume)
	p.SetVolume(float64(helpers.ConfigDefaultInt(s.sound.StartingVolume, 10)) / 10)
}

func (ss *soundStream) prepare(name string, file string, volume int) {
	var err error
	ss.Stream, err = loadMp3Steram(file)
	if err != nil {
		s.log.Errorf("load sound %s (%+v)", name, err)
	}
	ss.volume = float64(helpers.ConfigDefaultInt(volume, 10)) / 10
}

// play file and wait finishing
func Broken() {
	if !stopPreviewPlay() {
		return
	}

	p := playFile(s.sound.Broken, s.sound.BrokenVolume)
	p.SetVolume(float64(s.sound.BrokenVolume))
	for {
		if !p.IsPlaying() {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func stopPreviewPlay() (stoped bool) {
	if s.audioPlayer == nil {
		return false
	}
	if s.audioPlayer.IsPlaying() {
		s.audioPlayer.Close()
	}
	return true
}

func loadMp3Steram(file string) ([]byte, error) {
	f, err := os.Open(file)
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

// func playStream(byteStream *[]byte) *audio.Player {
func playStream(ss *soundStream) *audio.Player {
	if s.sound.Disabled {
		return nil
	}
	p := s.audioContext.NewPlayerFromBytes(ss.Stream)
	p.SetVolume(ss.volume)
	p.Play()
	return p
}

func playFile(fileName string, volume int) *audio.Player {
	if s.sound.Disabled {
		return nil
	}
	var ss soundStream
	ss.Stream, _ = loadMp3Steram(fileName)
	ss.volume = (float64(helpers.ConfigDefaultInt(volume, 10)) / 10)
	return playStream(&ss)
}
