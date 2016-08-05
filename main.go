package main

import (
	"errors"
	"github.com/mssola/user_agent"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"os"
)

type ReplaceBody struct {
	Body         io.ReadCloser
	Position     int64
	RewriteBytes *RewriteBytes
}

type RewriteBytes struct {
	StartPosition int64
	EndPosition   int64
	Elements      []RewriteByte
}

type RewriteByte struct {
	Position int64
	Byte     byte
}

func (rb *ReplaceBody) Read(p []byte) (n int, err error) {
	n, err = rb.Body.Read(p)
	start := rb.Position
	stop := rb.Position + int64(n)
	if rb.hit(n) {
		for _, e := range rb.RewriteBytes.Elements {
			if e.Position >= start && e.Position <= stop {
				offset := e.Position - rb.Position
				copy(p[offset:offset+1], []byte{e.Byte})
			}
		}

	}
	rb.Position += int64(n)
	return n, err
}

func (rb *ReplaceBody) Close() error {
	return rb.Body.Close()
}

func (rb *ReplaceBody) hit(bufferLength int) bool {
	bl := int64(bufferLength)
	return (rb.Position <= rb.RewriteBytes.StartPosition && rb.Position+bl >= rb.RewriteBytes.StartPosition) ||
		(rb.Position <= rb.RewriteBytes.EndPosition && rb.Position+bl >= rb.RewriteBytes.EndPosition)
}

type ReplaceTransport struct {
	DimensionsCache *DimensionsCache
}

func (t *ReplaceTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	d, err := t.DimensionsCache.Get(req.URL)
	if err != nil {
		return nil, err
	}

	res, err := http.DefaultTransport.RoundTrip(req)
	if err != nil {
		return nil, err
	}

	rb := &RewriteBytes{
		StartPosition: d.WidthPosition,
		EndPosition:   d.HeightPosition + int64(len(d.Height)),
		Elements:      make([]RewriteByte, len(d.Width)+len(d.Height)),
	}

	for i := range d.Width {
		rb.Elements[i] = RewriteByte{Position: d.WidthPosition + int64(i), Byte: d.Height[i]}
	}

	for i := range d.Height {
		rb.Elements[i + len(d.Width)] = RewriteByte{Position: d.HeightPosition + int64(i), Byte: d.Width[i]}
	}

	outres := new(http.Response)
	*outres = *res // includes shallow copies of maps, but okay
	outres.Body = &ReplaceBody{
		Position:     t.getRangeStart(req),
		Body:         res.Body,
		RewriteBytes: rb,
	}
	return outres, nil
}

func (t *ReplaceTransport) getRangeStart(req *http.Request) int64 {
	requestRange := req.Header.Get("Range")
	if requestRange == "" {
		return 0
	}

	re := regexp.MustCompile(`bytes=([0-9]+)-`)
	r := re.FindStringSubmatch(requestRange)
	if r == nil {
		panic("Cannot parse range start")
	}

	result, err := strconv.ParseInt(r[1], 10, 64)
	if err != nil {
		panic("Cannot parse range start")
	}

	return result
}

type HttpHandler struct {
	DimensionsCache *DimensionsCache
	Proxy           *httputil.ReverseProxy
}

func NewHttpHandler(dc *DimensionsCache) *HttpHandler {

	httpHandler := new(HttpHandler)
	httpHandler.DimensionsCache = dc
	httpHandler.Proxy = &httputil.ReverseProxy{
		Director: func(request *http.Request) {
			url, err := httpHandler.getFileUrl(request)
			check(err)

			request.Host = url.Host
			request.URL = url
		},
		Transport: &ReplaceTransport{
			DimensionsCache: dc,
		},
	}
	return httpHandler
}

func (h *HttpHandler) getFileUrl(r *http.Request) (*url.URL, error) {
	queryString := r.URL.Query().Get("url")
	if queryString == "" {
		return nil, errors.New("Empty URL")
	}
	url, err := url.Parse(queryString)
	return url, err

}

func (h *HttpHandler) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	url, err := h.getFileUrl(r)
	if err != nil {
		rw.WriteHeader(http.StatusBadRequest)
		return
	}

	redirect := func() {
		http.Redirect(rw, r, url.String(), http.StatusTemporaryRedirect)
	}

	ua := user_agent.New(r.UserAgent())
	name, version := ua.Browser()
	if name != "Chrome" || strings.Index(version, "52.") != 0 {
		redirect()
		return
	}

	d, err := h.DimensionsCache.Get(url)
	if err != nil {
		rw.WriteHeader(http.StatusInternalServerError)
		return
	}

	if d.SwapHeightAndWidth == 0 {
		redirect()
		return
	}

	h.Proxy.ServeHTTP(rw, r)
}

func main() {
	dc := NewDimensionsCache()
	dc.Start("http://localhost:5001")

	s := &http.Server{
		Addr:           getAddr(),
		Handler:        NewHttpHandler(dc),
		MaxHeaderBytes: 1 << 20,
	}
	log.Fatal(s.ListenAndServe())
}

func getAddr () string {
	s := os.Getenv("PORT")
	if s == "" {
		return ":5000"
	}

	return ":" + s
}

func check(e error) {
	if e != nil {
		panic(e)
	}
}
