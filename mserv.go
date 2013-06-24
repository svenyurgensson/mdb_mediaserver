package main

import (
        "labix.org/v2/mgo"
        "flag"
        "fmt"
        "log"
        "log/syslog"
        "net/http"
        "os"
        "os/user"
        "runtime"
        "strconv"
        "syscall"
        "os/signal"
)

const   Author  = "Yury Batenko"
const   Version = "1.2"

var  srv_bin string

/*
  database = Mongo::Connection.new('54.235.213.159', '27017').db('classic')
  database.authenticate('etv_import', 'asuh987237GSdzuffgzue3w26')
  Mongo::GridFileSystem.new(database, 'media')

 ./mserv -d etv_import:asuh987237GSdzuffgzue3w26@54.235.213.159:27017/classic
*/


var (
        session *mgo.Session
        db      *mgo.Database
        slog    *syslog.Writer
        err      error
)

type gridFSHandler struct {
        GFS      *mgo.GridFS
        PathFile string
}

func main() {

        var (
                srv_port   int
                cpu_cnt    int
        )

        empty := func(s string) bool {
                return len(s) == 0
        }

        fPort    := flag.Int("p", 8080, "port to listen on")
        fUser    := flag.String("u", "", "user to run as")
        fVersion := flag.Bool("v", false, "print version information")
        fCpu     := flag.Int("c", runtime.NumCPU(), "cpu used (max)")
        fDb      := flag.String("d", "localhost:27017",
                "url mongodb connection in format: myuser:mypass@localhost:40001,otherhost:40001/mydb")
        fFSname  :=  flag.String("f", "media", "name for the mongodb file system")

        flag.Parse()

        if *fVersion { version() }

        if !empty(*fUser) { setuid(*fUser) }

        cpu_cnt = *fCpu
        runtime.GOMAXPROCS(cpu_cnt)

        slog, err = syslog.New(syslog.LOG_INFO | syslog.LOG_ERR, "[mserv]")
        if err != nil { fatal(err) }

        srv_port = *fPort
        srv_addr := fmt.Sprintf("localhost:%d", srv_port)
        fmt.Printf("serving on %s\n", srv_addr)
        fmt.Printf("utilizing %d CPU\n", cpu_cnt)

        slog.Err( fmt.Sprintf("Mserv started and serving on: %s utilize: %d CPU", srv_addr, cpu_cnt))

        ConnectToMongo(*fDb)
        defer session.Close()

        http.Handle("/", GridFSServer(db.GridFS(*fFSname), ""))
        http.HandleFunc("/ping",  PingHandler)

        fmt.Println("Media server started!\n")

        setSignalCatchers()

        s := &http.Server{
                Addr:         srv_addr,
                ReadTimeout:  100000,
                WriteTimeout: 0}
        log.Fatal(s.ListenAndServe())
}


//-------- Utilites ------------------

func setSignalCatchers() {
        go func() {
                sigchan := make(chan os.Signal, 2)
                signal.Notify(sigchan, os.Interrupt)
                <-sigchan
                slog.Info( fmt.Sprintf("exit by SIGINT"))
                os.Exit(0)
        }()
        go func() {
                sigchane := make(chan os.Signal, 2)
                signal.Notify(sigchane, os.Kill, syscall.SIGKILL, syscall.SIGQUIT) // Not catched on MacOSX !
                <-sigchane
                slog.Err( fmt.Sprintf("exit by SIGKILL"))
                os.Exit(-1)
        }()
}


func fatal(err error) {
        fmt.Printf("[!] %s: %s\n", srv_bin, err.Error())
        if slog != nil {
                slog.Err(fmt.Sprintf("Fatal error: %s: %s", srv_bin, err.Error()))
        }
        os.Exit(1)
}

func checkFatal(err error) {
        if err != nil {
                fatal(err)
        }
}

func setuid(username string) {
        usr, err := user.Lookup(username)
        checkFatal(err)
        uid, err := strconv.Atoi(usr.Uid)
        checkFatal(err)
        err = syscall.Setreuid(uid, uid)
        checkFatal(err)
        slog.Info( fmt.Sprintf("set owner of process: %s", username))
}

func version() {
        fmt.Printf("%s Go-gridfs server author: %s version: %s\n", srv_bin, Author, Version)
        os.Exit(0)
}

//========================================================================

func (g *gridFSHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
        filename := r.URL.Path[1:]

        file, err := g.GFS.Open(filename)
        if err != nil {
                http.NotFound(w, r)
                slog.Warning( fmt.Sprintf("requested: %s response: 404 Not Exists", filename))
                return
        }

        slog.Info( fmt.Sprintf("requested: %s", filename))

        defer file.Close()

        w.Header().Set("Accept-Ranges", "bytes")
        w.Header().Set("Content-Type", file.ContentType())

        http.ServeContent(w, r, file.Name(), file.UploadDate(), file)
}

func PingHandler(w http.ResponseWriter, r *http.Request) {
        slog.Info( fmt.Sprintf("ping request"))
        fmt.Fprint(w, "OK")
}

// Handle server requests, find file and response.
func GridFSServer(gfs *mgo.GridFS, pathFile string) http.Handler {
        return &gridFSHandler{gfs, pathFile}
}


func ConnectToMongo(connection string) {
        var err error

        session, err = mgo.Dial(connection)
        if err != nil {
                log.Fatalf("Error connecting to MongoDB '%s' %v", connection, err.Error())
        }
        // session.SetMode(mgo.Monotonic, true)
        session.SetMode(mgo.Strong, true)
        db = session.DB("") // db will be taken from connection url

        slog.Info(fmt.Sprintf("Connected to mongodb"))
}
