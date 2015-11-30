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
	_DATA_V2 = "Bazz Bazz"
	_PORT    = "8080"
)

type (
	hotInstance struct {
		stop chan chan bool
	}

	stopListener struct {
		*net.TCPListener
		stop chan bool
	}
)

func (sl *stopListener) Accept() (net.Conn, error) {
	for {
		//Wait up to one second for a new connection
		sl.SetDeadline(time.Now().Add(time.Second))

		newConn, err := sl.TCPListener.Accept()

		//Check for the channel being closed
		select {
		case <-sl.stop:
			return nil, errors.New("Stopped")
		default:
			//If the channel is still open, continue as normal
		}

		if err != nil {
			netErr, ok := err.(net.Error)

			//If this is a timeout, then continue to wait for
			//new connections
			if ok && netErr.Timeout() && netErr.Temporary() {
				continue
			}
		}

		return newConn, err
	}
}

func init() {
	flag.BoolVar(&testFlag, "hot", false, "flag for run function in background and skip with testing")
	flag.Parse()

	hotDaemon = &Hot{
		Instance: &hotInstance{
			stop: make(chan chan bool),
		},
		Daemon: &Daemon{
			DefaultStdOut:   os.Stdout,
			WorkDir:         "./",
			ProcessFileName: os.Args[0],
			Arguments:       []string{"-test.run=TestDaemon", "-hot"},
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
		t.Fatalf("Error create daemon : %s", err)
	case <-time.After(time.Second):
	}
	err := checkProcess(_DATA)

	if err != nil {
		t.Fatalf("Error check hot instance daemon : %s", err)
	}

	err = hotDaemon.Daemon.GetProcess().Kill()

	if err != nil {
		t.Fatalf("Cant kill process with pid %d", err)
	}
}

func TestRunHotServiceAsDaemonWaitAndStop(t *testing.T) {
	errChan := make(chan error)
	// Run demonization
	go func() {
		err := hotDaemon.RunAndWait()
		errChan <- err
	}()

	select {
	case err := <-errChan:
		t.Fatalf("Error create daemon : %s", err)
	case <-time.After(time.Second):
	}

	err := checkProcess(_DATA)
	if err != nil {
		t.Fatalf("Error check hot instance daemon : %s", err)
	}

	err = hotDaemon.Stop()
	if err != nil {
		t.Fatalf("Error stop daemon : %s", err)
	}

	err = checkProcess(_DATA)
	if err == nil {
		t.Fatalf("Process still alive")
	}
}

func TestHotServiceStop(t *testing.T) {
	return
	process, err := hotDaemon.Run()
	if err != nil {
		t.Fatalf("Error demonize %s", err)
	}
	// Wait starting http server
	time.Sleep(time.Second)

	// Check process correct work
	checkProcess(_DATA)

	err = syscall.Kill(process.Pid, syscall.SIGUSR1)

	if err != nil {
		t.Fatalf("Error send signal %s", err)
	}

	time.Sleep(time.Second)

	err = process.Signal(syscall.Signal(0))

	if err != nil {
		t.Fatalf("Process with pid still alive")
	}
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
