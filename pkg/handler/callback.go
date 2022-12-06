package handler

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"strings"
)

func PreFinishResponseCallback(hook HookEvent) (err error) {
	fileName := hook.Upload.Storage["Path"]
	if fileName == "" {
		err = errors.New("can't find uploaded file")
		return
	}
	fileInfo, err := os.Stat(fileName)
	if err != nil {
		return
	}

	checksum := hook.Upload.MetaData["checksum"]
	if checksum == "" {
		err = errors.New("can't find checksum from MetaData")
		return
	}

	h := sha256.New()
	fd, err := os.Open(fileName)
	if err != nil {
		return
	}
	defer func() { _ = fd.Close() }()

	const bufLen = 1024 * 1024 * 1
	buf := make([]byte, bufLen)
	var read int
	var totalRead int64
	for {
		read, err = fd.Read(buf)
		if err != nil {
			return
		}

		h.Write([]byte(buf[:read]))
		totalRead += int64(read)
		if totalRead == fileInfo.Size() {
			break
		}
	}

	digest := h.Sum(nil)
	var strBuf []string
	for _, v := range digest {
		strBuf = append(strBuf, fmt.Sprintf("%02x", v))
	}
	beChecksum := strings.Join(strBuf, "")
	if beChecksum != checksum {
		err = errors.New("checksum mismatch")
	}

	return
}

func PreUploadCreateCallback(hook HookEvent) (err error) {

	return
}
