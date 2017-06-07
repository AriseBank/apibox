[![Build Status](https://travis-ci.org/iotaledger/apibox.svg?branch=master)](https://travis-ci.org/iotaledger/apibox)
[![GitHub license](https://img.shields.io/badge/license-MIT-blue.svg)](https://raw.githubusercontent.com/iotaledger/apibox/master/LICENSE)

# :baby_chick: IOTA APIBOX

## API endpoints

All APIs of IOTA except `attachToTangle` mirror IRI behaviour, i.e. they expect the 
same json and will return the same answer unless there's an error caused by the
webserver itself.

`attachToTangle` acts like a _mining pool_ in bitcoin. `worker`s connect to a `server`
and get and do a part of Proof of Work(PoW) until one of workers finish the PoW.

## Building

You will need C compiler to compile PoW routine in C.

You will need C compiler and OpenCL environemnt(hardware and software) to compile PoW routine for GPU.

Use glide to install the dependencies:

```
# glide install
```


Build the server :

```
# go build -o server ./server
```

and the worker :

```
# go build -o worker ./worker
```

If you want to use GPU for PoW, add `-tags=gpu` option.

## Server Settings

[server.json](server/server.json) in server directory is the settings for server. Parameters are:

* `listen_port`: listen port for API and workers.
*  `iri_server_port`: iri server and port, in form of server:port.
* `allowed_request`: IP address or CIDR representation which you want to allow for API caller. 
* `allowed_worker`: IP address or CIDR representation which you want to allow for worker. 
* `debug`: If true, print log to stdout.
* `standalone`: If  true, no worker is accepted and only a server does PoW.
* `tokens`: tokens for authentication.

`tokens` can be made with `mkpasswd`:

```
    $ go run mkpasswd/main.go
    Token: <input some tokens>

    added to "token" in server.json: LPJNul+wow4m6DsqxbninhsWHlwfp0JecwQzYpOLmCQ=
```

then add `LPJNul+wow4m6DsqxbninhsWHlwfp0JecwQzYpOLmCQ=` to `server.json`.

```
{
    "debug":true,
    "listen_port":14265,
    "iri_server_port":"",
    "allowed_request":["::1","192.168.1.0/24"],
    "allowed_worker":["::1","192.168.1.0/24"],
    "standalone":false,
    "tokens":["LPJNul+wow4m6DsqxbninhsWHlwfp0JecwQzYpOLmCQ="]
}
```


## Worker Arguments

* `verbose`:  If true, print log to stdout.
* `url`: URL to server, `http://<IP address>:<port>`

## Running APIBOX

Run server:

```
$ server/server
```

Run workers as many as you want:

```
$ worker/worker -url="http://server1:14265"
```

then, request `attachToTangle`:

```
$ curl http://localhost:14265   -X POST   -H 'Content-Type: application/json'   -d '{"command": "attachToTangle", "trunkTransaction": "JVMTDGDPDFYHMZPMWEKKANBQSLSDTIIHAYQUMZOKHXXXGJHJDQPOMDOMNRDKYCZRUFZROZDADTHZC9999", "branchTransaction": "P9KFSJVGSPLXAEBJSHWFZLGP9GGJTIO9YITDEHATDTGAFLPLBZ9FOFWWTKMAZXZHFGQHUOXLXUALY9999", "minWeightMagnitude": 18, "trytes": ["999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999A9RGRKVGWMWMKOLVMDFWJUHNUNYWZTJADGGPZGXNLERLXYWJE9WQHWWBMCPZMVVMJUMWWBLZLNMLDCGDJ999999999999999999999999999999999999999999999999999999YGYQIVD99999999999999999999TXEFLKNPJRBYZPORHZU9CEMFIFVVQBUSTDGSJCZMBTZCDTTJVUFPTCCVHHORPMGCURKTH9VGJIXUQJVHK999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999"]}'
```

or you can authenticate by adding `Authenticatation: token <token>`  to header, as:

```
$ curl http://localhost:14265   -X POST   -H "Authorization:token moemoe"  -H 'Content-Type: application/json'   -d '{"command": "attachToTangle", "trunkTransaction": "JVMTDGDPDFYHMZPMWEKKANBQSLSDTIIHAYQUMZOKHXXXGJHJDQPOMDOMNRDKYCZRUFZROZDADTHZC9999", "branchTransaction": "P9KFSJVGSPLXAEBJSHWFZLGP9GGJTIO9YITDEHATDTGAFLPLBZ9FOFWWTKMAZXZHFGQHUOXLXUALY9999", "minWeightMagnitude": 18, "trytes": ["999999999999...."]}'
```

TODO
=========================

* [ ] Tests :(

<hr>

Released under the [MIT License](LICENSE).
