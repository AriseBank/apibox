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

Use glide to install the dependencies:

```
# glide install
```

and then build the server :

```
# go build -o server ./server
```

and worker :

```
# go build -o worker ./worker
```

If you want to use GPU for PoW, add `-tags=gpu` option.

## Server Settings

[server.json](server/server.json) in server directory is the settings for server. Parameters are:

* `debug`: If true, print log to stdout.
* `listen_port`: listen port for API and workers.
* `allowed_request`: IP address or CIDR representation which you want to allow for API caller. 
* `allowed_worker`: IP address or CIDR representation which you want to allow for worker. 
* `standalone`: If  true, no worker is accepted and only a server does PoW.

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

TODO
=========================

* [ ] Tests :(

<hr>

Released under the [MIT License](LICENSE).
