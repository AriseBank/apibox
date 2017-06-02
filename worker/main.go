package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"time"

	"sync"

	"github.com/iotaledger/apibox/common"
	"github.com/iotaledger/giota"
)

type work struct {
	server string
	result giota.Trytes
	task   *common.Task
	sync.RWMutex
}

func readJSON(resp *http.Response, t interface{}) error {
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Print(err)
		}
	}()
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(b, t); err != nil {
		return err
	}
	return nil
}

func (w *work) getstatus() {
	var err error
	req, err := http.NewRequest("GET", w.server+"/control", nil)
	if err != nil {
		log.Fatal(err)
	}
	var values url.Values
	client := new(http.Client)
	for {
		time.Sleep(15 * time.Second)
		w.RLock()
		if w.task == nil {
			w.RUnlock()
			continue
		}
		if w.result != "" {
			log.Println("sending finished...")
			values = url.Values{"cmd": {"finished"}, "ID": {fmt.Sprintf("%d", w.task.ID)}, "trytes": {string(w.result)}}
		} else {
			values = url.Values{"cmd": {"getstatus"}}
		}
		w.RUnlock()
		req.URL.RawQuery = values.Encode()
		resp, err := client.Do(req)
		if err != nil {
			log.Print(err)
			continue
		}
		var status common.Status
		if err := readJSON(resp, &status); err == nil {
			if status.Working {
				continue
			}
		} else {
			log.Print(err)
		}
		w.task.StopPow()
		w.Lock()
		w.task = nil
		w.result = ""
		w.Unlock()
	}
}

func (w *work) getwork() {
	var err error
	req, err := http.NewRequest("GET", w.server+"/control", nil)
	if err != nil {
		log.Fatal(err)
	}
	values := url.Values{}
	values.Add("cmd", "getwork")
	req.URL.RawQuery = values.Encode()
	client := new(http.Client)

	for {
		time.Sleep(15 * time.Second)
		resp, err := client.Do(req)
		if err != nil {
			log.Print(err)
			continue
		}
		var task common.Task
		if err = readJSON(resp, &task); err != nil {
			log.Print(err)
			continue
		}
		if task.ID == 0 {
			log.Println("no work...")
			continue
		}
		w.Lock()
		w.task = &task
		w.result, err = task.Pow()
		w.Unlock()
		if err != nil {
			log.Print(err)
			continue
		}
	}
}

func main() {
	var verbose bool
	var server string
	flag.BoolVar(&verbose, "verbose", false, "print logs")
	flag.StringVar(&server, "url", "http://localhost:14265", "server ip:port")
	flag.Parse()
	common.SetLogger(".", verbose)
	log.Println("connecting...")
	w := work{
		server: server,
	}
	go func() {
		w.getwork()
	}()
	go func() {
		w.getstatus()
	}()
	pause := make(chan struct{})
	<-pause
}
