package hot

import (
	"os"
	"path/filepath"
)

const (
	_DAEMON_MARK = "_DAEMON"
	_MARK_VALUE  = "yes"
)

type (
	Daemon struct {
		DefaultStdIn  *os.File
		DefaultStdOut *os.File
		DefaultStdErr *os.File
		DefaultNull   *os.File

		WorkDir         string
		ProcessFileName string

		Arguments []string

		process *os.Process
	}
)

func NewDaemon() *Daemon {
	nullFile, err := os.Open(os.DevNull)
	if err != nil {
		panic("Error open /dev/null : " + err.Error())
	}

	return &Daemon{
		DefaultStdIn:  os.Stdin,
		DefaultStdOut: os.Stdout,
		DefaultStdErr: os.Stderr,
		DefaultNull:   nullFile,
		WorkDir:       "./",
		Arguments:     []string{},
	}
}

func (d *Daemon) SetCurrentFileAsDaemon() error {
	processFile, err := filepath.Abs(os.Args[0])
	if err != nil {
		return err
	}
	d.ProcessFileName = processFile
	return nil
}

func (d *Daemon) Demonization() (*os.Process, error) {
	proc := &os.ProcAttr{
		Dir:   d.WorkDir,
		Env:   []string{_DAEMON_MARK + "=" + _MARK_VALUE},
		Files: []*os.File{d.DefaultStdIn, d.DefaultStdOut, d.DefaultStdErr, d.DefaultNull},
	}

	process, err := os.StartProcess(d.ProcessFileName, d.Arguments, proc)

	if err != nil {
		return nil, err
	}

	d.process = process
	return process, nil
}

func (d *Daemon) IsDaemon() bool {
	return os.Getenv(_DAEMON_MARK) == _MARK_VALUE
}

func (d *Daemon) GetProcess() *os.Process {
	return d.process
}

func (d *Daemon) Stop(signal os.Signal) error {
	return d.process.Signal(signal)
}