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

## Development and deployment

You need installed and fully worked Go compiler.

In order to deploy you need have ruby installed and mina gem

    gem i mina

To compile mserv from source you need:

    make

After sucesfull compilation directory build/ will contain linux and osx executables

To deploy you need to run:

    mina tasks

And read output


## System requirements

Local development: Go compiler with installed cross-compile packages.

Target server: linux 64bit

It need to have /etc/mserv directory with config file:

    mserv.config.yaml

It is a configuration file which mserv use to set port, mongodb connections and so on

    port: 9876
    run_us:
    cpu_use: 4
    mongodb:
        user: etv_import
        password: PaSSword
        hosts:
            - 54.235.213.159:27017
            - 54.235.213.160:27017
        database: classic
        fs: media

All options are self-explained by their names

## Running

    > ./mserv # with command line configuration parameters, see below

    > ./mserv path/to/mserv.config.yaml


#### Example of how to run media server

    > ./mserv mserv.config.yaml
       serving on localhost:8080
       utilizing 8 CPU
       Media server started!
