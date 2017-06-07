package common

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path"
	"regexp"
	"strings"
	"time"

	"github.com/didip/tollbooth"
	"github.com/didip/tollbooth/config"
	terrors "github.com/didip/tollbooth/errors"
	"github.com/didip/tollbooth/libstring"
	"github.com/iotaledger/giota"
	"github.com/natefinch/lumberjack"
)

//Status represents PoW status
type Status struct {
	Task    *Task `json:"task_info"`
	Working bool  `json:"working"`
	N       int   `json:"worker_number"`
}

//Task represents PoW task with ID.
type Task struct {
	ID                 int64        `json:"id"`
	MinWeightMagnitude int64        `json:"minWeightMagnitude"`
	Trytes             giota.Trytes `json:"trytes"`
}

//SetLogger setups logger. whici outputs nothing, or file , or file and stdout
func SetLogger(logdir string, debug bool) {
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)

	l := &lumberjack.Logger{
		Filename:   path.Join(logdir, "apibox.log"),
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

var pow giota.PowFunc
var pname string

func init() {
	pname, pow = giota.GetBestPoW()
}

//Pow does PoW and returns result transaction as AttachToTangle API.
func (t *Task) Pow() (giota.Trytes, error) {
	log.Println("Doing PoW by", pname)
	out, err := pow(t.Trytes, int(t.MinWeightMagnitude))
	if err != nil {
		return "", err
	}
	log.Println("finished PoW")
	return out, nil
}

//StopPow stops PoW
func (t *Task) StopPow() {
	pow("", 0)
}

//Loop loops func f for 5 times.
func Loop(f func() error) error {
	var i int
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

//WriteJSON marshal v and writes to v.
func WriteJSON(w http.ResponseWriter, v interface{}) {
	bs, err := json.Marshal(v)
	if err != nil {
		log.Print(err)
		w.WriteHeader(400)
		return
	}
	if _, err := w.Write(bs); err != nil {
		log.Print(err)
	}
}

//ErrResp write error contents to w and set status=400.
func ErrResp(w http.ResponseWriter, err error) {
	w.WriteHeader(400)
	WriteJSON(w, struct {
		Error    string `json:"error"`
		Duration int    `json:"duration"`
	}{
		err.Error(),
		0,
	})
}

//Incr increments last Trytes of tr.
func Incr(tr giota.Trytes, plus int) giota.Trytes {
	btr := []byte(tr)
	for j := 0; j < plus; j++ {
		for i := len(btr) - 1; i > 0; i-- {
			c := btr[i]
			if c == '9' {
				btr[i] = 'A'
				return giota.Trytes(btr)
			}
			if c >= 'A' && c <= 'Y' {
				btr[i]++
				return giota.Trytes(btr)
			}
			btr[i] = '9'
		}
	}
	return giota.Trytes(btr)
}

//Allowed returns true if IP address remote  matches cs.
func Allowed(cs []string, remote string) bool {
	ip, _, err := net.SplitHostPort(remote)
	if err != nil {
		log.Fatal("invalid remote address " + err.Error())
	}
	r := net.ParseIP(ip)
	if r == nil {
		log.Fatal("invalid remote address " + remote)
	}
	for _, item := range cs {
		if strings.Contains(item, "-") {
			cs = append(cs, strings.Split(item, "-")...)
		}
	}
	for _, item := range cs {
		if !strings.Contains(item, "/") {
			a := net.ParseIP(item)
			if a == nil {
				log.Fatal("invalid IP address in config file", item)
			}
			if r.Equal(a) {
				return true
			}
			continue
		}
		_, a, err := net.ParseCIDR(item)
		if err != nil {
			log.Fatal("invalid IP address in config file", err)
		}
		if a.Contains(r) {
			return true
		}
	}
	return false
}

// ParseAuthorizationHeader expects a string in the form of
// `token authtoken` and returns the `authtoken` or an empty string.
func ParseAuthorizationHeader(h string) string {
	parts := regexp.MustCompile(`\s+`).Split(h, 2)
	if len(parts) != 2 {
		return ""
	} else if strings.ToLower(parts[0]) != "token" {
		return ""
	}

	return strings.TrimSpace(parts[1])
}

//IsValid returns true if token is in tokens.
func IsValid(token string, tokens []string) bool {
	t := sha256.Sum256([]byte(token))
	b64 := base64.StdEncoding.EncodeToString(t[:])
	for _, tk := range tokens {
		if tk == b64 {
			return true
		}
	}
	return false
}

//CmdLimiter define rate limite.
type CmdLimiter struct {
	limiters map[string]*config.Limiter
	fallback *config.Limiter
}

//NewCmdLimiter creates and returns CmdLimiter struct.
func NewCmdLimiter(limits map[string]int64, def int64) *CmdLimiter {
	limiters := map[string]*config.Limiter{}
	for k, v := range limits {
		limiters[k] = tollbooth.NewLimiter(v, 1*time.Minute)
	}

	defLim := tollbooth.NewLimiter(def, 1*time.Minute)
	clim := &CmdLimiter{fallback: defLim, limiters: limiters}

	return clim
}

//Limit returns HttpError if over limit.
func (c *CmdLimiter) Limit(cmd string, r *http.Request) *terrors.HTTPError {
	l, ok := c.limiters[cmd]
	remoteIP := libstring.RemoteIP(c.fallback.IPLookups, r)
	keys := []string{remoteIP, cmd}
	if !ok { // Use fallback if cmd was not found.
		return tollbooth.LimitByKeys(c.fallback, keys)
	}

	return tollbooth.LimitByKeys(l, keys)
}
