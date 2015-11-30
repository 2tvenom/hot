package hot

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultPidFileName(t *testing.T) {
	pn := DefaultPidFileName()

	dp, _ := filepath.Abs(os.Args[0])
	dp = dp + ".pid"

	if dp != pn {
		t.Fatalf("Incorrect default pid file name expected %s got %s", dp, pn)
	}
}

func TestOpenAndWritePidFile(t *testing.T) {
	p := &PidFile{}
	p.FileName = DefaultPidFileName()

	err := p.OpenFile()
	if err != nil {
		t.Fatalf("Got error at open file %s", err)
	}

	pid := 100

	err = p.WritePid(pid)
	if err != nil {
		t.Fatalf("Got error at write pid %s", err)
	}

	resultPid, err := GetPidFromFile(p.FileName)
	if err != nil {
		t.Fatalf("Error loading pid file for checking data", err)
	}

	if resultPid != pid {
		t.Fatalf("Incorrect pid file in file. Expected %d got %d", pid, resultPid)
	}

	pid = 2

	err = p.WritePid(pid)
	if err != nil {
		t.Fatalf("Got error at write pid %s", err)
	}
	resultPid, err = GetPidFromFile(p.FileName)
	if err != nil {
		t.Fatalf("Error loading pid file for checking data", err)
	}

	if resultPid != pid {
		t.Fatalf("Incorrect pid file in file. Expected %d got %d", pid, resultPid)
	}
}

func deleteFolderIfExist(folder string) {
	if _, err := os.Stat(folder); os.IsExist(err) {
		os.RemoveAll(folder)
	}
}
func TestOpenAndCreateFolder(t *testing.T) {
	folder_name := "./"

	deleteFolderIfExist(folder_name)
	defer deleteFolderIfExist(folder_name)

	p := &PidFile{}
	p.FileName = folder_name + "/pid"

	err := p.OpenFile()
	if err != nil {
		t.Errorf("Got error at open file %s", err)
	}

	if _, err := os.Stat(folder_name); os.IsNotExist(err) {
		t.Errorf("Folder " + folder_name + " not found")
	}
}

func TestOpenAndWritePidFileInTmp(t *testing.T) {
	p := &PidFile{}
	p.FileName = os.TempDir() + "pid_test.pid"

	err := p.OpenFile()
	if err != nil {
		t.Fatalf("Got error at open file %s", err)
	}

	pid := 100

	err = p.WritePid(pid)
	if err != nil {
		t.Fatalf("Got error at write pid %s", err)
	}

	if !PidFileExist(p.FileName) {
		t.Fatalf("Pid not exist in temp folder at path %s", p.FileName)
	}
}
