package gui

import (
	"fmt"
	"net"
	"net/http"
	"net/rpc"
	"os"
	"sync"

	"github.com/go-errors/errors"
	"github.com/phayes/freeport"
)

type GuiRpc struct {
	gui          *Gui
	requestMutex sync.Mutex
}

type RpcArgs struct {
	Filename string
}

func (g *GuiRpc) EditFileInSubprocess(args *RpcArgs, reply *int) error {
	g.requestMutex.Lock()
	defer g.requestMutex.Unlock()

	doneChan := g.gui.returnFromSubprocessChan

	editor, err := g.gui.OSCommand.GetEditor()
	if err != nil {
		return errors.New("No editor defined")
	}

	g.gui.PrepareSubProcess(editor, args.Filename)

	<-doneChan

	return nil
}

func (gui *Gui) runServer() error {
	g := &GuiRpc{gui: gui}

	if err := rpc.Register(g); err != nil {
		return err
	}

	rpc.HandleHTTP()
	l, err := net.Listen("tcp", os.Getenv("LAZYGIT_TCP_ADDRESS"))
	if err != nil {
		return err
	}
	go http.Serve(l, nil)

	return nil
}

func SwitchToEditor() error {
	client, err := rpc.DialHTTP("tcp", os.Getenv("LAZYGIT_TCP_ADDRESS"))
	if err != nil {
		return err
	}

	var reply int
	err = client.Call("GuiRpc.EditFileInSubprocess", &RpcArgs{Filename: os.Args[1]}, &reply)
	if err != nil {
		return err
	}

	return nil
}

func SetTCPAddressEnvVar() error {
	port, err := freeport.GetFreePort()
	if err != nil {
		return err
	}
	// this allows us to use a lazyit client as an editor when running a subprocess
	return os.Setenv("LAZYGIT_TCP_ADDRESS", fmt.Sprintf("localhost:%d", port))
}
