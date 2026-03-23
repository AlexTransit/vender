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
	"sync"
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
	config        *sound_config.Config
	alive         *alive.Alive
	log           *log2.Log
	playerMu      sync.Mutex
	volumeMu      sync.RWMutex
	audioContext  *audio.Context
	audioPlayer   *audio.Player
	keyBeep       soundStream
	moneyIn       soundStream
	currentVolume float64
}
type soundStream struct {
	Stream []byte
}

var (
	s           Sound
	sysProcAttr *syscall.SysProcAttr
)

func Init(ctx context.Context, startingVMC bool) {
	g := state.GetGlobal(ctx)
	s.config = &g.Config.Sound
	s.alive = g.Alive
	g.Engine.RegisterNewFuncAgr("sound(?)", func(ctx context.Context, arg engine.Arg) error {
		_ = PlayFileNoWait(arg.(string))
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
	s.volumeMu.Lock()
	s.currentVolume = float64(v) / 100
	s.volumeMu.Unlock()
}

// set volume from config
func SetDefaultVolume() {
	if s.config == nil {
		return
	}
	s.volumeMu.Lock()
	s.currentVolume = float64(s.config.DefaultVolume) / 100
	s.volumeMu.Unlock()
}

func getCurrentVolume() float64 {
	s.volumeMu.RLock()
	v := s.currentVolume
	s.volumeMu.RUnlock()
	return v
}

func TextSpeech(tts string) {
	if s.config == nil || s.audioContext == nil || len(s.config.TTSExec) == 0 || s.config.TTSExec[0] == "" {
		return
	}
	stdout := bytes.NewBuffer(nil)
	stdin := strings.NewReader(tts)
	stderr := bytes.NewBuffer(nil)
	ttsArgs := append([]string{}, s.config.TTSExec[1:]...)
	hasOutputRaw := false
	for _, arg := range ttsArgs {
		if arg == "--output_raw" {
			hasOutputRaw = true
			break
		}
	}
	if !hasOutputRaw {
		ttsArgs = append(ttsArgs, "--output_raw")
	}
	cmd := exec.Command(s.config.TTSExec[0], ttsArgs...)
	// cmd.Dir = "/home/vmc/00"
	cmd.Dir = filepath.Dir(s.config.TTSExec[0])
	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	cmd.SysProcAttr = sysProcAttr
	if err := cmd.Run(); err != nil {
		s.log.Errf("speech(%s) error:%v", tts, err)
		return
	}
	p := s.audioContext.NewPlayerFromBytes(stdout.Bytes())
	p.SetVolume(getCurrentVolume())
	p.Play()
}

func PlayFile(file string) {
	s.alive.Add(1)
	if err := PlayFileNoWait(file); err == nil {
		s.playerMu.Lock()
		player := s.audioPlayer
		s.playerMu.Unlock()
		waitingEndPlay(player)
	}
	s.alive.Done()
}

func PlayFileNoWait(file string) error {
	if err := playMP3controlled(file); err != nil {
		s.log.Errorf(" play file error(%v)", err)
		return err
	}
	return nil
}

func Stop() {
	s.playerMu.Lock()
	defer s.playerMu.Unlock()
	stopLocked()
}

func stopLocked() {
	if s.audioPlayer == nil {
		return
	}
	if s.audioPlayer.IsPlaying() {
		s.audioPlayer.Pause()
	}
	s.audioPlayer.Close()
	s.audioPlayer = nil
}

// play file. stopping if played other
func playMP3controlled(file string) (err error) {
	if s.config == nil || s.config.Disabled || s.audioContext == nil {
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
		f.Close()
		return
	}
	player, err := s.audioContext.NewPlayer(str)
	if err != nil {
		f.Close()
		return
	}
	player.SetVolume(getCurrentVolume())
	s.playerMu.Lock()
	stopLocked()
	s.audioPlayer = player
	s.playerMu.Unlock()
	go func() {
		player.Play()
		waitingEndPlay(player)
		f.Close()
	}()
	s.log.Infof("play %s", file)
	return nil
}

func waitingEndPlay(player *audio.Player) {
	for {
		if player == nil {
			return
		}
		if !player.IsPlaying() {
			player.Close()
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func (ss *soundStream) prepare(name string, file string) {
	var err error
	ss.Stream, err = loadStream(file)
	if err != nil {
		s.log.Errorf("load sound %s (%+v)", name, err)
	}
}

func loadStream(file string) ([]byte, error) {
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

func playStream(ss *soundStream) {
	if s.config == nil || s.config.Disabled || s.audioContext == nil || ss == nil || len(ss.Stream) == 0 {
		return
	}
	p := s.audioContext.NewPlayerFromBytes(ss.Stream)
	p.SetVolume(getCurrentVolume())
	p.Play()
}
