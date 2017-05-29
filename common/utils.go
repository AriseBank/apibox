package common

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path"
	"time"

	"github.com/iotaledger/giota"

	lumberjack "gopkg.in/natefinch/lumberjack.v2"
)

//SetLogger setups logger. whici outputs nothing, or file , or file and stdout
func SetLogger(logdir string, debug bool) {
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)

	l := &lumberjack.Logger{
		Filename:   path.Join(logdir, "powder_server.log"),
		MaxSize:    5, // megabytes
		MaxBackups: 10,
		MaxAge:     28, //days
	}
	if debug {
		fmt.Println("outputs logs to stdout and ", logdir)
		m := io.MultiWriter(os.Stdout, l)
		log.SetOutput(m)
	}
}

//HandleAttachToTangle does PoW and returns result transaction as AttachToTangle API.
func HandleAttachToTangle(req *giota.AttachToTangleRequest) (*giota.AttachToTangleResponse, error) {
	if len(req.Trytes) < 1 {
		return nil, errors.New("no trytes supplied")
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
		n, pow := giota.GetBestPoW()
		log.Println("using PoW", n)
		out, err := pow(ts.Trytes(), int(req.MinWeightMagnitude))
		if err != nil {
			return nil, err
		}
		prevTxHash = out
		tx, err := giota.NewTransaction(out)
		if err != nil {
			return nil, err
		}
		outTrytes[i] = *tx
	}

	finishedAt := time.Now().Unix()
	resp := &giota.AttachToTangleResponse{
		Trytes:   outTrytes,
		Duration: finishedAt - s.Unix(),
	}
	return resp, nil
}
