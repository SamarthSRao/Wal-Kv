package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/samarthsrao/wal-kv/pkg/kv"
	"github.com/samarthsrao/wal-kv/pkg/wal"
)

const (
	defaultAddr = ":8080"
	walPath     = "wal.log"
)

// SimpleResponse is used for JSON API responses.
type SimpleResponse struct {
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
	Value string `json:"value,omitempty"`
}

var indexTmpl = template.Must(template.New("index").Parse(`
<!doctype html>
<html>
<head>
  <meta charset="utf-8"/>
  <title>Wal-Kv Demo</title>
  <style>
    body { font-family: system-ui, -apple-system, "Segoe UI", Roboto, "Helvetica Neue", Arial; margin: 2rem; }
    input, button { padding: 0.5rem; margin: 0.25rem; }
    .row { margin-bottom: 1rem; }
    label { display:inline-block; width: 60px; }
    #log { white-space: pre-wrap; background: #f7f7f7; padding: 1rem; border-radius: 6px; max-height: 240px; overflow:auto; }
  </style>
</head>
<body>
  <h1>Wal-Kv Demo</h1>

  <div class="row">
    <label>Key</label><input id="key" type="text" value="user:1" />
    <label>Value</label><input id="value" type="text" value="John Doe" />
  </div>

  <div class="row">
    <button onclick="put()">PUT</button>
    <button onclick="get()">GET</button>
    <button onclick="del()">DELETE</button>
    <button onclick="clearLog()">Clear Log</button>
  </div>

  <h3>Result</h3>
  <div id="result">—</div>

  <h3>Server Log</h3>
  <div id="log">Ready.</div>

<script>
function appendLog(msg) {
  const el = document.getElementById('log');
  const now = new Date().toLocaleTimeString();
  el.textContent = now + '  ' + msg + '\n' + el.textContent;
}

function put() {
  const key = document.getElementById('key').value;
  const value = document.getElementById('value').value;
  fetch('/api/put?key=' + encodeURIComponent(key) + '&value=' + encodeURIComponent(value), {method: 'PUT'})
    .then(r => r.json()).then(show).catch(e => appendLog('PUT error: ' + e));
}

function get() {
  const key = document.getElementById('key').value;
  fetch('/api/get?key=' + encodeURIComponent(key))
    .then(r => r.json()).then(show).catch(e => appendLog('GET error: ' + e));
}

function del() {
  const key = document.getElementById('key').value;
  fetch('/api/delete?key=' + encodeURIComponent(key), {method: 'DELETE'})
    .then(r => r.json()).then(show).catch(e => appendLog('DELETE error: ' + e));
}

function show(obj) {
  const r = document.getElementById('result');
  if (obj.ok) {
    r.textContent = 'OK' + (obj.value ? ': ' + obj.value : '');
    appendLog('OK: ' + (obj.value || ''));
  } else {
    r.textContent = 'ERROR: ' + obj.error;
    appendLog('ERROR: ' + obj.error);
  }
}

function clearLog() {
  document.getElementById('log').textContent = '';
}
</script>
</body>
</html>
`))

func main() {
	// Create store and recover from WAL before accepting requests.
	store := kv.NewStore()

	// Ensure WAL exists and attempt recovery.
	recovery, err := wal.NewRecovery(walPath, store)
	if err != nil {
		log.Fatalf("failed to create recovery: %v", err)
	}
	log.Println("Starting WAL recovery...")
	if err := recovery.Replay(); err != nil {
		log.Fatalf("recovery failed: %v", err)
	}
	log.Println("Recovery finished.")

	// Open writer for subsequent operations.
	writer, err := wal.NewWalWriter(walPath)
	if err != nil {
		log.Fatalf("failed to open wal writer: %v", err)
	}
	// Ensure writer closed on exit.
	defer func() {
		if err := writer.Close(); err != nil {
			log.Printf("warning: closing wal writer: %v", err)
		}
	}()

	// HTTP handlers
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Serve frontend.
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		if err := indexTmpl.Execute(w, nil); err != nil {
			http.Error(w, "template error", http.StatusInternalServerError)
		}
	})

	// GET value
	http.HandleFunc("/api/get", func(w http.ResponseWriter, r *http.Request) {
		key := r.URL.Query().Get("key")
		if key == "" {
			writeJSON(w, http.StatusBadRequest, SimpleResponse{OK: false, Error: "missing key"})
			return
		}
		val, ok := store.Get(key)
		if !ok {
			writeJSON(w, http.StatusOK, SimpleResponse{OK: false, Error: "key not found"})
			return
		}
		writeJSON(w, http.StatusOK, SimpleResponse{OK: true, Value: val})
	})

	// PUT value (append to WAL before applying to in-memory store)
	http.HandleFunc("/api/put", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			writeJSON(w, http.StatusMethodNotAllowed, SimpleResponse{OK: false, Error: "use PUT"})
			return
		}
		key := r.URL.Query().Get("key")
		value := r.URL.Query().Get("value")
		if key == "" {
			writeJSON(w, http.StatusBadRequest, SimpleResponse{OK: false, Error: "missing key"})
			return
		}
		// Build entry and append to WAL
		entry := &wal.Entry{
			Op:        wal.OpSet,
			Key:       []byte(key),
			Value:     []byte(value),
			Timestamp: time.Now().Unix(),
		}
		if _, err := writer.Append(entry); err != nil {
			writeJSON(w, http.StatusInternalServerError, SimpleResponse{OK: false, Error: fmt.Sprintf("wal append: %v", err)})
			return
		}
		// WAL append durable; apply to in-memory store
		store.Set(key, value)
		writeJSON(w, http.StatusOK, SimpleResponse{OK: true})
	})

	// DELETE value
	http.HandleFunc("/api/delete", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			writeJSON(w, http.StatusMethodNotAllowed, SimpleResponse{OK: false, Error: "use DELETE"})
			return
		}
		key := r.URL.Query().Get("key")
		if key == "" {
			writeJSON(w, http.StatusBadRequest, SimpleResponse{OK: false, Error: "missing key"})
			return
		}
		entry := &wal.Entry{
			Op:        wal.OpDel,
			Key:       []byte(key),
			Value:     nil,
			Timestamp: time.Now().Unix(),
		}
		if _, err := writer.Append(entry); err != nil {
			writeJSON(w, http.StatusInternalServerError, SimpleResponse{OK: false, Error: fmt.Sprintf("wal append: %v", err)})
			return
		}
		store.Delete(key)
		writeJSON(w, http.StatusOK, SimpleResponse{OK: true})
	})

	// COMPACT logs
	http.HandleFunc("/api/compact", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, SimpleResponse{OK: false, Error: "use POST"})
			return
		}
		// Compact will lock the store and truncate the WAL.
		if err := recovery.Compact(); err != nil {
			writeJSON(w, http.StatusInternalServerError, SimpleResponse{OK: false, Error: fmt.Sprintf("compact: %v", err)})
			return
		}
		writeJSON(w, http.StatusOK, SimpleResponse{OK: true})
	})

	addr := defaultAddr
	if a := os.Getenv("ADDR"); a != "" {
		addr = a
	}

	log.Printf("Starting server on %s", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatalf("server exited: %v", err)
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
