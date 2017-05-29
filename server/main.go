package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/didip/tollbooth"
	"github.com/iotaledger/giota"

	"github.com/iotaledger/apibox/common"
)

//APIHandler handles API HTTP request
type APIHandler struct {
	cfg *Config
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

func loop(f func() error) error {
	i := 0
	var err error
	for i = 0; i < 5; i++ {
		if err = f(); err == nil {
			break
		}
		log.Print(err)
		continue
	}
	if i == 5 {
		return err
	}
	return nil
}

//Command is for getting command in request json.
type Command struct {
	Command string `json:"command"`
}

func (h *APIHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ok, err := allowed(h.cfg.AllowRequest, r.RemoteAddr)
	if err != nil {
		log.Print(err)
	}
	if !ok {
		log.Print(r.RemoteAddr + " not allowed")
	}
	if err != nil || !ok {
		w.WriteHeader(400)
		return
	}

	b, err := ioutil.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(400)
		return
	}
	var c Command
	json.Unmarshal(b, &c)
	if c.Command != "attachToTangle" {
		err := loop(func() error {
			return bypass(h.cfg.IRIserver, w, b)
		})
		if err != nil {
			w.WriteHeader(400)
		}
		return
	}
	if err := attachToTangle(w, b); err != nil {
		log.Print(err)
		w.WriteHeader(400)
	}
}

func attachToTangle(w http.ResponseWriter, b []byte) error {
	var req giota.AttachToTangleRequest
	if err := json.Unmarshal(b, &req); err != nil {
		return err
	}
	resp, err := common.HandleAttachToTangle(&req)
	if err != nil {
		return err
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

//Config contains configs for server.
type Config struct {
	Debug        bool     `json:"debug"`
	ListenPort   int      `json:"listen_port"`
	IRIserver    string   `json:"iri_server_port"`
	AllowRequest []string `json:"allowed_request"`
	AllowWorker  []string `json:"allowed_worker"`
}

func allowed(cs []string, remote string) (bool, error) {
	ip, _, err := net.SplitHostPort(remote)
	if err != nil {
		return false, errors.New("invalid remote address " + err.Error())
	}
	r := net.ParseIP(ip)
	if r == nil {
		return false, errors.New("invalid remote address " + remote)
	}
	for _, item := range cs {
		if strings.Index(item, "-") >= 0 {
			cs = append(cs, strings.Split(item, "-")...)
		}
	}
	for _, item := range cs {
		if strings.Index(item, "/") < 0 {
			a := net.ParseIP(item)
			if a == nil {
				log.Fatal("invalid IP address in config file", item)
			}
			if r.Equal(a) {
				return true, nil
			}
			continue
		}
		_, a, err := net.ParseCIDR(item)
		if err != nil {
			log.Fatal("invalid IP address in config file", err)
		}
		if a.Contains(r) {
			return true, nil
		}
	}
	return false, nil
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

	h := APIHandler{
		cfg: &cfg,
	}
	http.Handle("/", tollbooth.LimitFuncHandler(tollbooth.NewLimiter(10, time.Second), h.ServeHTTP))

	s := &http.Server{
		Addr:           fmt.Sprintf(":%d", cfg.ListenPort),
		Handler:        http.DefaultServeMux,
		ReadTimeout:    3 * time.Minute,
		WriteTimeout:   3 * time.Minute,
		MaxHeaderBytes: 1 << 20,
	}
	log.Println("start server on port", cfg.ListenPort)
	log.Fatal(s.ListenAndServe())
}
