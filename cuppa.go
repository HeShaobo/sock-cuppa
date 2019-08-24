package main

import (
    "fmt"
    "log"
    "flag"
    "os"
    "io/ioutil"
    "encoding/json"
    "net"
    "time"
    "os/signal"
    "syscall"
    "net/url"
    "net/http"
    "io"
    "bufio"
)

const (
    CONNECT_TIMEOUT = 0
    CONNECT_FAIL    = 1
)

const VERSION = "v0.1"

type Config struct {
    RemoteAddr      string  `json:"remote_addr"`
    LocalPort       int     `json:"local_port"`
    EnableReport   bool    `json:"enable_report"`
    MonitorInterval int     `json:"monitor_interval"`
    MonitorTimeout  int     `json:"monitor_timeout"`
    ReportInterval  int     `json:"report_interval"`
    ReportThreshold int     `json:"report_threshold"`
    LogPath         string  `json:"log_path"`
    PushUrl         string  `json:"push_url"`
}

func (c *Config)LoadConfig(path string) (err error) {
    file, err := os.Open(path)
	if err != nil {
		return
    }
    defer file.Close()
    data, err := ioutil.ReadAll(file)
	if err != nil {
		return
	}
	if err = json.Unmarshal(data, c); err != nil {
		return
	}
	return nil
}

func (c *Config)LoadLogFile() (file *os.File, err error) {
    file, err = os.OpenFile(c.LogPath, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0755)
    if err != nil {
        return
    }
    return file, nil
}

type MonitorResult struct {
    IsSuccess   bool
    FailReason  int
    Err         error
    Cost        int64
}

type Server struct {
    config *Config
    listener net.Listener
    errCh chan error
    monitorCh chan *MonitorResult
    monitorResultList []*MonitorResult
}

func (s *Server)Start() {
    listener, err := net.Listen("tcp", fmt.Sprintf(":%d", s.config.LocalPort))
    if err != nil {
        debug.Printf(err.Error())
        os.Exit(1)
    }
    s.listener = listener
    s.monitorCh = make(chan *MonitorResult)
    s.monitorResultList = make([]*MonitorResult, 0)
    go s.monitor()
    for {
        conn, err := listener.Accept()
        if err != nil {
            debug.Printf("accept a connection fail: %s", err)
        }
        debug.Printf("receive new client connect: %s", conn.RemoteAddr().String())
        go s.handle(conn)
    }
}

func (s *Server)handle(conn net.Conn) {
    // set tcp keepalive
    if tconn, ok := conn.(*net.TCPConn); ok {
        tconn.SetKeepAlive(true)
        tconn.SetKeepAlivePeriod(time.Duration(120 * time.Second))
    }
    request, err := net.Dial("tcp", s.config.RemoteAddr)
    if err != nil {
        debug.Printf("request remote addr fail: %s", err)
        return
    }
    if tconn, ok := request.(*net.TCPConn); ok {
        tconn.SetKeepAlive(true)
        tconn.SetKeepAlivePeriod(time.Duration(120 * time.Second))
    }
    remoteAddr := request.RemoteAddr()
    clientAddr := conn.RemoteAddr()
    defer func() {
        if _, ok := conn.(net.Conn); ok {
            debug.Printf("client connect close[clientaddr %s]", clientAddr)
            conn.Close()
        }
        if _, ok := request.(net.Conn); ok {
            debug.Printf("remote connect close[remoteaddr %s]", remoteAddr)
            request.Close()
        }
    }()
	inbuff := bufio.NewReader(conn)
    outbuff := bufio.NewReader(request)
    errCh := make(chan error, 2)
	go s.proxy(request, inbuff, errCh)
	go s.proxy(conn, outbuff, errCh)
	for i := 0; i < 2; i++ {
		e := <-errCh
		if e != nil {
			debug.Printf("tcp tunnel get an error: %s", e)
			return
		}
	}
	return
}

func (s *Server)proxy(dst io.Writer, src io.Reader, errCh chan error) {
    _, err := io.Copy(dst, src)
	errCh <- err
}

func (s *Server)monitor() {
    checkDuration := time.Duration(time.Second * time.Duration(s.config.MonitorInterval))
    checkTick := time.NewTicker(checkDuration)
    reportDuration := time.Duration(time.Second * time.Duration(s.config.ReportInterval))
    reportTick := time.NewTicker(reportDuration)
    for{
        select {
        case <- checkTick.C :
            go s.check()
        case <- reportTick.C :
            go s.report()
        case result := <- s.monitorCh :
            s.statistics(result)
        }
    }
}

func (s *Server)report() {
    len := len(s.monitorResultList)
    fails := 0
    for _, result := range s.monitorResultList {
        if ! result.IsSuccess {
            fails++
        }
    }
    s.monitorResultList = make([]*MonitorResult, 0)
    rate := (float32(fails) / float32(len)) * 100
    if rate >= float32(s.config.ReportThreshold) {
        text := fmt.Sprintf("Cuppa探测报告(最近%d分钟失败率%.2f%%)", s.config.ReportInterval/60, rate)
        debug.Printf(text)
        if s.config.EnableReport {
            u := s.config.PushUrl + "?text=" + url.QueryEscape(text)
            _, err := http.Get(u)
            if err != nil {
                debug.Printf("push report error: %s", err)
            }
        }
    }
}

func (s *Server)check() {
    startTime := time.Now()
    conn, err := net.DialTimeout("tcp", s.config.RemoteAddr, time.Duration(s.config.MonitorTimeout)*time.Second)
    endTime := time.Now()
    duration := endTime.Sub(startTime).Nanoseconds() / 1000000
    defer func(conn net.Conn) {
        if _, ok := conn.(net.Conn); ok {
            conn.Close()
        }
    }(conn)
    result := &MonitorResult{}
    result.Cost = duration
    result.Err = err
    result.IsSuccess = true
    if err != nil {
        result.IsSuccess = false
        if nerr, ok := err.(net.Error); ok && nerr.Timeout() {
            result.FailReason = CONNECT_TIMEOUT
        } else {
            result.FailReason = CONNECT_FAIL
        }
    }
    s.monitorCh <- result
}

func (s *Server)statistics(result *MonitorResult) {
    if ! result.IsSuccess {
        debug.Printf("check remote fail; cost: %dms; reason: %s", result.Cost, result.Err)
    }
    s.monitorResultList = append(s.monitorResultList, result)
}

var configFile string
var debug *log.Logger

func main() {
    config := &Config{}
    var version bool

    flag.StringVar(&configFile, "c", "config.json", "specify config file")
    flag.BoolVar(&version, "v", false, "print version")
    flag.Parse()

    if version {
        fmt.Printf("Cuppa Version: %s\n", VERSION)
        os.Exit(0)
    }

    if err := config.LoadConfig(configFile); err != nil {
        fmt.Errorf(err.Error())
        os.Exit(1)
    }

    logFile, err := config.LoadLogFile()
    if err != nil {
        fmt.Errorf(err.Error())
        os.Exit(1)
    }

    if config.MonitorInterval <= 0 {
        fmt.Errorf("config.json: monitor_interval must > 0 and must be an int")
    }
    if config.MonitorTimeout <= 0 {
        fmt.Errorf("config.json: monitor_timeout must > 0 and must be an int")
    }
    debug = log.New(logFile, "[Debug]", log.LstdFlags)

    server := Server{}
    server.config = config
    go server.Start()

    var sigCh = make(chan os.Signal, 1)
    signal.Notify(sigCh, syscall.SIGINT)
    <- sigCh
    debug.Printf("cuppa server exit")
    os.Exit(0)
}