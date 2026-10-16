package archive

import (
	"net/http"
)

func FakeDo(do func(req *http.Request) (*http.Response, error)) (restore func()) {
	_httpDo := httpDo
	_bulkDo := bulkDo
	httpDo = do
	bulkDo = do
	return func() {
		httpDo = _httpDo
		bulkDo = _bulkDo
	}
}

type Credentials = credentials

var FindCredentials = findCredentials
var FindCredentialsInDir = findCredentialsInDir

func FakeDefaultCredsDir(dir string) (restore func()) {
	_defaultCredsDir := defaultCredsDir
	defaultCredsDir = dir
	return func() {
		defaultCredsDir = _defaultCredsDir
	}
}

var ProArchiveInfo = proArchiveInfo
