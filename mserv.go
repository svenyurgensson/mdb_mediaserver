/*
curl -i -H "Range: bytes=0-3" http://localhost:9876/1c75e2bd-156c-3491-9bf0-409a4c238a4a.jpg
*/

package main

import (
	"errors"
	"flag"
	"fmt"
	"github.com/kylelemons/go-gypsy/yaml"
	"labix.org/v2/mgo"
	"log"
	"log/syslog"
	"net/http"
	"os"
	"os/signal"
	"os/user"
	"runtime"
	"time"
	"strconv"
	"strings"
	"syscall"
)

const Author = "Yury Batenko"
const Version = "1.8.1 / 2014-11-11"

var (
	srv_bin       string
	session       *mgo.Session
	db            *mgo.Database
	db_connect    string
	slog          *syslog.Writer
	http_port     int
	server_user   string
	cpu_cnt       int
	mongo_hosts   string
	mongo_user    string
	mongo_pass    string
	mongo_db      string
	mongo_fs_name string
	err           error
	ping_resp     string
	req_total     uint64
	req_failed    uint64
	boot_time     string
	hostname      string
)

type gridFSHandler struct {
	GFS      *mgo.GridFS
	PathFile string
}

func usage() {
	fmt.Fprintf(os.Stderr, "Usage: %s path/to/mserv.config.yaml\n", os.Args[0])
	flag.PrintDefaults()
	os.Exit(2)
}

func getConfig(filename string) {
	config, err := yaml.ReadFile(filename)
	if err != nil {
		log.Fatalf("readfile(%q): %s", filename, err)
	}

	http_port64, err := config.GetInt("port")
	if err != nil {
		http_port = 9876
	} else {
		http_port = int(http_port64)
	}

	server_user, err = config.Get("run_us")
	if err != nil {
		server_user = ""
	}

	cpu_cnt64, err := config.GetInt("cpu_use")
	if err != nil {
		cpu_cnt = runtime.NumCPU()
	} else {
		cpu_cnt = int(cpu_cnt64)
	}

	mh, err := yaml.Child(config.Root, "mongodb.hosts")
	if err != nil {
		fatal(errors.New("MongoDB hosts not defined!"))
	}
	list, _ := mh.(yaml.List)
	conn := []string{}
	for i := 0; i < list.Len(); i++ {
		conn = append(conn, strings.TrimSpace(yaml.Render(list.Item(i))))
	}
	mongo_hosts = strings.Join(conn, ",")

	mongo_user, err = config.Get("mongodb.user")
	mongo_pass, err = config.Get("mongodb.password")

	mongo_db, err = config.Get("mongodb.database")
	if err != nil {
		fatal(errors.New("MongoDB database not defined!"))
	}

	mongo_fs_name, err = config.Get("mongodb.fs")
	if err != nil {
		mongo_fs_name = "fs"
	}
}

func main() {

	empty := func(s string) bool { return len(s) == 0 }

	flag.Usage = usage
	flag.Parse()

	srv_bin = os.Args[0]

	args := flag.Args()
	if len(args) < 1 {
		fmt.Println("Config file is missing")
		version()
	}

	getConfig(args[0])

	if !empty(server_user) {
		setuid(server_user)
	}

	runtime.GOMAXPROCS(cpu_cnt)

	req_total = 0
	req_failed = 0

	slog, err = syslog.New(syslog.LOG_INFO|syslog.LOG_ERR|syslog.LOG_LOCAL0, "[mserv]")
	if err != nil {
		fatal(err)
	}

	srv_addr := fmt.Sprintf("localhost:%d", http_port)
	fmt.Printf("serving on %s\n", srv_addr)
	fmt.Printf("utilizing %d CPU\n", cpu_cnt)

	slog.Info(fmt.Sprintf("Mserv started and serving on: %s utilizing: %d CPU", srv_addr, cpu_cnt))

	if !empty(mongo_user) && !empty(mongo_pass) {
		db_connect = fmt.Sprintf("%s:%s@%s/%s", mongo_user, mongo_pass, mongo_hosts, mongo_db)
	} else {
		db_connect = fmt.Sprintf("%s/%s", mongo_hosts, mongo_db)
	}

	fmt.Println(db_connect)

	ConnectToMongo(db_connect)
	defer session.Close()

	http.Handle("/", GridFSServer(db.GridFS(mongo_fs_name), ""))
	http.HandleFunc("/ping", PingHandler)

	fmt.Println("Media server started!\n")

	boot_time = fmt.Sprintf(time.Now().Format(time.RFC3339))
	hostname, _  = os.Hostname()

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
		slog.Info(fmt.Sprintf("exit by SIGINT"))
		os.Exit(0)
	}()
	go func() {
		sigchane := make(chan os.Signal, 2)
		signal.Notify(sigchane, os.Kill, syscall.SIGKILL, syscall.SIGQUIT) // Not catched on MacOSX !
		<-sigchane
		slog.Err(fmt.Sprintf("exit by SIGKILL"))
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
	slog.Info(fmt.Sprintf("set owner of process: %s", username))
}

func version() {
	fmt.Printf("%s Go-GridFS server; author: %s version: %s\n", srv_bin, Author, Version)
	os.Exit(0)
}

//========================================================================

func (g *gridFSHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	req_total += 1
	filename := r.URL.Path[1:]
	slog.Info(fmt.Sprintf("%s %s %s", r.RemoteAddr, r.Method, r.URL))

	file, err := g.GFS.Open(filename)
	if err != nil {
		req_failed += 1
		http.NotFound(w, r)
		slog.Warning(fmt.Sprintf("requested: %s response: 404 Not Exists", filename))
		return
	}

	defer file.Close()

	etag := file.MD5()
	if inm := r.Header.Get("If-None-Match"); inm != "" {
		if inm == etag || inm == "*" {
				h := w.Header()
				delete(h, "Content-Type")
				delete(h, "Content-Length")
				w.WriteHeader(http.StatusNotModified)
				return
			}
	}

	w.Header().Set("Cache-Control", "max-age=86400, public, must-revalidate, proxy-revalidate")
	w.Header().Set("ETag", etag)
	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("Content-Type", file.ContentType())

	// For some reason SOMETIMES we need to seek to the end of file
	// otherwise we get: seeker can't seek error
	_, err = file.Seek(0, os.SEEK_END)

	slog.Info(fmt.Sprintf("Starting upload of %s", filename))
	http.ServeContent(w, r, file.Name(), file.UploadDate(), file)
	slog.Info(fmt.Sprintf("Finished upload of %s", filename))
}

func PingHandler(w http.ResponseWriter, r *http.Request) {
	slog.Info(fmt.Sprintf("ping request"))
	stat := mgo.GetStats()
	e := session.Ping()
	connected := true
	if e != nil { connected = false }
	ping_resp =
`[status]
status=OK

[server]
version=%s
boot_timestamp=%s
hostname=%s

[mongodb]
connected=%t
MasterConns=%d
SlaveConns=%d
SentOps=%d
ReceivedOps=%d
ReceivedDocs=%d
SocketsAlive=%d
SocketsInUse=%d
SocketRefs=%d

[requests]
total=%d
failed=%d`
	str := fmt.Sprintf(ping_resp, Version, boot_time, hostname, connected, stat.MasterConns, stat.SlaveConns, stat.SentOps, stat.ReceivedOps, stat.ReceivedDocs, stat.SocketsAlive, stat.SocketsInUse, stat.SocketRefs, req_total, req_failed)

	w.Header().Set("Content-Type", "text/plain")
	fmt.Fprint(w, str)
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
	// session.SetMode(mgo.Strong, true)
	session.SetMode(mgo.Eventual, true)
	db = session.DB("") // db will be taken from connection url

	mgo.SetStats(true)
	slog.Info(fmt.Sprintf("Connected to mongodb"))
}
