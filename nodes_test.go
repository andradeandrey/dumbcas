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
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"path"
	"strings"
	"testing"
	"time"
)

type mockNodesTable struct {
	entries map[string]Node
	cas     CasTable
	t       *testing.T
	log     *log.Logger
}

func (a *DumbcasAppMock) LoadNodesTable(rootDir string, cas CasTable) (NodesTable, error) {
	//return loadNodesTable(rootDir, cas, a.GetLog())
	if a.nodes == nil {
		a.nodes = &mockNodesTable{make(map[string]Node), a.cas, a.T, a.log}
	}
	return a.nodes, nil
}

func (m *mockNodesTable) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	m.log.Printf("mockNodesTable.ServeHTTP(%s)", r.URL.Path)
	suburl := r.URL.Path[1:]
	if suburl != "" {
		// Slow search, it's fine for a mock.
		for k, v := range m.entries {
			if strings.HasPrefix(suburl, k) {
				// Found.
				rest := suburl[len(k):]
				if rest == "" {
					// TODO(maruel): posix-specific.
					localRedirect(w, r, path.Base(r.URL.Path)+"/")
					return
				}
				entry, err := LoadEntry(m.cas, v.Entry)
				if err != nil {
					http.Error(w, fmt.Sprintf("Failed to load the entry file: %s", err), http.StatusNotFound)
					return
				}
				// Defer to the cas file system.
				r.URL.Path = rest
				entryFs := EntryFileSystem{cas: m.cas, entry: entry}
				entryFs.ServeHTTP(w, r)
				return
			}
		}
	}

	if !strings.HasSuffix(r.URL.Path, "/") {
		// Not strictly valid but fine enough for a mock.
		// TODO(maruel): posix-specific.
		localRedirect(w, r, path.Base(r.URL.Path)+"/")
		return
	}

	// List the corresponding "directory", if found.
	items := []string{}
	for k, _ := range m.entries {
		if strings.HasPrefix(k, suburl) {
			v := strings.SplitAfterN(k[len(suburl):], "/", 2)
			items = append(items, v[0])
		}
	}
	if len(items) != 0 {
		dirList(w, items)
		return
	}
	http.Error(w, "Yo dawg", http.StatusNotFound)
}

func (m *mockNodesTable) AddEntry(node *Node, name string) error {
	m.log.Printf("mockNodesTable.AddEntry(%s)", name)

	now := time.Now().UTC()
	monthName := now.Format("2006-01")

	suffix := 0
	for {
		nodeName := now.Format("2006-01-02_15-04-05") + "_" + name
		if suffix != 0 {
			nodeName += fmt.Sprintf("(%d)", suffix)
		}
		nodePath := monthName + "/" + nodeName
		if _, ok := m.entries[nodePath]; !ok {
			m.entries[nodePath] = *node
			break
		}
		// Try ad nauseam.
		suffix += 1
	}
	m.entries[tagsName+"/"+name] = *node
	return nil
}

func (m *mockNodesTable) Enumerate() <-chan NodeEntry {
	m.log.Printf("mockNodesTable.Enumerate() %d", len(m.entries))
	c := make(chan NodeEntry)
	go func() {
		// TODO(maruel): Will blow up if mutated concurrently.
		for k, v := range m.entries {
			c <- NodeEntry{RelPath: k, Node: &v}
		}
		close(c)
	}()
	return c
}

func TestNodesTable(t *testing.T) {
	t.Parallel()
	tempData := makeTempDir(t, "nodes")
	defer removeTempDir(tempData)

	log := getLog(false)
	cas := &mockCasTable{make(map[string][]byte), false, t, log}
	nodes, err := loadNodesTable(tempData, cas, log)
	if err != nil {
		t.Fatal(err)
	}
	testNodesTableImpl(t, cas, nodes)
}

func TestNodesTableMock(t *testing.T) {
	t.Parallel()
	log := getLog(false)
	cas := &mockCasTable{make(map[string][]byte), false, t, log}
	nodes := &mockNodesTable{make(map[string]Node), cas, t, log}
	testNodesTableImpl(t, cas, nodes)
}

func request(t *testing.T, nodes NodesTable, path string, expectedCode int, expectedBody string) string {
	req, err := http.ReadRequest(bufio.NewReader(bytes.NewBufferString("GET " + path + " HTTP/1.1\r\nHost: test\r\n\r\n")))
	if err != nil {
		t.Fatalf("%s: %s", path, err)
	}
	resp := httptest.NewRecorder()
	nodes.ServeHTTP(resp, req)
	bytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	body := string(bytes)
	if resp.Code != expectedCode {
		t.Errorf(body)
		t.Fatalf("%s: %d != %d", path, expectedCode, resp.Code)
	}
	if expectedBody != "" && body != expectedBody {
		t.Fatalf("%s: %#s != %#s", path, expectedBody, body)
	}
	return body
}

// Archive fictious data.
// file1: content1
// dir1/dir2/file2: content2
func archiveData(t *testing.T, cas CasTable, nodes NodesTable) (string, string, string) {
	file1, err := AddBytes(cas, []byte("content1"))
	if err != nil {
		t.Fatal(err)
	}
	file2, err := AddBytes(cas, []byte("content2"))
	if err != nil {
		t.Fatal(err)
	}
	entries := &Entry{
		Files: map[string]*Entry{
			"file1": &Entry{
				Sha1: file1,
				Size: 8,
			},
			"dir1": &Entry{
				Files: map[string]*Entry{
					"dir2": &Entry{
						Files: map[string]*Entry{
							"file2": &Entry{
								Sha1: file2,
								Size: 8,
							},
						},
					},
				},
			},
		},
	}
	data, err := json.Marshal(entries)
	if err != nil {
		t.Fatal(err)
	}
	entrySha1, err := AddBytes(cas, data)
	if err != nil {
		t.Fatal(err)
	}

	if err := nodes.AddEntry(&Node{entrySha1, "useful comment"}, "fictious"); err != nil {
		t.Fatal(err)
	}
	return file1, file2, entrySha1
}

func testNodesTableImpl(t *testing.T, cas CasTable, nodes NodesTable) {
	for _ = range nodes.Enumerate() {
		t.Fatal("Found unexpected value")
	}

	archiveData(t, cas, nodes)
	count := 0
	name := ""
	for v := range nodes.Enumerate() {
		count += 1
		name = v.RelPath
	}
	if count != 2 {
		t.Fatalf("Found %d items", count)
	}

	body := request(t, nodes, "/", 200, "")
	if strings.Count(body, "<a ") != 2 {
		t.Fatal("Unexpected output:\n%s", body)
	}
	// TODO(maruel): The mock misbehaves for: request(t, nodes, "/foo", 404, "")
	request(t, nodes, "/foo/", 404, "")
	request(t, nodes, "/"+name, 301, "")
	request(t, nodes, "/"+name+"/", 200, "")
	request(t, nodes, "/"+name+"/file1", 200, "content1")
	request(t, nodes, "/"+name+"/dir1/dir2/file2", 200, "content2")
	request(t, nodes, "/"+name+"/dir1/dir2/file3", 404, "")
	request(t, nodes, "/"+name+"/dir1/dir2", 301, "")
}
