/* Copyright 2012 Marc-Antoine Ruel. Licensed under the Apache License, Version
2.0 (the "License"); you may not use this file except in compliance with the
License.  You may obtain a copy of the License at
http://www.apache.org/licenses/LICENSE-2.0. Unless required by applicable law or
agreed to in writing, software distributed under the License is distributed on
an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express
or implied. See the License for the specific language governing permissions and
limitations under the License. */

package main

import (
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"testing"
	"time"
)

type WebDumbcasAppMock struct {
	DumbcasAppMock
	socket  net.Listener
	closed  chan bool
	baseUrl string
}

func makeWebDumbcasAppMock(t *testing.T, verbose bool) *WebDumbcasAppMock {
	return &WebDumbcasAppMock{
		DumbcasAppMock: *makeDumbcasMock(t, verbose),
		closed:         make(chan bool),
	}
}

func (f *WebDumbcasAppMock) goWeb() {
	if f.socket != nil {
		f.Fatal("Socket is empty")
	}
	c := make(chan net.Listener)
	go func() {
		err := webMain(f, 0, c)
		f.log.Printf("Closed: %s", err)
		f.closed <- true
	}()
	f.log.Print("Starting")
	f.socket = <-c
	f.baseUrl = fmt.Sprintf("http://%s", f.socket.Addr().String())
	f.log.Printf("Started at %s", f.baseUrl)
}

func (f *WebDumbcasAppMock) closeWeb() {
	f.socket.Close()
	f.socket = nil
	f.baseUrl = ""
	<-f.closed
	f.checkBuffer(false, false)
}

func (f *WebDumbcasAppMock) get(url string, expectedUrl string) *http.Response {
	r, err := http.Get(f.baseUrl + url)
	if err != nil {
		f.Fatal(err)
	}
	if expectedUrl != "" && r.Request.URL.Path != expectedUrl {
		f.Fatalf("%s != %s", expectedUrl, r.Request.URL.Path)
	}
	return r
}

func (f *WebDumbcasAppMock) get404(url string) {
	r, err := http.Get(f.baseUrl + url)
	if err != nil {
		f.Fatal(err)
	}
	if r.StatusCode != 404 {
		f.Fatal(r.StatusCode, r.Body)
	}
}

func readBody(t *testing.T, r *http.Response) string {
	bytes, err := ioutil.ReadAll(r.Body)
	if err != nil {
		t.Fatal(err)
	}
	r.Body.Close()
	return string(bytes)
}

func expectedBody(t *testing.T, r *http.Response, expected string) {
	actual := readBody(t, r)
	if actual != expected {
		t.Fatalf("%v != %v", expected, actual)
	}
}

func TestSmoke(t *testing.T) {
	t.Parallel()
	f := makeWebDumbcasAppMock(t, false)
	cmd := FindCommand(f, "web")
	if cmd == nil {
		t.Fatal("Failed to find 'web'")
	}

	// Sets -root to an invalid non-empty string.
	cmd.GetFlags().Set("root", "\\")

	// Create a tree of stuff.
	f.DumbcasAppMock.MakeCasTable("")
	f.DumbcasAppMock.LoadNodesTable("", f.cas)
	// sha1
	_, _, _ = archiveData(t, f.cas, f.nodes)

	f.log.Print("T: Serve over web and verify files are accessible.")
	f.goWeb()
	f.log.Print("T: Make sure it gets a redirect.")
	r := f.get("/content/retrieve/nodes", "/content/retrieve/nodes/")
	month := time.Now().UTC().Format("2006-01")
	expected := fmt.Sprintf("<html><body><pre><a href=\"%s/\">%s/</a>\n<a href=\"tags/\">tags/</a>\n</pre></body></html>", month, month)
	expectedBody(t, r, expected)
	/*
		f.log.Print("T: Get the directory.")
		r = f.get("/content/retrieve/nodes/"+month, "/content/retrieve/nodes/"+month+"/")
		actual := readBody(t, r)
		re := regexp.MustCompile("\\\"(.*)\\\"")
		nodeItems := re.FindStringSubmatch(actual)
		if len(nodeItems) != 2 {
			t.Fatal(actual)
		}

		f.log.Print("T: Get the node.")
		nodeName := nodeItems[1]
		r = f.get("/content/retrieve/nodes/"+month+"/"+nodeName, "/content/retrieve/nodes/"+month+"/"+nodeName+"/")
		expected = "<html><body><pre><a href=\"tmp/\">tmp/</a>\n</pre></body></html>"
		expectedBody(t, r, expected)

		r = f.get("/content/retrieve/default/"+sha1, "/content/retrieve/default/"+sha1)
		expectedBody(t, r, "content1")
		r = f.get("/content/retrieve/nodes/"+month+"/"+nodeName+"/file1", "")
		expectedBody(t, r, "content1")
		r = f.get("/content/retrieve/nodes/"+month+"/"+nodeName+"/dir1/dir2/file2", "")
		expectedBody(t, r, "content2")

		f.closeWeb()
		f.log.Print("T: Remove dir1/dir2/dir3/foo, archive again and gc.")
		// ...

		f.log.Print("T: Lookup dir1/dir2/dir3/foo is still present in the backup")
		f.goWeb()
		r = f.get("/content/retrieve/nodes/"+month+"/"+nodeName+f.tempData+"/dir1/dir2/dir3/foo", "")
		expectedBody(t, r, tree["dir1/dir2/dir3/foo"])
		r = f.get("/content/retrieve/nodes/"+month+"/"+nodeName+f.tempData+"/dir1/bar", "")
		expectedBody(t, r, tree["dir1/bar"])
		f.closeWeb()

		f.log.Print("T: Remove the node, gc and lookup with web the file is not present anymore.")
		// ...

		f.goWeb()
		f.get404("/content/retrieve/nodes/" + month + "/" + nodeName + f.tempData + "/dir1/dir2/dir3/foo")
		r = f.get("/content/retrieve/nodes/"+month+"/"+nodeName+f.tempData+"/dir1/bar", "")
		expectedBody(t, r, tree["dir1/bar"])
		f.closeWeb()

		f.log.Print("T: Corrupt and fsck.")
		// ...

		// Lookup with web the file is not present anymore.
		f.goWeb()
		f.get404("/content/retrieve/nodes/" + month + "/" + nodeName + f.tempData + "/dir1/bar")
		f.closeWeb()
	*/
}
