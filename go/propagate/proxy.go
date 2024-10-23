package propagate

import (
	"bytes"
	"net/http"
	"sparrowhawktech/toolkit/util"
)

const (
	HttpRequestProxyRequestName  = "httpRequestProxy"
	ProxyTargetAddressHeaderName = "propagate-Proxy-Target-Address"
)

type ProxyRequestData struct {
	Path     *string     `json:"path"`
	RawQuery *string     `json:"rawQuery"`
	Header   http.Header `json:"header"`
	PostData []byte      `json:"postData"`
}

func NewProxyRequesData(path string, rawQuery *string, header http.Header, postData []byte) *ProxyRequestData {
	return &ProxyRequestData{
		Path:     &path,
		RawQuery: rawQuery,
		Header:   header,
		PostData: postData,
	}
}

type ProxyRequestResult struct {
	StatusCode *int        `json:"statusCode"`
	Header     http.Header `json:"header"`
	Data       []byte      `json:"data"`
}

type ProxyResponseWriter struct {
	http.ResponseWriter
	header     http.Header
	statusCode int
	data       bytes.Buffer
}

func (o *ProxyResponseWriter) Header() http.Header {
	return o.header
}

func (o *ProxyResponseWriter) Write(data []byte) (int, error) {
	return o.data.Write(data)
}

func (o *ProxyResponseWriter) WriteHeader(statusCode int) {
	o.statusCode = statusCode
}

type ProxyPacketHandler struct {
	serveMux *http.ServeMux
}

func (o *ProxyPacketHandler) Handle(transactionId string, payload []byte, trajectory []string) []byte {
	data := ProxyRequestData{}
	util.Unmarshal(payload, &data)
	buffer := bytes.Buffer{}
	if data.PostData != nil {
		util.Write(&buffer, data.PostData)
	}
	location := *data.Path
	if data.RawQuery != nil && *data.RawQuery != "" {
		location += "?" + *data.RawQuery
	}
	r, err := http.NewRequest("POST", location, &buffer)
	util.CheckErr(err)
	w := &ProxyResponseWriter{header: make(http.Header), statusCode: 200}
	o.serveMux.ServeHTTP(w, r)
	result := ProxyRequestResult{
		StatusCode: &w.statusCode,
		Header:     w.header,
		Data:       w.data.Bytes(),
	}
	return util.Marshal(result)
}

func NewProxyPacketHandler(serveMux *http.ServeMux) *ProxyPacketHandler {
	return &ProxyPacketHandler{serveMux: serveMux}
}
