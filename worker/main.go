package main

import (
	"encoding/json"
	"flag"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/iotaledger/apibox/common"
	"github.com/iotaledger/giota"
)

type work struct {
	server string
	result giota.Trytes
	task   *common.Task
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
		if w.task == nil {
			continue
		}
		if w.result != "" {
			log.Println("sending finished...")
			values = url.Values{"cmd": {"finished"}, "trytes": {string(w.result)}}
		} else {
			values = url.Values{"cmd": {"getstatus"}}
		}
		req.URL.RawQuery = values.Encode()
		resp, err := client.Do(req)
		if err != nil {
			log.Print(err)
			continue
		}
		var status common.Status
		if err := readJSON(resp, &status); err != nil {
			log.Print(err)
			continue
		}
		if !status.Working {
			w.task.StopPow()
			w.task = nil
			w.result = ""
		}
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
		w.task = &task
		w.result, err = task.Pow()
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
