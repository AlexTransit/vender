package cli

import (
	"bytes"
	"io"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"github.com/c-bata/go-prompt"
	"github.com/mattn/go-isatty"
)

func MainLoop(tag string, execP func(line string), complete func(d prompt.Document) []prompt.Suggest) {
	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT)
	go func() {
		for range signalCh {
			// TODO engine.Interrupt()
			// if s == syscall.SIGINT { }
			os.Exit(0)
		}
	}()

	if isatty.IsTerminal(os.Stdin.Fd()) {
		// TODO OptionHistory
		prompt.New(execP, complete).Run()
		// AlexM в какой то момент перестало работать эхо в stty после выхода
		// пришлось сделать эту затычку
		rawModeOff := exec.Command("/bin/stty", "-raw", "echo")
		rawModeOff.Stdin = os.Stdin
		_ = rawModeOff.Run()
		rawModeOff.Wait()
	} else {
		stdinAll, err := io.ReadAll(os.Stdin)
		if err != nil {
			log.Fatal(err)
		}
		linesb := bytes.Split(stdinAll, []byte{'\n'})
		for _, lineb := range linesb {
			line := string(bytes.TrimSpace(lineb))
			execP(line)
		}
	}
}
