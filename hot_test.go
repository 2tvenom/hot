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

	hotDaemon = &Hot{
		Instance: &hotInstance{
			stop: make(chan chan bool),
		},
		Daemon: &Daemon{
			DefaultStdOut:   os.Stdout,
			WorkDir:         "./",
			ProcessFileName: os.Args[0],
			Arguments:       []string{"-test.run=\"^TestDaemon$\"", "-hot"},
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
			FileName: pidFileName,
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

	//	sListener := &stopListener{
	//		TCPListener: ln.(*net.TCPListener),
	//		stop: make(chan bool),
	//	}

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

func TestDaemon(t *testing.T) {
	if !testFlag {
		return
	}

	fmt.Println("Started!")
	// Run demonization
	_, err := hotDaemon.Run()
	if err != nil {
		fmt.Printf("Err: %s\n", err)
	}
	fmt.Println("Done!")
}

func TestRunStandAloneProcessAndStop(t *testing.T) {
	errChan := make(chan error)

	go func() {
		_, err := hotStandAlone.Run()
		errChan <- err
	}()

	select {
	case err := <-errChan:
		t.Fatalf("Error create daemon : %s", err)
	case <-time.After(time.Second):
	}

	err := checkProcess(_DATA)
	if err != nil {
		t.Fatalf("Error check hot instance : %s", err)
	}

	hotStandAlone.Stop()

	err = checkProcess(_DATA)
	if err == nil {
		t.Fatalf("Hot instance still alive")
	}
}

func TestRunAndHotReplaceStandAloneProcess(t *testing.T) {
	errChan := make(chan error)

	go func() {
		_, err := hotStandAlone.Run()
		errChan <- err
	}()

	select {
	case err := <-errChan:
		t.Fatalf("Error create daemon : %s", err)
	case <-time.After(time.Second):
	}

	err := checkProcess(_DATA)
	if err != nil {
		t.Fatalf("Error check hot instance : %s", err)
	}

	httpOutput = _DATA_V2
	go func() {
		_, err = hotStandAlone.Run()
	}()

	time.Sleep(time.Second)

	if err != nil {
		t.Fatalf("Error run second hot instance : %s\n", err)
	}
	err = checkProcess(_DATA_V2)
	if err != nil {
		t.Fatalf("Error check hot instance : %s", err)
	}
	hotStandAlone.Stop()
}

func TestRunHotServiceAsDaemonAndWait(t *testing.T) {
	errChan := make(chan error)
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

	println("Process pid %d\n", hotDaemon.Daemon.GetProcess().Pid)

	time.Sleep(time.Second * 15)

	err = hotDaemon.Daemon.GetProcess().Kill()

	if err != nil {
		t.Errorf("Cant kill process with pid %d", err)
	}

	t.Logf("Process pid %d\n", hotDaemon.Daemon.GetProcess().Pid)

	err = hotDaemon.Daemon.GetProcess().Signal(syscall.Signal(0))
	if err == nil {
		t.Errorf("Process still alive")
	}
}

func TestRunHotServiceAsDaemonWaitAndStop(t *testing.T) {
	errChan := make(chan error)
	// Run demonization
	go func() {
		err := hotDaemon.RunAndWait()
		errChan <- err
	}()
	//	defer hotDaemon.Daemon.GetProcess().Kill()

	select {
	case err := <-errChan:
		t.Errorf("Error create daemon : %s", err)
	case <-time.After(time.Second):
	}

	err := checkProcess(_DATA)
	if err != nil {
		t.Errorf("Error check hot instance daemon : %s", err)
	}

	err = hotDaemon.Stop()
	if err != nil {
		t.Errorf("Error stop daemon : %s", err)
	}

	err = hotDaemon.Daemon.GetProcess().Signal(syscall.Signal(0))
	if err == nil {
		t.Errorf("Process still alive")
	}

	err = checkProcess(_DATA)
	if err == nil {
		t.Errorf("Process still work")
	}
}

func TestRunHotServiceAsDaemonRunAndReload(t *testing.T) {
	process1, err := hotDaemon.Run()
	defer process1.Kill()
	fmt.Printf("Daemon1 pid %d\n", process1.Pid)
	time.Sleep(time.Second)

	err = checkProcess(_DATA)
	if err != nil {
		t.Fatalf("Error check hot instance daemon : %s", err)
	}

	hotDaemon.Daemon.Arguments = append(hotDaemon.Daemon.Arguments, "-httpout="+_DATA_V2)

	process2, err := hotDaemon.Run()
	defer process2.Kill()


	err = process1.Signal(syscall.Signal(0))
	if err == nil {
		t.Errorf("Process still alive")
	}

	time.Sleep(time.Second * 15)
	hotDaemon.Daemon.Arguments = hotDaemon.Daemon.Arguments[0:2]

	time.Sleep(time.Second)

	err = checkProcess(_DATA_V2)
	if err != nil {
		t.Errorf("Error check hot instance daemon : %s", err)
	}
	hotDaemon.Stop()
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
