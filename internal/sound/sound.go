package sound

import (
	"bytes"
	"io"
	"os"
	"time"

	"github.com/AlexTransit/vender/log2"
	"github.com/hajimehoshi/ebiten/v2/audio"
)

const sampleRate = 48000

type audioStream interface {
	io.ReadSeeker
	Length() int64
}

type Sound struct {
	log           *log2.Log
	audioContext  *audio.Context
	audioPlayer   *audio.Player
	enabled       bool
	keyBeepStream []byte
}

type Config struct { //nolint:maligned
	Disabled bool   `hcl:"sound_disabled"`
	KeyBeep  string `hcl:"sound_key"`
	Starting string `hcl:"sound_starting"`
	Started  string `hcl:"sound_started"`
}

var Snd Sound

func Init(conf *Config, log *log2.Log) {
	if conf.Disabled {
		return
	}
	Snd.log = log
	audioContext := audio.NewContext(sampleRate)
	Snd.audioContext = audioContext
	f, err := os.Open(conf.KeyBeep)
	if err != nil {
		return
	}
	var s audioStream
	s, err = mp3.DecodeWithResampling(bytes.NewReader(f))
	if err != nil {
		log.Fatal(err)
		return
	}
	b, _ := io.ReadAll(s)
	Snd.keyBeepStream = b

	sePlayer := Snd.audioContext.NewPlayerFromBytes(Snd.keyBeepStream)
	sePlayer.Play()

	// m, err := NewPlayer(g, audioContext, typeOgg)
	// m, err := NewPlayer(g, audioContext, typeOgg)

	// c, ready, err := oto.NewContext(op)
	// if err != nil {
	// 	Snd.log.Errorf("sound (%v)", err)
	// }
	// <-ready
	// Snd.audioContext = c
	// Snd.enabled = true

	// Snd.loadKeyBeepStream(conf.KeyBeep)
	// KeyBeep()
	time.Sleep(2 * time.Second)
	// KeyBeep()
	time.Sleep(1 * time.Second)
	// time.Sleep(1 * time.Second)
}

func (s *Sound) loadKeyBeepStream(file string) {
	// f, err := os.Open(file)
	// if err != nil {
	// 	return
	// }
	//	defer f.Close()
	// s.keyBeepStream, _ = mp3.NewDecoder(f)
	// s.keyBeeps, _ = mp3.NewDecoder(f)
	// s.keyBeep = s.sndCtx.NewPlayer(s.keyBeeps)
}

func KeyBeep() {
	// Snd.sndCtx.NewPlayerFromBytes()
	// go func() {
	// 	p := Snd.sndCtx.NewPlayer(Snd.keyBeepStream)
	// 	p.Play()
	// 	// time.Sleep(2 * time.Second)
	// 	fmt.Printf("\033[41m %v \033[0m\n", p)
	// }()
}

// // f, err := os.Open(conf.Starting)
// // fmt.Printf("\033[41m %v \033[0m\n", err)
// // /*/
// // f, _ := os.Open("./bb.mp3")
// // //*/
// // defer f.Close()
// // Snd.KeyBeep, _ = mp3.NewDecoder(f)

// // p := c.NewPlayer(Snd.KeyBeep)
// // defer p.Close()
// // p.SetVolume(1.5)
// // p.Play()
// // time.Sleep(10 * time.Second)
// // fmt.Printf("\033[41m asd \033[0m\n")

// // func Init() {
// // 	f, _ := os.Open("/home/vmc/aa.mp3")
// // 	/*/
// // 	f, _ := os.Open("./bb.mp3")
// // 	//*/

// // 	defer f.Close()

// // 	d, _ := mp3.NewDecoder(f)

// // 	op := &oto.NewContextOptions{}
// // 	op.SampleRate = 44100
// // 	op.ChannelCount = 2
// // 	op.Format = oto.FormatSignedInt16LE

// // 	// c, ready, err := oto.NewContext(d.SampleRate(), 2, 2)
// // 	c, ready, err := oto.NewContext(op)
// // 	if err != nil {
// // 	}
// // 	<-ready

// // 	p := c.NewPlayer(d)
// // 	defer p.Close()
// // 	p.SetVolume(1.5)
// // 	p.Play()
// // 	time.Sleep(5 * time.Second)
// // }
