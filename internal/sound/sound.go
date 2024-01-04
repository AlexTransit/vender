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

const sampleRate = 24000

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
	Folder         string
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

func Init(conf *Config, log *log2.Log, startingVMC bool) {
	s.sound = conf
	if conf.Disabled {
		return
	}
	s.log = log
	audioContext := audio.NewContext(sampleRate)
	s.audioContext = audioContext
	if startingVMC {
		PlayStarting()
		go func() {
			s.keyBeep.prepare("keyBeep", s.sound.KeyBeep, s.sound.KeyBeepVolume)
			s.moneyIn.prepare("money in", s.sound.MoneyIn, s.sound.MoneyInVolume)
			s.trash.prepare("trash", s.sound.Trash, s.sound.TrashVolume)
		}()
	}
}

func PlayStarting() { playMP3controlled(s.sound.Starting, s.sound.StartingVolume) }
func PlayStarted()  { playMP3controlled(s.sound.Started, s.sound.StartedVolume) }
func PlayKeyBeep()  { playStream(&s.keyBeep) }
func PlayMoneyIn()  { playStream(&s.moneyIn) }
func PlayTrash()    { playStream(&s.trash) }

// play file and wait finishing
func Broken() { playMP3controlled(s.sound.Broken, s.sound.BrokenVolume); waitingEndPlay() }

func Stop() {
	if s.audioPlayer == nil {
		return
	}
	if s.audioPlayer.IsPlaying() {
		s.audioPlayer.Pause()
		s.audioPlayer.Close()
	}
}

func playMP3controlled(file string, volume int) {
	if s.sound.Disabled {
		return
	}
	Stop()
	f, err := os.Open(s.sound.Folder + file)
	if err != nil {
		s.log.Errorf("open starting (%v)", err)
	}
	str, _ := mp3.DecodeWithoutResampling(f)
	s.audioPlayer, err = s.audioContext.NewPlayer(str)
	s.audioPlayer.SetVolume(float64(helpers.ConfigDefaultInt(s.sound.StartingVolume, 10)) / 10)
	if err != nil {
		s.log.Errorf("new player (%v)", err)
	}
	go func() {
		s.audioPlayer.Play()
		waitingEndPlay()
		f.Close()
	}()
	s.log.Infof("play %s", file)
}

func waitingEndPlay() {
	for {
		if !s.audioPlayer.IsPlaying() {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func (ss *soundStream) prepare(name string, file string, volume int) {
	var err error
	ss.Stream, err = loadSteram(file)
	if err != nil {
		s.log.Errorf("load sound %s (%+v)", name, err)
	}
	ss.volume = float64(helpers.ConfigDefaultInt(volume, 10)) / 10
}

func loadSteram(file string) ([]byte, error) {
	f, err := os.Open(s.sound.Folder + file)
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
	if s.sound.Disabled {
		return nil
	}
	p := s.audioContext.NewPlayerFromBytes(ss.Stream)
	p.SetVolume(ss.volume)
	p.Play()
	return p
}
