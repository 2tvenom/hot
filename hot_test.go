package hot

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"sync"
	"syscall"
	"testing"
	"time"
)

var (
	testFlag    bool
	pidFileName = os.TempDir() + "/hot/hot.pid"

	hotDaemon     *Hot
	hotDaemon2    *Hot
	hotStandAlone *Hot

	httpOutput = _DATA
)

const (
	_DATA    = "Foo bar"
	_DATA_V2 = "BazzBazz"
	_PORT    = "8080"
)

type (
	hotInstance struct {
		stop chan chan bool
	}
)

func init() {
	flag.BoolVar(&testFlag, "hot", false, "flag for run function in background and skip with testing")
	flag.StringVar(&httpOutput, "httpout", _DATA, "http mock result")
	flag.Parse()

	hot := &hotInstance{
		stop: make(chan chan bool),
	}

	hotDaemon = &Hot{
		Instance: hot,
		Daemon: &Daemon{
			DefaultStdOut:   os.Stderr,
			WorkDir:         "./",
			ProcessFileName: os.Args[0],
			Arguments:       []string{"-test.run=TestDaemonService", "-hot"},
		},
		Pid: &PidFile{
			FileName: pidFileName,
		},
		Signal: NewSignal(syscall.SIGUSR1),
	}

	hotDaemon2 = &Hot{
		Instance: hot,
		Daemon: &Daemon{
			DefaultStdOut:   os.Stderr,
			DefaultStdErr:   os.Stderr,
			WorkDir:         "./",
			ProcessFileName: os.Args[0],
			Arguments:       []string{"-test.run=TestDaemonService", "-hot", "-httpout=" + _DATA_V2},
		},
		Pid: &PidFile{
			FileName: pidFileName,
		},
		Signal: NewSignal(syscall.SIGUSR1),
	}

	hotStandAlone = &Hot{
		Instance: &hotInstance{
			stop: make(chan chan bool),
		},
		Pid: &PidFile{
			FileName: pidFileName + "_standalone",
		},
		Signal: NewSignal(syscall.SIGUSR1),
	}
}

func (h *hotInstance) Run() error {
	mustOutput := httpOutput
	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, mustOutput)
	})

	ln, err := net.Listen("tcp", ":"+_PORT)
	if err != nil {
		return err
	}

	errChan := make(chan error, 2)

	srv := &http.Server{
		Handler: mux,
	}

	var wg sync.WaitGroup
	go func() {
		wg.Add(1)
		defer wg.Done()
		err = srv.Serve(ln)
		if err != nil {
			errChan <- err
		}
	}()

	select {
	case err := <-errChan:
		return err
	case stopCallback := <-h.stop:
		ln.Close()
		wg.Wait()
		stopCallback <- true
		return nil
	}
}

func (h *hotInstance) Stop() error {
	stopCallback := make(chan bool)
	h.stop <- stopCallback
	<-stopCallback
	return nil
}

func TestDaemonService(t *testing.T) {
	if !testFlag {
		return
	}

	fmt.Printf("%d started\n", os.Getpid())
	//	 Run demonization
	err := hotDaemon.Run()
	if err != nil {
		fmt.Printf("%d err: %s\n", os.Getpid(), err)
	}
	fmt.Printf("%d done\n", os.Getpid())
}

func TestStandaloneService(t *testing.T) {
	if !testFlag {
		return
	}

	fmt.Printf("%d started standalone\n", os.Getpid())
	//	 Run demonization
	err := hotStandAlone.Run()
	if err != nil {
		fmt.Printf("%d err: %s\n", os.Getpid(), err)
	}
	fmt.Printf("%d done\n", os.Getpid())
}

func TestRunStandAloneProcessAndStop(t *testing.T) {
	process1 := &Daemon{
		DefaultStdOut:   os.Stderr,
		WorkDir:         "./",
		ProcessFileName: os.Args[0],
		Arguments:       []string{"-test.run=TestStandaloneService", "-hot"},
	}
	_, err := process1.Demonization()

	if err != nil {
		t.Fatalf("Error run process1 : %s", err)
	}

	time.Sleep(time.Second)
	err = checkProcess(_DATA)
	if err != nil {
		t.Fatalf("Error check hot instance : %s", err)
	}

	err = process1.Stop(syscall.SIGUSR1)
	if err != nil {
		t.Fatalf("Send stop signl : %s", err)
	}

	err = checkProcess(_DATA)
	if err == nil {
		t.Fatalf("Hot instance still alive")
	}
}

func TestRunAndHotReplaceStandAloneProcess(t *testing.T) {
	process1 := &Daemon{
		DefaultStdOut:   os.Stderr,
		WorkDir:         "./",
		ProcessFileName: os.Args[0],
		Arguments:       []string{"-test.run=TestStandaloneService", "-hot"},
	}
	_, err := process1.Demonization()

	if err != nil {
		t.Fatalf("Error start process1: %s", err)
	}
	time.Sleep(time.Second)
	err = checkProcess(_DATA)
	if err != nil {
		t.Fatalf("Error check hot instance : %s", err)
	}

	process2 := &Daemon{
		DefaultStdOut:   os.Stderr,
		WorkDir:         "./",
		ProcessFileName: os.Args[0],
		Arguments:       []string{"-test.run=TestStandaloneService", "-hot", "-httpout=" + _DATA_V2},
	}
	_, err = process2.Demonization()
	if err != nil {
		t.Fatalf("Error start process2: %s", err)
	}

	time.Sleep(time.Second)
	err = checkProcess(_DATA_V2)
	if err != nil {
		t.Fatalf("Error check hot instance : %s", err)
	}

	process2.Stop(syscall.SIGUSR1)
}

func TestRunHotServiceAsDaemonAndWait(t *testing.T) {
	errChan := make(chan error, 2)
	// Run demonization
	go func() {
		err := hotDaemon.RunAndWait()
		errChan <- err
	}()

	select {
	case err := <-errChan:
		t.Errorf("Error create daemon : %s", err)
	case <-time.After(time.Second):
	}
	err := checkProcess(_DATA)

	if err != nil {
		t.Errorf("Error check hot instance daemon : %s", err)
	}

	err = hotDaemon.Daemon.GetProcess().Kill()
	if err != nil {
		t.Errorf("Cant kill process with pid %d", err)
	}

	select {
	case err := <-errChan:
		if err != nil {
			t.Fatalf("Incorrect kill process")
		}
	case <-time.After(time.Second * 3):
		err = syscall.Kill(hotDaemon.Daemon.GetProcess().Pid, syscall.Signal(0))
		if err == nil {
			t.Errorf("Process still alive")
		}
	}
}

func TestRunHotServiceAsDaemonWaitAndStop(t *testing.T) {
	errChan := make(chan error, 2)
	// Run demonization
	go func() {
		err := hotDaemon.RunAndWait()
		errChan <- err
	}()

	// wait of get error
	select {
	case err := <-errChan:
		t.Fatalf("Error create daemon : %s", err)
	case <-time.After(time.Second):
	}

	// check process
	err := checkProcess(_DATA)
	if err != nil {
		t.Errorf("Error check hot instance daemon : %s", err)
	}

	err = hotDaemon.Stop()
	if err != nil {
		t.Errorf("Error stop daemon : %s", err)
	}
	return
	select {
	case err := <-errChan:
		if err != nil {
			t.Fatalf("Incorrect kill process")
		}
	case <-time.After(time.Second * 3):
		err = syscall.Kill(hotDaemon.Daemon.GetProcess().Pid, syscall.Signal(0))
		if err == nil {
			t.Errorf("Process still alive")
		}
	}

	err = checkProcess(_DATA)
	if err == nil {
		t.Errorf("Process still work")
	}
}

func TestRunHotServiceAsDaemonRunAndReload(t *testing.T) {
	_, err := hotDaemon.RunAndRelease()

	if err != nil {
		t.Fatalf("Error run daemon1 : %s", err)
	}
	time.Sleep(time.Second)

	err = checkProcess(_DATA)
	if err != nil {
		t.Fatalf("Error check hot instance daemon : %s", err)
	}

	_, err = hotDaemon2.RunAndRelease()
	if err != nil {
		t.Fatalf("Error run daemon2 : %s", err)
	}
	time.Sleep(time.Second)

	err = checkProcess(_DATA_V2)
	if err != nil {
		t.Errorf("Error check hot instance daemon : %s", err)
	}
	hotDaemon2.Stop()
}

func checkProcess(checkData string) error {
	http.DefaultTransport.(*http.Transport).CloseIdleConnections()

	r, err := http.Get("http://localhost:" + _PORT + "/")

	if err != nil {
		return err
	}

	defer r.Body.Close()

	data, err := ioutil.ReadAll(r.Body)

	if err != nil {
		return err
	}

	if string(data) != checkData {
		return errors.New("Incorrect return data")
	}
	return nil
}
