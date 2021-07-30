package main

/*
读取config.yaml 文件
对yaml里面的site进行http检查
huangmingyou@gmail.com
2021.07
*/

import (
"bytes"
	"crypto/tls"
        "crypto/x509"
	"flag"
	"fmt"
"text/template"
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
	Name    string        `yaml:"name"`
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
func infoGet2(t U) {
returnInfo := make([]hostinfo, 0, 1)
        info := hostinfo{Host: t.Name, Port: 443}
        err := info.getCerts(5*time.Second)
               if err == nil {
                        returnInfo = append(returnInfo, info)
fmt.Println("add 1")
                }else{
		fmt.Println(err)
fmt.Println("get error")
}


                tt := template.Must(template.New("").Parse(`
{{- range . -}}
Host: {{ .Host }}:{{ .Port }}
Certs:
    {{ range .Certs -}}
    Issuer: {{ .Issuer.CommonName }}
    Subject: {{ .Subject.CommonName }}
    Not Before: {{ .NotBefore.Format "Jan 2, 2006 3:04 PM" }}
    Not After: {{ .NotAfter.Format "Jan 2, 2006 3:04 PM" }}
    DNS names: {{ range .DNSNames }}{{ . }} {{ end }}
{{ end }}
{{ end -}}
        `))
fmt.Println(t.Name)
                err = tt.Execute(os.Stdout, &returnInfo)
fmt.Println("end")

}

func infoGet(t U, c chan string) {
returnInfo := make([]hostinfo, 0, 1)
        info := hostinfo{Host: t.Name, Port: 443}
        err := info.getCerts(5*time.Second)
	if err != nil {
		fmt.Println(err)
	}else{
                        returnInfo = append(returnInfo, info)
}

                tt := template.Must(template.New("").Parse(`
{{- range . -}}
ssl_ttl{target="{{ .Host }}"}  {{ range .Certs -}}  {{ .NotAfter.Format "20060102"}} {{ end }}
{{ end -}}
        `))
buffer := new(bytes.Buffer)
                //err = tt.Execute(os.Stdout, &info)
                err = tt.Execute(buffer, &returnInfo)

st1 := buffer.String()
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
