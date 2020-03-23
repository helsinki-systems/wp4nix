// based on https://gitlab.com/jhinrichsen/svn/

package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"
)

// List represents XML output of an `svn list` subcommand
type ListElement struct {
	XMLName xml.Name `xml:"lists"`
	Entries []Entry  `xml:"list>entry"`
}

// List represents XML output of an `svn info` subcommand
type InfoElement struct {
	XMLName xml.Name `xml:"info"`
	Entry   Entry    `xml:"entry"`
}

// Entry represents XML output of an `svn list` or `svn info` subcommand
type Entry struct {
	Kind   string `xml:"kind,attr"`
	Name   string `xml:"name"`
	Commit Commit `xml:"commit"`
}

// Commit represents XML output of an `svn list` subcommand
type Commit struct {
	Revision string    `xml:"revision,attr"`
	Author   string    `xml:"author"`
	Date     time.Time `xml:"date"`
}

// Repository holds information about a (possibly remote) repository
type Repository struct {
	Location string
}

// NewRepository will initialize the internal structure of a possible remote
// repository, usually pointing to the parent of the default trunk/ tags/ branches
// structure.
func NewRepository(l string) *Repository {
	return &Repository{
		Location: l,
	}
}

// FullPath returns the full path into a repository
func (a *Repository) FullPath(relpath string) string {
	return fmt.Sprintf("%s/%s", a.Location, relpath)
}

// List will execute an `svn list` subcommand.
// Any non-nil xmlWriter will receive the XML content
func (a *Repository) List(relpath string, w io.Writer) ([]Entry, error) {
	log.Printf("listing %s\n", a.FullPath(relpath))
	fp := a.FullPath(relpath)
	cmd := exec.Command("svn", "list", "--xml", fp)
	// log.Printf("executing %+v\n", cmd)
	buf, err := cmd.CombinedOutput()
	if w != nil {
		io.Copy(w, bytes.NewReader(buf))
	}
	if err != nil {
		fmt.Fprintf(os.Stdout, "%s", buf)
		return nil, fmt.Errorf("Cannot list %s: %s", fp, err)
	}
	var l ListElement
	if err := xml.Unmarshal(buf, &l); err != nil {
		return nil, fmt.Errorf("cannot parse XML: %s: %s", buf, err)
	}
	return l.Entries, nil
}

// List will execute an `svn info` subcommand.
// Any non-nil xmlWriter will receive the XML content
func (a *Repository) Info(relpath string, w io.Writer) (*Entry, error) {
	log.Printf("info %s\n", a.FullPath(relpath))
	fp := a.FullPath(relpath)
	cmd := exec.Command("svn", "info", "--xml", fp)
	cmd.Dir = "/"
	// log.Printf("executing %+v\n", cmd)
	buf, err := cmd.CombinedOutput()
	if w != nil {
		io.Copy(w, bytes.NewReader(buf))
	}
	if err != nil {
		// fmt.Fprintf(os.Stdout, "%s", buf)
		return nil, fmt.Errorf("Cannot list %s: %s\n%s", fp, err, buf)
	}
	var i InfoElement
	if err := xml.Unmarshal(buf, &i); err != nil {
		return nil, fmt.Errorf("cannot parse XML: %s: %s", buf, err)
	}
	return &i.Entry, nil
}

// Export will execute an `svn export` subcommand.
// combined output of stdout and stderr will be written to w
// absolute filenames will be written to notifier channel for each exported file
func (a *Repository) Export(relpath, revision, into string, w io.Writer, notifier chan string) error {
	log.Printf("exporting %s at revision %s\n", a.FullPath(relpath), revision)
	fp := a.FullPath(relpath)
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "svn", "-r", revision, "export", fp, into)

	// stdout is written to both w and export notifier
	pr, pw := io.Pipe()
	if w != nil {
		mw := io.MultiWriter(w, pw)
		cmd.Stdout = mw

		// stderr is written to w
		cmd.Stderr = w
	}

	// log.Printf("executing %+v\n", cmd)
	if err := cmd.Start(); err != nil {
		return err
	}

	if notifier != nil {
		go exportNotifier(pr, notifier)
	}

	if ctx.Err() == context.DeadlineExceeded {
		return fmt.Errorf("Exporting %s timed out: %s", fp, ctx.Err())
	}

	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("error exporting %q: %s", fp, err)
	}
	pw.Close()
	return nil
}

// Notify  will report incoming exported filenames to notifier channel
// channel will be closed once EOF is read
func exportNotifier(r io.Reader, c chan string) {
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		line := sc.Text()
		parts := strings.Fields(line)
		if len(parts) == 2 {
			col1 := strings.TrimSpace(parts[0])
			if col1 != "A" {
				log.Printf("ignoring line because of unknown prefix %q\n", col1)
				continue
			}
			filename := strings.TrimSpace(parts[1])
			c <- filename
		} else {
			log.Printf("ignoring line %q\n", line)
		}
	}
	close(c)
}

// Since returns all entries created after t
func Since(entries []Entry, t time.Time) []Entry {
	var es []Entry
	for _, e := range entries {
		if e.Commit.Date.After(t) {
			es = append(es, e)
		}
	}
	return es
}
