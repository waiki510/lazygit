package oscommands

import (
	"os"
	"os/exec"
	"strings"

	"github.com/jesseduffield/lazygit/pkg/secureexec"
	"github.com/mgutz/str"
)

// A command object is a general way to represent a command to be run on the
// command line. If you want to log the command you'll use .ToString() and
// if you want to run it you'll use .GetCmd()
type ICmdObj interface {
	GetCmd() *exec.Cmd
	ToString() string
	AddEnvVars(...string) ICmdObj
}

type CmdObj struct {
	cmdStr string
	cmd    *exec.Cmd
}

func (self *CmdObj) GetCmd() *exec.Cmd {
	return self.cmd
}

func (self *CmdObj) ToString() string {
	return self.cmdStr
}

func (self *CmdObj) AddEnvVars(vars ...string) ICmdObj {
	self.cmd.Env = append(self.cmd.Env, vars...)

	return self
}

func NewCmdObjFromStr(cmdStr string) ICmdObj {
	args := str.ToArgv(cmdStr)
	cmd := secureexec.Command(args[0], args[1:]...)
	cmd.Env = os.Environ()

	return &CmdObj{
		cmdStr: cmdStr,
		cmd:    cmd,
	}
}

func NewCmdObjFromArgs(args []string) ICmdObj {
	cmd := secureexec.Command(args[0], args[1:]...)

	return &CmdObj{
		cmdStr: strings.Join(args, " "),
		cmd:    cmd,
	}
}

func NewCmdObj(cmd *exec.Cmd) ICmdObj {
	return &CmdObj{
		cmdStr: strings.Join(cmd.Args, " "),
		cmd:    cmd,
	}
}
