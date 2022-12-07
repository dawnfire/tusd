// Package filestore provide a storage backend based on the local file system.
//
// FileStore is a storage backend used as a handler.DataStore in handler.NewHandler.
// It stores the uploads in a directory specified in two different files: The
// `[id].info` files are used to store the fileinfo in JSON format. The
// `[id]` files without an extension contain the raw binary data uploaded.
// No cleanup is performed so you may want to run a cronjob to ensure your disk
// is not filled up with old and finished uploads.
package filestore

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/dawnfire/tusd/internal/uid"
	"github.com/dawnfire/tusd/pkg/handler"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var defaultFilePerm = os.FileMode(0664)

// FileStore See the handler.DataStore interface for documentation about the different
// methods.
type FileStore struct {
	// Relative or absolute path to store files in. FileStore does not check
	// whether the path exists, use os.MkdirAll in this case on your own.
	Path string
}

// New creates a new file based storage backend. The directory specified will
// be used as the only storage entry. This method does not check
// whether the path exists, use os.MkdirAll to ensure.
// In addition, a locking mechanism is provided.
func New(path string) FileStore {
	return FileStore{path}
}

// UseIn sets this store as the core data store in the past composer and adds
// all possible extension to it.
func (store FileStore) UseIn(composer *handler.StoreComposer) {
	composer.UseCore(store)
	composer.UseTerminater(store)
	composer.UseConcater(store)
	composer.UseLengthDeferrer(store)
	composer.UseChecksum(store)
}

func (store FileStore) NewUpload(ctx context.Context, info handler.FileInfo) (handler.Upload, error) {
	if info.ID == "" {
		info.ID = uid.FileID(info)
	}
	binPath := store.binPath(info.ID)
	info.Storage = map[string]string{
		"Type": "filestore",
		"Path": binPath,
	}

	// Create binary file with no content
	file, err := os.OpenFile(binPath, os.O_CREATE|os.O_WRONLY, defaultFilePerm)
	if err != nil {
		if os.IsNotExist(err) {
			err = fmt.Errorf("upload directory does not exist: %s", store.Path)
		}
		return nil, err
	}
	err = file.Close()
	if err != nil {
		return nil, err
	}

	upload := &fileUpload{
		info:     info,
		infoPath: store.infoPath(info.ID),
		binPath:  binPath,
	}

	// writeInfo creates the file by itself if necessary
	err = upload.writeInfo()
	if err != nil {
		return nil, err
	}

	return upload, nil
}

func (store FileStore) GetUpload(ctx context.Context, id string) (handler.Upload, error) {
	info := handler.FileInfo{}
	data, err := os.ReadFile(store.infoPath(id))
	if err != nil {
		if os.IsNotExist(err) {
			// Interpret os.ErrNotExist as 404 Not Found
			err = handler.ErrNotFound
		}
		return nil, err
	}
	if err := json.Unmarshal(data, &info); err != nil {
		return nil, err
	}

	binPath := store.binPath(id)
	infoPath := store.infoPath(id)
	stat, err := os.Stat(binPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Interpret os.ErrNotExist as 404 Not Found
			err = handler.ErrNotFound
		}
		return nil, err
	}

	info.Offset = stat.Size()

	return &fileUpload{
		info:     info,
		binPath:  binPath,
		infoPath: infoPath,
	}, nil
}

func (store FileStore) AsChecksumableUpload(upload handler.Upload) handler.ChecksumableUpload {
	return upload.(*fileUpload)
}

func (store FileStore) AsTerminatableUpload(upload handler.Upload) handler.TerminatableUpload {
	return upload.(*fileUpload)
}

func (store FileStore) AsLengthDeclarableUpload(upload handler.Upload) handler.LengthDeclarableUpload {
	return upload.(*fileUpload)
}

func (store FileStore) AsConcatableUpload(upload handler.Upload) handler.ConcatableUpload {
	return upload.(*fileUpload)
}

// binPath returns the path to the file storing the binary data.
func (store FileStore) binPath(id string) string {
	return filepath.Join(store.Path, id)
}

// infoPath returns the path to the .info file storing the file's info.
func (store FileStore) infoPath(id string) string {
	return filepath.Join(store.Path, id+".info")
}

type fileUpload struct {
	// info stores the current information about the upload
	info handler.FileInfo
	// infoPath is the path to the .info file
	infoPath string
	// binPath is the path to the binary file (which has no extension)
	binPath string
}

func (upload *fileUpload) GetInfo(ctx context.Context) (handler.FileInfo, error) {
	return upload.info, nil
}

func (upload *fileUpload) WriteChunk(ctx context.Context, offset int64, src io.Reader) (int64, error) {
	file, err := os.OpenFile(upload.binPath, os.O_WRONLY|os.O_APPEND, defaultFilePerm)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	n, err := io.Copy(file, src)

	upload.info.Offset += n
	return n, err
}

func (upload *fileUpload) GetReader(ctx context.Context) (io.Reader, error) {
	return os.Open(upload.binPath)
}

func (upload *fileUpload) Checksum(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
	v := r.Header.Get("Upload-Checksum")
	fmt.Println(v)
	//Upload-Checksum
	//upload.
	//if err := os.Remove(upload.infoPath); err != nil {
	//	return err
	//}
	//if err := os.Remove(upload.binPath); err != nil {
	//	return err
	//}
	return nil
}

func (upload *fileUpload) Terminate(ctx context.Context) error {
	if err := os.Remove(upload.infoPath); err != nil {
		return err
	}
	if err := os.Remove(upload.binPath); err != nil {
		return err
	}
	return nil
}

func (upload *fileUpload) ConcatUploads(ctx context.Context, uploads []handler.Upload) (err error) {
	file, err := os.OpenFile(upload.binPath, os.O_WRONLY|os.O_APPEND, defaultFilePerm)
	if err != nil {
		return err
	}
	defer file.Close()

	for _, partialUpload := range uploads {
		fileUpload := partialUpload.(*fileUpload)

		src, err := os.Open(fileUpload.binPath)
		if err != nil {
			return err
		}

		if _, err := io.Copy(file, src); err != nil {
			return err
		}
	}

	return
}

func (upload *fileUpload) DeclareLength(ctx context.Context, length int64) error {
	upload.info.Size = length
	upload.info.SizeIsDeferred = false
	return upload.writeInfo()
}

// writeInfo updates the entire information. Everything will be overwritten.
func (upload *fileUpload) writeInfo() error {
	data, err := json.Marshal(upload.info)
	if err != nil {
		return err
	}
	return os.WriteFile(upload.infoPath, data, defaultFilePerm)
}

func (upload *fileUpload) FinishUpload(ctx context.Context) error {
	return nil
}

// Query file by criteria
func (store FileStore) Query(ctx context.Context, criteria string) (result []byte, err error) {
	//TODO implement me
	rFileNamePattern, err := regexp.Compile(criteria)
	if err != nil {
		return
	}

	returnAll := false
	if criteria == ".*" {
		returnAll = true
	}

	var fileList []string
	infoRe := regexp.MustCompile("\\.info$")
	filepath.Walk(store.Path, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		if !infoRe.MatchString(info.Name()) {
			return nil
		}

		data, err := os.ReadFile(fmt.Sprintf("%s/%s", store.Path, info.Name()))
		if err != nil {
			return err
		}

		desc := make(map[string]interface{})
		err = json.Unmarshal(data, &desc)
		if err != nil {
			return err
		}

		if returnAll {
			fileList = append(fileList, string(data))
			return nil
		}

		d, ok := desc["MetaData"]
		if !ok {
			return nil
		}

		metadata, ok := d.(map[string]interface{})
		if !ok {
			return nil
		}

		d, ok = metadata["filename"]
		if !ok {
			return nil
		}

		filename, ok := d.(string)
		if !ok {
			return nil
		}

		if !rFileNamePattern.Match([]byte(filename)) {
			return nil
		}

		fileList = append(fileList, string(data))
		return nil
	})

	if len(fileList) == 0 {
		return nil, nil
	}

	return []byte("[" + strings.Join(fileList, ",") + "]"), nil
}
