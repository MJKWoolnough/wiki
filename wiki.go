package main

import (
	"bytes"
	"encoding/xml"
	"io"
	"net/http"
	"os"
	"path"
	"strings"
	"sync"

	"github.com/MJKWoolnough/memio"
	"github.com/MJKWoolnough/pages"
)

func New(dir string, p *pages.Pages) http.Handler {
	return &wiki{
		dir:   path.Clean(dir),
		pages: p,
	}
}

type wiki struct {
	dir   string
	pages *pages.Pages
}

var wikiLock sync.RWMutex

func (wk *wiki) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	p := path.Clean(string(wk) + strings.TrimPrefix(r.URL.Path, "/wiki/") + ".part")
	os.MkdirAll(path.Dir(p), 0755)
	if r.Method == http.MethodPost {
		var b memio.Buffer
		v := verifyHTML(r.Body, &b)
		r.Body.Close()
		if !v {
			w.WriteHeader(http.StatusNotAcceptable)
		} else if err := writePage(p, b); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(err.Error()))
		} else {
			w.WriteHeader(http.StatusOK)
		}
		return
	}
	wikiLock.RLock()
	defer wikiLock.RUnlock()
	f, err := os.Open(p)
	if err != nil && !os.IsNotExist(err) {
		w.WriteHeader(http.StatusForbidden)
		return
	}
	header, _ := os.Open(string(wk) + "header.html")
	footer, _ := os.Open(string(wk) + "footer.html")
	io.Copy(w, header)
	if f != nil {
		io.Copy(w, f)
	}
	io.Copy(w, footer)
}

func verifyHTML(r io.Reader, w io.Writer) bool {
	d := xml.NewDecoder(r)
	d.Strict = false
	d.AutoClose = xml.HTMLAutoClose
	d.Entity = xml.HTMLEntity
	e := xml.NewEncoder(w)
	for {
		t, err := d.Token()
		if err != nil {
			if err == io.EOF {
				break
			}
			return false
		}
		e.EncodeToken(t)
	}
	e.Flush()
	return true
}

var (
	oldBR = []byte("<br></br>")
	newBR = []byte("<br />")
	oldHR = []byte("<hr></hr>")
	newHR = []byte("<hr />")
)

func writePage(page string, b []byte) error {
	b = bytes.Replace(b, oldBR, newBR, -1)
	b = bytes.Replace(b, oldHR, newHR, -1)
	wikiLock.Lock()
	defer wikiLock.Unlock()
	f, err := os.Create(page)
	if err != nil {
		return err
	}
	if _, err = f.Write(b); err != nil {
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return nil
}
