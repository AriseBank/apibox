package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"sync"
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
	Tokens       []string `json:"tokens"`
	Limit        int64    `json:"limit"`
}

//Handler handles API HTTP request
type Handler struct {
	cfg        *Config
	nWorker    int
	ctask      chan *common.Task
	task       *common.Task
	result     chan giota.Trytes
	cmdLimiter *common.CmdLimiter
	wwait      *sync.Cond
	wfin       *sync.Cond
	sync.RWMutex
	finished chan giota.Trytes
}

func newHandler(cfg *Config) *Handler {
	return &Handler{
		cfg:        cfg,
		ctask:      make(chan *common.Task),
		result:     make(chan giota.Trytes),
		finished:   make(chan giota.Trytes),
		cmdLimiter: common.NewCmdLimiter(map[string]int64{"attachToTangle": cfg.Limit}, 9999),
		wwait:      sync.NewCond(&sync.Mutex{}),
		wfin:       sync.NewCond(&sync.Mutex{}),
	}
}

func (h *Handler) reset() {
	h.nWorker = 0
	h.task = nil
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
		log.Print(err)
		common.ErrResp(w, err)
		return
	}
	var c struct {
		Command string `json:"command"`
	}
	if err := json.Unmarshal(b, &c); err != nil {
		log.Print(err)
		common.ErrResp(w, err)
		return
	}
	if c.Command != "attachToTangle" {
		err := common.Loop(func() error {
			return bypass(h.cfg.IRIserver, w, b)
		})
		if err != nil {
			w.WriteHeader(400)
		}
		return
	}
	hdr := r.Header.Get(http.CanonicalHeaderKey("Authorization"))
	token := common.ParseAuthorizationHeader(hdr)
	if !common.IsValid(token, h.cfg.Tokens) && h.cfg.Limit != 0 {
		log.Print("not authed")
		limitReached := h.cmdLimiter.Limit(c.Command, r)
		if limitReached != nil {
			common.ErrResp(w, limitReached)
			return
		}
	} else {
		log.Print("authed")
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
		h.ctask <- t
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

func (h *Handler) proxy() {
	for {
		t := <-h.ctask
		h.Lock()
		h.task = t
		h.Unlock()
		h.wwait.Broadcast()
		f := <-h.finished
		h.wfin.Broadcast()
		h.Lock()
		h.reset()
		h.Unlock()
		h.result <- f
	}
}

//ServeControl handles controls.
func (h *Handler) ServeControl(w http.ResponseWriter, r *http.Request) {
	cmd := r.FormValue("cmd")
	if !common.Allowed(h.cfg.AllowWorker, r.RemoteAddr) {
		log.Print(r.RemoteAddr + " note allowed")
		w.WriteHeader(400)
		return
	}
	switch cmd {
	case "getwork":
		log.Println("called getwork")
		h.RLock()
		t := h.task
		h.RUnlock()
		if t == nil {
			h.wwait.L.Lock()
			h.wwait.Wait()
			h.wwait.L.Unlock()
		}
		t = h.task
		if t == nil {
			t = &common.Task{}
		}
		t.Trytes = common.Incr(t.Trytes, 1)
		h.Lock()
		h.nWorker++
		h.Unlock()
		common.WriteJSON(w, t)
	case "finished":
		log.Println("called finished")
		id := r.FormValue("ID")
		h.RLock()
		t := h.task
		h.RUnlock()
		if t == nil {
			log.Print("no work, already finished?")
			w.WriteHeader(400)
			return
		}
		h.RLock()
		tid := t.ID
		h.RUnlock()
		if id != fmt.Sprintf("%d", tid) {
			log.Print("id is incorrect now:", tid, " from worker:", id)
			w.WriteHeader(400)
			return
		}
		h.RLock()
		try := t.Trytes
		h.RUnlock()
		nonce := giota.Trytes(r.FormValue("trytes"))
		trytes := try[:len(try)-giota.NonceTrinarySize/3] + nonce
		tx, err := giota.NewTransaction(trytes)
		if err != nil {
			log.Print(err)
			return
		}
		h.RLock()
		mwm := t.MinWeightMagnitude
		h.RUnlock()
		if !tx.HasValidNonce(mwm) {
			log.Print("invalid MinWieightMagniture ", tx.Hash())
			return
		}
		h.finished <- trytes
	case "getstatus":
		log.Println("called getstatus")
		isWorking := false
		h.RLock()
		t := h.task
		h.RUnlock()
		if t != nil {
			h.wfin.L.Lock()
			h.wfin.Wait()
			h.wfin.L.Unlock()
		}
		if t != nil {
			isWorking = true
		}
		common.WriteJSON(w, &common.Status{
			Task:    h.task,
			N:       h.nWorker,
			Working: isWorking,
		})
	}
}

func (h *Handler) goPow() {
	go func() {
		for {
			t := <-h.ctask
			nonce, err := t.Pow()
			if err != nil {
				log.Print(err)
				continue
			}
			trytes := h.task.Trytes[:len(h.task.Trytes)-giota.NonceTrinarySize/3] + nonce
			h.task = nil
			h.result <- trytes
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
	if !h.cfg.Standalone {
		go h.proxy()
		http.HandleFunc("/control", h.ServeControl)
	}
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
