package hot

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
)

type (
	// PidFile represent pid file
	PidFile struct {
		FileName string
		pidFile  *os.File
	}
)

const (
	_OLD_POSTFIX = ".old"
)

// OpenFile is method for open pidfile
// create folder if not exist
func (p *PidFile) OpenFile() error {
	var err error

	folder := filepath.Dir(p.FileName)

	if _, err := os.Stat(folder); os.IsNotExist(err) {
		err := os.MkdirAll(folder, 0750)
		if err != nil {
			return err
		}
	}

	p.pidFile, err = os.OpenFile(p.FileName, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0750)
	if err != nil {
		return err
	}

	return nil
}

// WritePid write pid to opened file
func (p *PidFile) WritePid(pid int) error {
	_, err := p.pidFile.Seek(0, 0)
	if err != nil {
		return err
	}

	n, err := fmt.Fprint(p.pidFile, pid)
	if err != nil {
		return err
	}

	err = p.pidFile.Truncate(int64(n))
	if err != nil {
		return err
	}

	return err
}

// PidFileExist is method for check exist bool file
func PidFileExist(FileName string) bool {
	_, err := os.Stat(FileName)
	return err == nil
}

// PidFileOldExist is method for check exist bool file
func PidFileOldExist(FileName string) bool {
	return PidFileExist(FileName + _OLD_POSTFIX)
}

// PidMoveToOld move pid file with "old" postfix
func PidFileMoveToOld(FileName string) error {
	return os.Rename(FileName, FileName+_OLD_POSTFIX)
}

// PidFileOldRemove is remove old pid file
func PidFileOldRemove(FileName string) error {
	return PidFileRemove(FileName + _OLD_POSTFIX)
}

// PidFileRemove is remove pid file
func PidFileRemove(FileName string) error {
	return os.Remove(FileName)
}

// DefaultPidFileName return default pid file file name
func DefaultPidFileName() string {
	dp, _ := filepath.Abs(os.Args[0])
	return dp + ".pid"
}

// GetPidFromFile read pid file and return process id as int
func GetPidFromFile(filePath string) (int, error) {
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		return 0, err
	}

	pidId, err := strconv.Atoi(string(data))
	if err != nil {
		return 0, err
	}

	return pidId, nil
}
