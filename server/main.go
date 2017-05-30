package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/didip/tollbooth"
	"github.com/iotaledger/apibox/common"
	"github.com/iotaledger/giota"
)

//Config contains configs for server.
type Config struct {
	ListenPort   int      `json:"listen_port"`
	IRIserver    string   `json:"iri_server_port"`
	AllowRequest []string `json:"allowed_request"`
	AllowWorker  []string `json:"allowed_worker"`
	Debug        bool     `json:"debug"`
	Standalone   bool     `json:"standalone"`
}

//Handler handles API HTTP request
type Handler struct {
	cfg     *Config
	nWorker int
	result  chan giota.Trytes
	ready   chan struct{}
	task    *common.Task
}

func newHandler(cfg *Config) *Handler {
	return &Handler{
		cfg:    cfg,
		ready:  make(chan struct{}, 1),
		result: make(chan giota.Trytes),
	}
}

func (h *Handler) reset() {
	h.nWorker = 0
	h.task = nil
	<-h.ready
}

//ServeAPI handles API.
//If not AttachToTangle command, just call IRI server and returns its response.
//If AttachToTangle, do PoW by myself or workers.
func (h *Handler) ServeAPI(w http.ResponseWriter, r *http.Request) {
	if !common.Allowed(h.cfg.AllowRequest, r.RemoteAddr) {
		err := errors.New(r.RemoteAddr + " not allowed")
		log.Print(err)
		common.ErrResp(w, err)
		return
	}

	b, err := ioutil.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(400)
		return
	}
	var c struct {
		Command string `json:"command"`
	}
	json.Unmarshal(b, &c)
	if c.Command != "attachToTangle" {
		err := common.Loop(func() error {
			return bypass(h.cfg.IRIserver, w, b)
		})
		if err != nil {
			w.WriteHeader(400)
		}
		return
	}
	if err := h.attachToTangle(w, b); err != nil {
		log.Print(err)
		common.ErrResp(w, err)
	}
}

func bypass(iri string, w http.ResponseWriter, b []byte) error {
	rd := bytes.NewReader(b)
	req, err := http.NewRequest("POST", iri, rd)
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer func() {
		if err = resp.Body.Close(); err != nil {
			log.Println(err)
		}
	}()

	bs, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	origin := resp.Header.Get("Access-Control-Allow-Origin")
	if origin != "" {
		w.Header().Set("Access-Control-Allow-Origin", origin)
	}
	w.WriteHeader(resp.StatusCode)
	_, err = w.Write(bs)
	return err
}

//ServeControl handles controls.
func (h *Handler) ServeControl(w http.ResponseWriter, r *http.Request) {
	cmd := r.FormValue("cmd")
	if h.cfg.Standalone {
		if cmd != "getStatus" {
			log.Print("cannot call control while standalone mode")
			w.WriteHeader(400)
			return
		}
	}
	if !common.Allowed(h.cfg.AllowWorker, r.RemoteAddr) {
		log.Print(r.RemoteAddr + " note allowed")
		w.WriteHeader(400)
		return
	}
	switch cmd {
	case "getwork":
		log.Println("called getwork")
		var t common.Task
		if h.task != nil {
			t = *h.task
		}
		t.Trytes = common.Incr(t.Trytes, h.nWorker)
		h.nWorker++
		common.WriteJSON(w, t)
	case "finished":
		log.Println("called finished")
		trytes := giota.Trytes(r.FormValue("trytes"))
		tx, err := giota.NewTransaction(trytes)
		if err != nil {
			log.Print(err)
			return
		}
		if !tx.HasValidNonce(h.task.MinWeightMagnitude) {
			log.Print("invalid MinWieightMagniture", tx.Hash())
			return
		}
		h.result <- trytes
		h.reset()
		fallthrough
	case "getstatus":
		log.Println("called getstatus")
		id := r.FormValue("ID")
		if id != "" && id != strconv.Itoa(int(h.task.ID)) {
			log.Print("incorrect ID", id)
			w.WriteHeader(400)
			return
		}
		isWorking := false
		if h.task != nil {
			isWorking = true
		}
		common.WriteJSON(w, &common.Status{
			Task:    h.task,
			N:       h.nWorker,
			Working: isWorking,
		})
	}
}

func (h *Handler) attachToTangle(w http.ResponseWriter, b []byte) error {
	var req giota.AttachToTangleRequest
	if err := json.Unmarshal(b, &req); err != nil {
		return err
	}
	if len(req.Trytes) < 1 {
		return errors.New("no trytes supplied")
	}
	outTrytes := make([]giota.Transaction, len(req.Trytes))
	var prevTxHash giota.Trytes

	s := time.Now()
	for i, ts := range req.Trytes {
		if prevTxHash == "" {
			ts.TrunkTransaction = req.TrunkTransaction
			ts.BranchTransaction = req.BranchTransaction
		} else {
			ts.TrunkTransaction = prevTxHash
			ts.BranchTransaction = req.TrunkTransaction
		}
		t := &common.Task{
			ID:                 time.Now().Unix(),
			MinWeightMagnitude: req.MinWeightMagnitude,
			Trytes:             ts.Trytes(),
		}
		h.ready <- struct{}{}
		h.task = t
		prevTxHash = <-h.result
		tx, err := giota.NewTransaction(prevTxHash)
		if err != nil {
			return err
		}
		outTrytes[i] = *tx
	}

	finishedAt := time.Now().Unix()
	resp := &giota.AttachToTangleResponse{
		Trytes:   outTrytes,
		Duration: finishedAt - s.Unix(),
	}
	bs, err := json.Marshal(resp)
	if err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	_, err = w.Write(bs)
	return err
}

func (h *Handler) goPow() {
	go func() {
		for {
			if h.task == nil {
				time.Sleep(15 * time.Second)
				continue
			}
			trytes, err := h.task.Pow()
			if err != nil {
				log.Print(err)
				continue
			}
			h.task = nil
			h.result <- trytes
			<-h.ready
		}
	}()
}

func main() {
	bytes, err := ioutil.ReadFile("server.json")
	if err != nil {
		log.Fatal(err)
	}
	var cfg Config
	if err := json.Unmarshal(bytes, &cfg); err != nil {
		log.Fatal(err)
	}

	common.SetLogger(".", cfg.Debug)

	if cfg.IRIserver == "" {
		cfg.IRIserver = giota.RandomNode()
	}
	log.Print("using IRI server ", cfg.IRIserver)

	h := newHandler(&cfg)

	http.Handle("/", tollbooth.LimitFuncHandler(tollbooth.NewLimiter(10, time.Second), h.ServeAPI))
	http.Handle("/control", tollbooth.LimitFuncHandler(tollbooth.NewLimiter(10, time.Second), h.ServeControl))

	s := &http.Server{
		Addr:           fmt.Sprintf(":%d", cfg.ListenPort),
		Handler:        http.DefaultServeMux,
		ReadTimeout:    30 * time.Minute,
		WriteTimeout:   30 * time.Minute,
		MaxHeaderBytes: 1 << 20,
	}
	if cfg.Standalone {
		h.goPow()
	}
	log.Println("start server on port", cfg.ListenPort)
	log.Fatal(s.ListenAndServe())
}
