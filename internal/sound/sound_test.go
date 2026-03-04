package sound

import (
	"context"
	"reflect"
	"testing"

	config_global "github.com/AlexTransit/vender/internal/config"
	"github.com/AlexTransit/vender/internal/engine"
	sound_config "github.com/AlexTransit/vender/internal/sound/config"
	"github.com/AlexTransit/vender/internal/state"
	"github.com/AlexTransit/vender/log2"
	"github.com/hajimehoshi/ebiten/v2/audio"
	"github.com/temoto/alive/v2"
)

type soundStateSnapshot struct {
	config        *sound_config.Config
	alive         *alive.Alive
	log           *log2.Log
	audioContext  *audio.Context
	audioPlayer   *audio.Player
	keyBeep       soundStream
	moneyIn       soundStream
	currentVolume float64
}

func snapshotSoundState() soundStateSnapshot {
	return soundStateSnapshot{
		config:        s.config,
		alive:         s.alive,
		log:           s.log,
		audioContext:  s.audioContext,
		audioPlayer:   s.audioPlayer,
		keyBeep:       s.keyBeep,
		moneyIn:       s.moneyIn,
		currentVolume: s.currentVolume,
	}
}

func restoreSoundState(st soundStateSnapshot) {
	s.config = st.config
	s.alive = st.alive
	s.log = st.log
	s.audioContext = st.audioContext
	s.audioPlayer = st.audioPlayer
	s.keyBeep = st.keyBeep
	s.moneyIn = st.moneyIn
	s.currentVolume = st.currentVolume
}

func TestSetVolume(t *testing.T) {
	st := snapshotSoundState()
	t.Cleanup(func() { restoreSoundState(st) })

	SetVolume(15)

	if got := getCurrentVolume(); got != 1.5 {
		t.Fatalf("getCurrentVolume()=%v want=1.5", got)
	}
}

func TestSetDefaultVolume(t *testing.T) {
	st := snapshotSoundState()
	t.Cleanup(func() { restoreSoundState(st) })

	s.config = &sound_config.Config{DefaultVolume: 7}
	SetDefaultVolume()

	if got := getCurrentVolume(); got != 0.7 {
		t.Fatalf("getCurrentVolume()=%v want=0.7", got)
	}
}

func TestSetDefaultVolumeNilConfig(t *testing.T) {
	st := snapshotSoundState()
	t.Cleanup(func() { restoreSoundState(st) })

	s.config = nil
	SetVolume(13)
	SetDefaultVolume()

	if got := getCurrentVolume(); got != 1.3 {
		t.Fatalf("getCurrentVolume()=%v want=1.3", got)
	}
}

func TestTextSpeechEarlyReturn(t *testing.T) {
	st := snapshotSoundState()
	t.Cleanup(func() { restoreSoundState(st) })

	t.Run("nil config", func(t *testing.T) {
		s.config = nil
		TextSpeech("hello")
	})

	t.Run("empty tts exec", func(t *testing.T) {
		s.config = &sound_config.Config{TTSExec: nil}
		TextSpeech("hello")
	})

	t.Run("nil audio context", func(t *testing.T) {
		s.config = &sound_config.Config{TTSExec: []string{"/bin/echo"}}
		s.audioContext = nil
		TextSpeech("hello")
	})
}

func TestPlayStreamEarlyReturn(t *testing.T) {
	st := snapshotSoundState()
	t.Cleanup(func() { restoreSoundState(st) })
	audioCtx := audio.NewContext(sampleRate)

	assertNoPanic := func(t *testing.T, fn func()) {
		t.Helper()
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("unexpected panic: %v", r)
			}
		}()
		fn()
	}

	t.Run("nil config", func(t *testing.T) {
		s.config = nil
		assertNoPanic(t, func() { playStream(&soundStream{Stream: []byte{1}}) })
	})

	t.Run("disabled", func(t *testing.T) {
		s.config = &sound_config.Config{Disabled: true}
		assertNoPanic(t, func() { playStream(&soundStream{Stream: []byte{1}}) })
	})

	t.Run("nil audio context", func(t *testing.T) {
		s.config = &sound_config.Config{}
		s.audioContext = nil
		assertNoPanic(t, func() { playStream(&soundStream{Stream: []byte{1}}) })
	})

	t.Run("nil stream pointer", func(t *testing.T) {
		s.config = &sound_config.Config{}
		s.audioContext = audioCtx
		assertNoPanic(t, func() { playStream(nil) })
	})

	t.Run("empty stream", func(t *testing.T) {
		s.config = &sound_config.Config{}
		s.audioContext = audioCtx
		assertNoPanic(t, func() { playStream(&soundStream{}) })
	})
}

func TestInitDoesNotMutateTTSExec(t *testing.T) {
	st := snapshotSoundState()
	t.Cleanup(func() { restoreSoundState(st) })

	cfg := &config_global.Config{
		Sound: sound_config.Config{
			Disabled: true,
			TTSExec:  []string{"/bin/echo", "--foo"},
		},
	}
	lg := log2.NewTest(t, log2.LOG_DEBUG)
	g := &state.Global{
		Alive:  alive.NewAlive(),
		Config: cfg,
		Engine: engine.NewEngine(lg),
		Log:    lg,
	}
	ctx := context.WithValue(context.Background(), state.ContextKey, g)

	want := append([]string(nil), cfg.Sound.TTSExec...)

	Init(ctx, false)
	if !reflect.DeepEqual(cfg.Sound.TTSExec, want) {
		t.Fatalf("TTSExec mutated after first Init got=%v want=%v", cfg.Sound.TTSExec, want)
	}

	Init(ctx, false)
	if !reflect.DeepEqual(cfg.Sound.TTSExec, want) {
		t.Fatalf("TTSExec mutated after second Init got=%v want=%v", cfg.Sound.TTSExec, want)
	}
}
