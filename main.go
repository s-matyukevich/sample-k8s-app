/**
# Copyright 2015 Google Inc. All rights reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
**/

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/http/httputil"

	"cloud.google.com/go/compute/metadata"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/mysql"
)

type Instance struct {
	Id         string
	Name       string
	Version    string
	Hostname   string
	Zone       string
	Project    string
	InternalIP string
	ExternalIP string
	LBRequest  string
	ClientIP   string
	Local      bool
	Notes      []string
	Error      string
}

type Note struct {
	gorm.Model

	Note string
}

const version string = "1.0.0"
const dbName = "sample_app"

func main() {
	showversion := flag.Bool("version", false, "display version")
	frontend := flag.Bool("frontend", false, "run in frontend mode")
	port := flag.Int("port", 8080, "port to bind")
	backend := flag.String("backend-service", "http://127.0.0.1:8081", "hostname of backend server")
	dbHost := flag.String("db-host", "db", "hostname of DB server")
	dbUser := flag.String("db-user", "root", "DB username")
	dbPassword := flag.String("db-password", "root", "DB password")
	flag.Parse()

	if *showversion {
		fmt.Printf("Version %s\n", version)
		return
	}

	http.HandleFunc("/version", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "%s\n", version)
	})

	if *frontend {
		frontendMode(*port, *backend)
	} else {
		db, err := gorm.Open("mysql", fmt.Sprintf("%s:%s@tcp(%s)/mysql", *dbUser, *dbPassword, *dbHost))
		if err != nil {
			log.Fatal(err)
		}
		err = db.Exec(fmt.Sprintf("CREATE DATABASE IF NOT EXISTS %s", dbName)).Error
		if err != nil {
			log.Fatal(err)
		}
		db, err = gorm.Open("mysql", fmt.Sprintf("%s:%s@tcp(%s)/%s?charset=utf8&parseTime=True&loc=Local", *dbUser, *dbPassword, *dbHost, dbName))
		if err != nil {
			log.Fatal(err)
		}
		err = db.AutoMigrate(&Note{}).Error
		if err != nil {
			log.Fatal(err)
		}
		defer db.Close()
		backendMode(*port, db)
	}

}

func backendMode(port int, db *gorm.DB) {
	log.Println("Operating in backend mode...")
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		i := newInstance(db)
		raw, _ := httputil.DumpRequest(r, true)
		i.LBRequest = string(raw)
		resp, _ := json.Marshal(i)
		fmt.Fprintf(w, "%s", resp)
	})
	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	http.HandleFunc("/add-note", func(w http.ResponseWriter, r *http.Request) {
		data, err := ioutil.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, "Error: %s\n", err.Error())
			return
		}
		var note Note
		err = json.Unmarshal(data, &note)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, "Error: %s\n", err.Error())
			return
		}
		err = db.Create(&note).Error
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, "Error: %s\n", err.Error())
			return
		}
		fmt.Fprintf(w, "Ok\n")

	})
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%v", port), nil))

}

func frontendMode(port int, backendURL string) {
	log.Println("Operating in frontend mode...")
	html, err := ioutil.ReadFile("/go/src/app/templates/base.html")
	if err != nil {
		log.Fatal(fmt.Sprintf("Can't read template: %s", err.Error()))
	}
	tpl := template.Must(template.New("out").Parse(string(html)))

	transport := http.Transport{DisableKeepAlives: false}
	client := &http.Client{Transport: &transport}
	req, _ := http.NewRequest(
		"GET",
		backendURL,
		nil,
	)
	req.Close = false

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		i := &Instance{}
		resp, err := client.Do(req)
		if err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			fmt.Fprintf(w, "Error: %s\n", err.Error())
			return
		}
		defer resp.Body.Close()
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, "Error: %s\n", err.Error())
			return
		}
		err = json.Unmarshal([]byte(body), i)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, "Error: %s\n", err.Error())
			return
		}
		tpl.Execute(w, i)
	})

	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		resp, err := client.Do(req)
		if err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			fmt.Fprintf(w, "Backend could not be connected to: %s", err.Error())
			return
		}
		defer resp.Body.Close()
		ioutil.ReadAll(resp.Body)
		w.WriteHeader(http.StatusOK)
	})

	http.HandleFunc("/add-note", func(w http.ResponseWriter, r *http.Request) {
		req, _ := http.NewRequest(
			"GET",
			backendURL+"/add-note",
			r.Body,
		)
		resp, err := client.Do(req)
		if err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			fmt.Fprintf(w, "Backend could not be connected to: %s", err.Error())
			return
		}
		defer resp.Body.Close()
		_, err = io.Copy(w, resp.Body)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, "Can't read backend response: %s", err.Error())
			return
		}
	})
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%v", port), nil))
}

type assigner struct {
	err error
}

func (a *assigner) assign(getVal func() (string, error)) string {
	if a.err != nil {
		return ""
	}
	s, err := getVal()
	if err != nil {
		a.err = err
	}
	return s
}

func GetOutboundIP() net.IP {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)

	return localAddr.IP
}

func newInstance(db *gorm.DB) *Instance {
	i := &Instance{}
	notes := []Note{}
	err := db.Find(&notes).Error
	if err != nil {
		i.Error = err.Error()
		return i
	}
	i.Notes = []string{}
	for _, note := range notes {
		i.Notes = append(i.Notes, note.Note)
	}
	if !metadata.OnGCE() {
		i.Local = true
		i.InternalIP = GetOutboundIP().String()
		return i
	}

	a := &assigner{}
	i.Id = a.assign(metadata.InstanceID)
	i.Zone = a.assign(metadata.Zone)
	i.Name = a.assign(metadata.InstanceName)
	i.Hostname = a.assign(metadata.Hostname)
	i.Project = a.assign(metadata.ProjectID)
	i.InternalIP = a.assign(metadata.InternalIP)
	i.ExternalIP = a.assign(metadata.ExternalIP)
	i.Version = version

	if a.err != nil {
		i.Error = a.err.Error()
	}
	return i
}
