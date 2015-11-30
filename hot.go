package hot

import (
	"errors"
	"os"
	"syscall"
	"time"
	"fmt"
)

type (
	// HotInstance process interface
	HotInstance interface {
		Run() error
		Stop() error
	}

	// HotInstancePrepare prepare interface
	HotInstancePrepare interface {
		Prepare() error
	}

	// Hot is general structure
	Hot struct {
		Instance HotInstance

		Daemon *Daemon
		Pid    *PidFile
		Signal *Signal
	}

	HotStandalone interface {
		Run() (*os.Process, error)
		Stop() error
	}

	HotDaemon interface {
		Run() (*os.Process, error)
		RunAndObserve() (*os.Process, error)
		RunAndWait() (*os.Process, error)
		Stop() error
	}
)

func NewHot(i HotInstance) HotStandalone {
	return &Hot{
		Instance: i,
		Pid: &PidFile{
			FileName: DefaultPidFileName(),
		},
		Signal: NewSignal(syscall.SIGUSR1),
	}
}

func NewHotDaemon(i HotInstance) HotStandalone {
	return &Hot{
		Instance: i,
		Pid: &PidFile{
			FileName: DefaultPidFileName(),
		},
		Signal: NewSignal(syscall.SIGUSR1),
		Daemon: NewDaemon(),
	}
}

// Run hot service
func (h *Hot) Run() (*os.Process, error) {
	return h.run()
}

func (h *Hot) run() (*os.Process, error) {
	if h.Instance == nil {
		panic("Set execute hot instance")
	}

	// Start demonization
	if h.Daemon != nil && !h.Daemon.IsDaemon() {
		return h.Daemon.Demonization()
	}

	// Prepare instance
	if i, ok := h.Instance.(HotInstancePrepare); ok {
		err := i.Prepare()
		if err != nil {
			return nil, errors.New("Error prepare instance : " + err.Error())
		}
	}

	//Check old pid file, delete it if exist
	if PidFileOldExist(h.Pid.FileName) {
		PidFileOldRemove(h.Pid.FileName)
	}

	// If current pid file exist
	if PidFileExist(h.Pid.FileName) {
		// Get pid
		pid, err := GetPidFromFile(h.Pid.FileName)

		if err != nil {
			return nil, err
		}

		// Move to old file name
		err = PidFileMoveToOld(h.Pid.FileName)

		if err != nil {
			return nil, err
		}

		// Find process
		process, err := os.FindProcess(pid)
		if err == nil {
			//Check process
			err := process.Signal(syscall.Signal(0))
			//Process alive
			if err == nil {
				fmt.Printf("Found pid %d\n", process.Pid)
				// If found send kill signal
				err := process.Signal(h.Signal.signal)
				if err != nil {
					return nil, err
				}

				time.Sleep(time.Millisecond * 10)
			}
		}

		// Remove old pid file
		PidFileOldRemove(h.Pid.FileName)
	}

	// Try open file
	if err := h.Pid.OpenFile(); err != nil {
		return nil, err
	}

	// Write pid
	err := h.Pid.WritePid(os.Getpid())
	if err != nil {
		return nil, err
	}

	// Run in goroutine signal watch
	go func() {
		h.Signal.WatchHandler(func() {
			fmt.Printf("Catch signal at pid %d\n",os.Getpid())
			h.Stop()
		})
	}()

	// Run instance
	err = h.Instance.Run()

	return nil, err
}

// Stop method send stop signal to instance
func (h *Hot) Stop() error {
	//If demonized
	if h.Daemon != nil && !h.Daemon.IsDaemon() {
		return h.Daemon.Stop(h.Signal.signal)
	}
	return h.Instance.Stop()
}

func (h *Hot) RunAndWait() error {
	process, err := h.run()

	if err != nil {
		return err
	}
	_, err = process.Wait()
	return err
}
