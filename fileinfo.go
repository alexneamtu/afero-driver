package aferodriver

import (
	"fmt"
	"github.com/pkg/errors"
	"github.com/spf13/afero"
	"github.com/stiletto/goftp-server"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type AferoDriver struct {
	aferoFS afero.Fs
	curDir  string
	server.Perm
}

func (driver *AferoDriver) Init(conn *server.Conn) {
	//driver.conn = conn
}

func (driver *AferoDriver) Stat(path string) (server.FileInfo, error) {
	path = filepath.ToSlash(path)
	f, err := driver.aferoFS.Stat(path)
	if err != nil {
		path = strings.TrimPrefix(path, "/")
		f, err = driver.aferoFS.Stat(path)
		if err == nil {
			return &FileInfo{f.Name(), f.Size(), f.IsDir()}, err
		}
		return nil, err
	}
	return &FileInfo{f.Name(), f.Size(), f.IsDir()}, err
}

func (driver *AferoDriver) ChangeDir(path string) error {
	f, err := driver.Stat(path)
	if err != nil {
		return err
	}

	if !f.IsDir() {
		return errors.New("not a dir")
	}
	driver.curDir = path
	return nil
}

func (driver *AferoDriver) ListDir(path string, callback func(server.FileInfo) error) error {
	entries, err := afero.ReadDir(driver.aferoFS.(afero.Fs), path)
	for _, entry := range entries {
		f := &FileInfo{entry.Name(), entry.Size(), entry.IsDir()}
		err = callback(f)
		if err != nil {
			return err
		}
	}
	return nil
}

func (driver *AferoDriver) DeleteDir(path string) error {
	return driver.aferoFS.RemoveAll(path)
}

func (driver *AferoDriver) DeleteFile(path string) error {
	return driver.aferoFS.Remove(path)
}

func (driver *AferoDriver) Rename(fromPath string, toPath string) error {
	fromPath = filepath.ToSlash(fromPath)
	_, err := driver.aferoFS.Stat(toPath)
	if err == nil {
		return errors.New("file already exists")
	}
	err = driver.aferoFS.Rename(fromPath, toPath)
	if err != nil {
		fromPath = strings.TrimPrefix(fromPath, "/")
		err = driver.aferoFS.Rename(fromPath, toPath)
	}
	return err
}

func (driver *AferoDriver) MakeDir(path string) error {
	return driver.aferoFS.Mkdir(path, os.ModePerm)
}

func (driver *AferoDriver) GetFile(path string, offset int64) (int64, io.ReadCloser, error) {
	f, err := driver.aferoFS.Open(path)
	if err != nil {
		return 0, nil, err
	}

	info, err := f.Stat()
	if err != nil {
		return 0, nil, err
	}

	f.Seek(offset, os.SEEK_SET)

	return info.Size(), f, nil
}

func (driver *AferoDriver) PutFile(destPath string, data io.Reader, appendData bool) (int64, error) {
	var isExist bool
	f, err := driver.aferoFS.Stat(destPath)
	if err == nil {
		if f != nil {
			isExist = true
			if f.IsDir() {
				return 0, errors.New("A dir has the same name")
			}
		}
	} else {
		if os.IsNotExist(err) {
			isExist = false
		} else {
			return 0, errors.New(fmt.Sprintln("Put File error:", err))
		}
	}

	if appendData && !isExist {
		appendData = false
	}

	if !appendData {
		if isExist {
			err = driver.aferoFS.Remove(destPath)
			if err != nil {
				return 0, err
			}
		}
		f, err := driver.aferoFS.Create(destPath)
		if err != nil {
			return 0, err
		}
		defer f.Close()
		bytes, err := io.Copy(f, data)
		if err != nil {
			return 0, err
		}
		return bytes, nil
	}

	of, err := driver.aferoFS.OpenFile(destPath, os.O_APPEND|os.O_RDWR, 0660)
	if err != nil {
		return 0, err
	}
	defer of.Close()

	_, err = of.Seek(0, os.SEEK_END)
	if err != nil {
		return 0, err
	}

	bytes, err := io.Copy(of, data)
	if err != nil {
		return 0, err
	}

	return bytes, nil
}

type AferoDriverFactory struct {
	AferoFs afero.Fs
}

func (factory *AferoDriverFactory) NewDriver() (server.Driver, error) {
	return &AferoDriver{factory.AferoFs, "/", server.NewSimplePerm("test", "test")}, nil
}
