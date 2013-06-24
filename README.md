# MediaServer for mongodb GridFS Documentation

This server intended to serve files from mongodb GridFS.

## How it works

This server works as HTTP server and expose two endpoints:

* `/ping` respond `OK` with status code 200
* `/:filename`
  * if file with `:filename` finded in connected GridFS:
      * send finded file over HTTP and set status code 200
      * in case of malformed `Range` header it return error and appropriate status code
      * in case of error during reading file from connected GridFS it return error and appropriate status code
*  if file with name `:filename` not exists in connected GridFS
      * return status code 404 Not Found

It also support `Range` header and serve requested chunks of file.

All events such as: start server, stopping server, connection to GridFS, http requests, http responses, errors
are logged in syslog with its own severity.

It accepts startup commandline options which are explained in `Running` section.

## System requirements

UNIX compatible os with installed golang compiler, check this link: [installation instructions](http://golang.org/doc/install)

MediaServer was developed and tested only for go version 1.1.

MediaServer also assume that environment variables was set in ~.profile or ~.bashrc

    export GOROOT=$HOME/go
    export PATH=$PATH:$GOROOT/bin
    export GOPATH=/usr/local/go

In order to satisfy dependencies it need to have VCS `bzr` installed

## Compilation

MediaServer have a few external dependencies so they are have to be installed before compilation:

    go get .

After sucesfull install of dependencies it can be compiled from directory with main source file:

    go build mserv.go

The result of compilation will be `mserv` executable file which can be moved to appropriate directory or start inplace.

    ./mserv # with command line configuration parameters, see below

## Running

    >./mserv -h
    Usage of ./mserv:
       -c=8: cpu used (max)
       -d="localhost:27017": url mongodb connection in format: myuser:mypass@localhost:40001,otherhost:40001/mydb
       -f="media": name for the mongodb file system
       -p=8080: port to listen on
       -u="": user to run as
       -v=false: print version information

#### Example of how to run media server

    >./mserv -d user:longPasSWorD@localhost:27017/media_files
       serving on localhost:8080
       utilizing 8 CPU
       Media server started!
