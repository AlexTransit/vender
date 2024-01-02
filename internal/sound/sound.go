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
	f, _ := os.Open(conf.Starting)
	str, _ := mp3.DecodeWithoutResampling(f)
	s.audioPlayer, _ = s.audioContext.NewPlayer(str)
	s.audioPlayer.SetVolume(float64(helpers.ConfigDefaultInt(conf.StartingVolume, 10)) / 10)
	s.audioPlayer.Play()
	go func() {
		s.keyBeep.prepare("keyBeep", conf.KeyBeep, conf.KeyBeepVolume)
		s.moneyIn.prepare("money in", conf.MoneyIn, conf.MoneyInVolume)
		s.trash.prepare("trash", conf.Trash, conf.TrashVolume)
	}()
	s.log.Info("sound module started")
}

func KeyBeep() { playStream(&s.keyBeep.Stream) }
func MoneyIn() { playStream(&s.moneyIn.Stream) }
func Trash()   { playStream(&s.trash.Stream) }
func Started() {
	if !stopPreviewPlay() {
		return
	}
	p := playFile(s.sound.Started)
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
	p := playFile(s.sound.Broken)
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
		s.log.Errorf("error open file: %v (%v)", file, err)
		return nil, err
	}
	defer f.Close()
	bs, err := mp3.DecodeWithSampleRate(sampleRate, f)
	if err != nil {
		s.log.Errorf("error decode sound:%v (%v)", file, err)
		return nil, err
	}
	soundStream, err := io.ReadAll(bs)
	return soundStream, err
}

func playStream(byteStream *[]byte) *audio.Player {
	if s.sound.Disabled {
		return nil
	}
	p := s.audioContext.NewPlayerFromBytes(*byteStream)
	p.SetVolume(1.5)
	p.Play()
	return p
}

func playFile(fileName string) *audio.Player {
	if s.sound.Disabled {
		return nil
	}
	bs, _ := loadMp3Steram(fileName)
	return playStream(&bs)
}
