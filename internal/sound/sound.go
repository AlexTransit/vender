package sound

// used low level sound via github.com/hajimehoshi/ebiten/
// need install a few packages
//  apt install pkg-config libasound2-dev

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/AlexTransit/vender/internal/engine"
	sound_config "github.com/AlexTransit/vender/internal/sound/config"
	"github.com/AlexTransit/vender/internal/state"
	"github.com/AlexTransit/vender/log2"
	"github.com/hajimehoshi/ebiten/v2/audio"
	"github.com/hajimehoshi/ebiten/v2/audio/mp3"
	"github.com/temoto/alive/v2"
)

const sampleRate = 11025

type Sound struct {
	keyBeep       soundStream
	moneyIn       soundStream
	config        *sound_config.Config
	alive         *alive.Alive
	log           *log2.Log
	audioContext  *audio.Context
	audioPlayer   *audio.Player
	currentVolume float64
}
type soundStream struct {
	Stream []byte
}

var (
	s           Sound
	stdout      io.Writer
	sysProcAttr *syscall.SysProcAttr
)

func Init(ctx context.Context, startingVMC bool) {
	g := state.GetGlobal(ctx)
	s.config = &g.Config.Sound
	s.config.TTS_exec = append(s.config.TTS_exec, "--output_raw")
	s.alive = g.Alive
	g.Engine.RegisterNewFuncAgr("sound(?)", func(ctx context.Context, arg engine.Arg) error {
		PlayFileNoWait(arg.(string))
		return nil
	})

	g.Engine.RegisterNewFuncAgr("play(?)", func(ctx context.Context, arg engine.Arg) error {
		PlayFile(arg.(string))
		return nil
	})
	g.Engine.RegisterNewFuncAgr("speech(?)", func(ctx context.Context, arg engine.Arg) error {
		TextSpeech(arg.(string))
		return nil
	})

	g.Engine.RegisterNewFuncAgr("sound.volume(?)", func(ctx context.Context, arg engine.Arg) error { SetVolume(arg.(int16)); return nil })

	g.Engine.RegisterNewFunc("sound.volume.default", func(ctx context.Context) error { SetDefaultVolume(); return nil })

	if s.config.Disabled {
		return
	}
	s.log = g.Log
	SetDefaultVolume()
	audioContext := audio.NewContext(sampleRate)
	s.audioContext = audioContext
	// g.Engine.Exec(ctx, g.Engine.Resolve("sound(cat.mp3)"))
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

func TextSpeech(tts string) {
	stdout = bytes.NewBuffer(nil)
	stdin := strings.NewReader(tts)
	stderr := bytes.NewBuffer(nil)
	cmd := exec.Command(s.config.TTS_exec[0], s.config.TTS_exec[1:]...)
	// cmd.Dir = "/home/vmc/00"
	cmd.Dir = filepath.Dir(s.config.TTS_exec[0])
	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	cmd.SysProcAttr = sysProcAttr
	if err := cmd.Run(); err != nil {
		s.log.Errf("speech(%s) error:%v", tts, err)
		return
	}
	p := s.audioContext.NewPlayerFromBytes(stdout.(*bytes.Buffer).Bytes())
	p.SetVolume(s.currentVolume)
	p.Play()
}

func PlayFile(file string) error {
	s.alive.Add(1)
	PlayFileNoWait(file)
	waitingEndPlay()
	s.alive.Done()
	return nil
}

func PlayFileNoWait(file string) error {
	if err := playMP3controlled(file); err != nil {
		s.log.Errorf(" play file error(%v)", err)
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
	// str, err := mp3.DecodeWithoutResampling(f)
	str, err := mp3.DecodeWithSampleRate(sampleRate, f)
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
