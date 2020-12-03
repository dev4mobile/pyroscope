package server

import (
	"net/http"
	"time"

	"github.com/petethepig/pyroscope/pkg/attime"
	"github.com/petethepig/pyroscope/pkg/storage/tree"
	"github.com/petethepig/pyroscope/pkg/testing"
	log "github.com/sirupsen/logrus"
)

type ingestParams struct {
	grouped           bool
	format            string
	labels            string
	samplingFrequency int
	modifiers         []string
	from              time.Time
	until             time.Time
}

func ingestParamsFromRequest(r *http.Request) *ingestParams {
	ip := &ingestParams{}
	q := r.URL.Query()
	ip.grouped = q.Get("grouped") != ""

	if qt := q.Get("from"); qt != "" {
		ip.from = attime.Parse(qt)
	} else {
		ip.from = time.Now()
	}

	if qt := q.Get("until"); qt != "" {
		ip.until = attime.Parse(qt)
	} else {
		ip.until = time.Now()
	}

	ip.labels = normalizeLabels(q)

	return ip
}

func (ctrl *Controller) ingestHandler(w http.ResponseWriter, r *http.Request) {
	ip := ingestParamsFromRequest(r)
	parserFunc := parseIndividualLines
	if ip.grouped {
		parserFunc = parseGroups
	}

	if r.Header.Get("Content-Type") == "binary/octet-stream+trie" {
		parserFunc = parseTrie
	}

	t := tree.New()

	samples := 0
	i := 0
	log.Debugf("inserting into bucket %s", ip.labels)
	testing.Profile("put-"+r.URL.Query().Get("from"), func() {
		parserFunc(r.Body, func(k []byte, v int) {
			samples += v * globalMultiplier
			i++
			t.Insert(k, uint64(v))
		})
		log.Debug("lines", i)

		timer, err := ctrl.s.Put(ip.from, ip.until, ip.labels, t)
		if err != nil {
			log.Fatal(err)
		}
		w.WriteHeader(200)
		w.Write(timer.Marshal())
	})
}
