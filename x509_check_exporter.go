package main

/*
读取config.yaml 文件
对yaml里面的site进行http检查
huangmingyou@gmail.com
2021.07
*/

import (
	//"bytes"
	"crypto/tls"
	"crypto/x509"
	"flag"
	"fmt"
	"github.com/robfig/cron/v3"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"time"
)

var cfgfile string
var metrics string

type hostinfo struct {
	Host  string
	Port  int
	Certs []*x509.Certificate
}

type U struct {
	Name string `yaml:"name"`
}

type C struct {
	Thread     int    `yaml:"thread"`
	Updatecron string `yaml:"updatecron"`
	Targets    []U    `yaml:",flow"`
}

var yc C

func ValidateConfigPath(path string) error {
	s, err := os.Stat(path)
	if err != nil {
		return err
	}
	if s.IsDir() {
		return fmt.Errorf("'%s' is a directory, not a normal file", path)
	}
	return nil
}

func ParseFlags() (string, string, error) {
	var configPath string
	var mode string

	flag.StringVar(&configPath, "config", "./config.yml", "path to config file")
	flag.StringVar(&mode, "mode", "cli", "run mode, cli or web")

	flag.Parse()
	if err := ValidateConfigPath(configPath); err != nil {
		return "", "", err
	}
	return configPath, mode, nil
}

func infoGet(t U, c chan string) {
	returnInfo := make([]hostinfo, 0, 1)
	info := hostinfo{Host: t.Name, Port: 443}
	err := info.getCerts(5 * time.Second)
	if err != nil {
		fmt.Println(t.Name,err)
		c <- ""
		return
	} else {
		returnInfo = append(returnInfo, info)
	}

	now := time.Now()
	subM := info.Certs[0].NotAfter.Sub(now)
	//fmt.Printf("cert_ttl{dns=\"%s\"}  %d\n",t.Name,int(subM.Hours()/24))
	st1 := fmt.Sprintf("cert_liveday{name=\"%s\",dns=\"%s\"}  %d\n", t.Name, info.Certs[0].DNSNames[0],int(subM.Hours()/24))
	//fmt.Println("debug",info.Certs[0].DNSNames)
	c <- st1
}

func Exporter(w http.ResponseWriter, r *http.Request) {
	ch1 := make(chan string)
	res2 := ""
	for i := 0; i < len(yc.Targets); i++ {
		go infoGet(yc.Targets[i], ch1)
		res2 += <-ch1
	}
	fmt.Fprintf(w, res2)
}

func runcli() {
	ch1 := make(chan string)
	res2 := ""
	for i := 0; i < len(yc.Targets); i++ {
		go infoGet(yc.Targets[i], ch1)
		res2 += <-ch1
		//infoGet2(yc.Targets[i])
	}

	metrics = res2
	//	fmt.Println(time.Now())
	fmt.Println(res2)
}

// host function

func (h *hostinfo) getCerts(timeout time.Duration) error {
	//log.Printf("connecting to %s:%d", h.Host, h.Port)
	dialer := &net.Dialer{Timeout: timeout}
	conn, err := tls.DialWithDialer(
		dialer,
		"tcp",
		h.Host+":"+strconv.Itoa(h.Port),
		&tls.Config{
			InsecureSkipVerify: true,
		})
	if err != nil {
		return err
	}

	defer conn.Close()

	if err := conn.Handshake(); err != nil {
		return err
	}

	pc := conn.ConnectionState().PeerCertificates
	h.Certs = make([]*x509.Certificate, 0, len(pc))
	for _, cert := range pc {
		if cert.IsCA {
			continue
		}
		h.Certs = append(h.Certs, cert)
	}

	return nil
}

func main() {
	cfgPath, runmode, err := ParseFlags()
	if err != nil {
		log.Fatal(err)
	}
	content, err := ioutil.ReadFile(cfgPath)
	err1 := yaml.Unmarshal(content, &yc)
	if err1 != nil {
		log.Fatalf("error: %v", err1)
	}
	cfgfile = cfgPath
	// cron job
	cjob := cron.New()
	cjob.AddFunc(yc.Updatecron, runcli)
	cjob.Start()
	//
	if runmode == "web" {
		//init data
		runcli()
		//
		http.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprintf(w, metrics)
		})
		log.Fatal(http.ListenAndServe(":8080", nil))
	} else {
		runcli()
	}

}
