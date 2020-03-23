package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

var DEBUG bool
var COMMIT_LOG bool
var WP_VERSION string

func loadFile(t string) map[string]interface{} {
	fname := t + ".json"
	log.Print("Loading " + fname)
	file, _ := os.OpenFile(fname, os.O_CREATE|os.O_RDONLY, 0644)
	defer file.Close()
	m := make(map[string]interface{})
	dat, _ := ioutil.ReadAll(file)
	if json.Unmarshal(dat, &m) != nil {
		m = make(map[string]interface{})
		log.Printf("Failed to parse %s", fname)
	}
	log.Printf("Loaded %s", fname)
	return m
}

func writeLog(t string, po, pn map[string]interface{}) {
	file, _ := os.OpenFile(t+"-new.log", os.O_CREATE|os.O_WRONLY, 0644)
	defer file.Close()
	log.Printf("Writing %s-new.log", t)
	for name, p := range pn {
		p := p.(map[string]interface{})
		newVersion := p["version"].(string)
		newRev := p["rev"].(string)
		// find what's in pn but not in po
		if po[name] == nil {
			file.WriteString(fmt.Sprintf("ADD %s %s (%s)\n", name, newVersion, newRev))
		} else if po[name] != nil {
			pOld := po[name].(map[string]interface{})
			oldVersion := pOld["version"].(string)
			oldRev := pOld["rev"].(string)
			if oldRev != newRev {
				file.WriteString(fmt.Sprintf("UPD %s %s (%s) -> %s (%s)\n", name, oldVersion, oldRev, newVersion, newRev))
			}
		}
	}
	log.Printf("Replacing %s.log with %s-new.log", t, t)
	os.Rename(t+"-new.log", t+".log")
}

func writeFile(t string, c map[string]interface{}) {
	file, _ := os.OpenFile(t+"-new.json", os.O_CREATE|os.O_WRONLY, 0644)
	defer file.Close()
	enc := json.NewEncoder(file)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	log.Printf("Writing %s-new.json", t)
	enc.Encode(c)
	log.Printf("Replacing %s.json with %s-new.json", t, t)
	os.Rename(t+"-new.json", t+".json")
}

func execCmd(name, dir, file string, args ...string) (string, error) {
	log.Printf("%s %s %s", name, file, args)
	cmd := exec.Command(file, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("Failed: %s %s %s %s", err, name, file, args)
	}
	return strings.TrimSpace(string(out)), err
}

func svnPrefetch(repo *Repository, path string, rev string, rawName string) (string, error) {
	// people push some weird shit
	reg, _ := regexp.Compile("[^a-zA-Z0-9+\\-._?=]+")
	fixedName := reg.ReplaceAllString(rawName, "")
	dir, _ := ioutil.TempDir("", "wp4nix-prefetch-")
	defer os.RemoveAll(dir)
	var err error
	var resp string
	err = repo.Export(path, rev, dir+"/"+fixedName, nil, nil)
	if err == nil {
		resp, err = execCmd("Hash", dir, "nix-hash", "--type", "sha256", "--base32", fixedName)
	}
	return resp, err
}

// copy every element from every map into the resulting map
// meaning, merge all maps with the later maps having precedence over the previous one(s)
func mergePs(ps ...map[string]interface{}) map[string]interface{} {
	res := make(map[string]interface{})
	for _, m := range ps {
		for k, v := range m {
			res[k] = v
		}
	}
	return res
}

func buildPkgQueueWorker(jobs <-chan map[string]interface{}, results chan<- map[string]interface{}, exited chan<- bool, repo *Repository, t string) {
	for {
		p := <-jobs
		if p == nil {
			log.Printf("buildPkgQueueWorker: jobs queue empty, exiting")
			break
		}
		if p["kind"].(string) != "dir" {
			continue
		}
		name := p["name"].(string)
		var version string
		var err error
		if t != "languages" {
			// query api for current version
			var resp *http.Response
			if t == "plugins" {
				url := "https://api.wordpress.org/plugins/info/1.0/" + name + ".json"
				resp, err = http.Get(url)
			} else if t == "themes" {
				resp, err = http.Get("https://api.wordpress.org/themes/info/1.2/?action=theme_information&request[slug]=" + name)
			}
			if err != nil {
				log.Printf("API request failed for %s %s", t, name)
				continue
			}
			defer resp.Body.Close()
			var resp_json map[string]interface{}
			json.NewDecoder(resp.Body).Decode(&resp_json)
			if resp_json["error"] != nil {
				// don't log "not found"s, because thats 20% of all plugins and themes
				if !strings.Contains(resp_json["error"].(string), "not found") {
					log.Printf("API request returned error %s for %s %s", resp_json["error"], t, name)
				}
				continue
			}

			switch v := resp_json["version"].(type) {
			case float64:
				version = strconv.FormatFloat(v, 'f', -1, 64)
			case string:
				version = v
			default:
				log.Printf("version of %s %s was neither string nor float64, defaulting to B\n", t, name)
				continue
			}
		} else {
			version = WP_VERSION
		}
		var path string
		if t == "plugins" {
			path = name + "/tags/" + version
		} else if t == "themes" {
			path = name + "/" + version
		} else if t == "languages" {
			path = name
		}
		var entry *Entry
		entry, err = repo.Info(path, nil)
		if err != nil && strings.Contains(err.Error(), "W170000") && strings.Contains(err.Error(), "E200009") {
			path = name + "/trunk"
			entry, err = repo.Info(path, nil)
		} else if err != nil {
			log.Printf("Unexpected error occured in svn info: %s", err.Error())
			continue
		}
		if entry == nil {
			log.Printf("Something went very wrong with running info for %s", path)
			continue
		}
		results <- map[string]interface{}{
			"name":    name,
			"path":    path,
			"rev":     entry.Commit.Revision,
			"version": version,
		}
		// log.Printf("Put %s, %s, %s in queue", name, currentVersion, rev)
	}
	exited <- true
}

type Worker func(jobs <-chan map[string]interface{}, results chan<- map[string]interface{}, exited chan<- bool, repo *Repository, t string)

func startWorkers(worker Worker, jobs chan map[string]interface{}, results chan<- map[string]interface{}, repo *Repository, t string) {
	numWorkers := 32
	if DEBUG {
		numWorkers = 1
	}

	exited := make(chan (bool))
	for i := 0; i < numWorkers; i++ {
		go worker(jobs, results, exited, repo, t)
	}
	// wait for all workers to return and close results channel afterwards
	for i := 0; i < numWorkers; i++ {
		<-exited
		log.Printf("A worker just exited, %d worker(s) remaining", numWorkers-i-1)
	}
	close(exited)
	close(results)
}

func buildPkgQueue(pkgs []Entry, repo *Repository, results chan<- map[string]interface{}, t string) {
	log.Printf("Starting to build %s queue", t)
	jobs := make(chan map[string]interface{})
	go startWorkers(buildPkgQueueWorker, jobs, results, repo, t)

	for _, e := range pkgs {
		jobs <- map[string]interface{}{"kind": e.Kind, "name": e.Name}
	}
	close(jobs)
	log.Printf("Finished building %s queue", t)
}

func processPkgQueueWorker(jobs <-chan map[string]interface{}, results chan<- map[string]interface{}, exited chan<- bool, repo *Repository, t string) {
	for {
		p := <-jobs
		if p == nil {
			break
		}
		pn := p["n"].(map[string]interface{})
		var po = make(map[string]interface{})
		iRevOld := 0
		sRevOld := "0"
		if p["o"] != nil {
			po = p["o"].(map[string]interface{})
			if po["rev"] != nil {
				sRevOld = po["rev"].(string)
				iRevOld, _ = strconv.Atoi(sRevOld)
			}
		}
		name := pn["name"].(string)
		version := pn["version"].(string)
		path := pn["path"].(string)
		sRevNew := pn["rev"].(string)
		iRevNew, _ := strconv.Atoi(sRevNew)
		if iRevNew > iRevOld {
			var err error
			pn["sha256"], err = svnPrefetch(repo, path, sRevNew, name+"-"+version)
			if err != nil {
				continue
			}
			results <- pn
			log.Printf("Processed %s %s %s", name, version, sRevNew)
		} else {
			log.Printf("Skipping %s %s because %d >= %d", name, version, iRevOld, iRevNew)
		}
	}
	exited <- true
}

func submitProcessPkgQueueJobs(queue <-chan map[string]interface{}, jobs chan<- map[string]interface{}, po map[string]interface{}) {
	for {
		j, more := <-queue
		if !more {
			break
		}
		jn := map[string]interface{}{
			"n": j,
			"o": po[j["name"].(string)],
		}
		jobs <- jn
	}
	close(jobs)
}

func processPkgQueue(queue <-chan map[string]interface{}, po map[string]interface{}, t string, repo *Repository) {
	log.Printf("Starting to process %s queue", t)
	pn := make(map[string]interface{})
	jobs := make(chan map[string]interface{})
	results := make(chan map[string]interface{})

	go startWorkers(processPkgQueueWorker, jobs, results, repo, t)
	go submitProcessPkgQueueJobs(queue, jobs, po)
	for i := 1; ; i++ {
		x, more := <-results
		if !more {
			break
		}
		n := x["name"].(string)
		delete(x, "name")
		pn[n] = x
		mod := 50
		if DEBUG {
			mod = 10
		}
		if i%mod == 0 {
			go writeFile(t, mergePs(po, pn))
			go writeLog(t, po, pn)
		}
	}

	log.Printf("Finished processing %s queue", t)
	writeLog(t, po, pn)
	writeFile(t, mergePs(po, pn))
}

func processType(t string) {
	po := loadFile(t)
	log.Printf("Starting to process %s", t)
	subdomain := t
	directory := ""
	if t == "languages" {
		subdomain = "i18n"
		directory = "/core/" + WP_VERSION
	}
	repo := NewRepository("https://" + subdomain + ".svn.wordpress.org" + directory)
	pkgs, _ := repo.List("", nil)
	log.Printf("Got list of %d %s", len(pkgs), t)
	if DEBUG {
		log.Printf("Only processing first and last 50 %s, because we are in debug mode.", t)
		if len(pkgs) > 100 {
			pkgs = append(pkgs[:50], pkgs[len(pkgs)-50:]...)
		}
	}
	queue := make(chan map[string]interface{})

	go buildPkgQueue(pkgs, repo, queue, t)
	processPkgQueue(queue, po, t, repo)
	log.Printf("Finished processing %s", t)
}

func main() {
	_, DEBUG = os.LookupEnv("DEBUG")
	_, COMMIT_LOG = os.LookupEnv("COMMIT_LOG")
	var isSet bool
	WP_VERSION, isSet = os.LookupEnv("WP_VERSION")
	if !isSet {
		log.Fatal("WP_VERSION needs to be set to the wordpress release you want to fetch languages for")
	} else {
		split := strings.Split(WP_VERSION, ".")
		WP_VERSION = fmt.Sprintf("%s.%s", split[0], split[1])
	}
	processType("languages")
	processType("themes")
	processType("plugins")
}
