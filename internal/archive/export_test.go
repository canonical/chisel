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
