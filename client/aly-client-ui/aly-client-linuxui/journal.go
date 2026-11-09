package main

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"time"
)

const (
	maxSelfUpdateAttempts = 3
	selfUpdateRetryWindow = 30 * time.Minute
	journalSuffix         = ".selfupdate.json"
	failureWindowForReset = 30 * 60
)

// updateJournal 与 winui 的 journal 同 schema/语义。
type updateJournal struct {
	Count     int    `json:"count"`
	LastTS    int64  `json:"last_ts"`
	LastError string `json:"last_error"`
	Pending   bool   `json:"pending,omitempty"`
}

func journalPath(target string) string { return target + journalSuffix }

func loadJournal(path string) updateJournal {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return updateJournal{}
	}
	var j updateJournal
	if json.Unmarshal(data, &j) != nil {
		return updateJournal{}
	}
	return j
}

func saveJournal(path string, j updateJournal) {
	data, err := json.Marshal(j)
	if err != nil {
		return
	}
	tmp := path + ".tmp"
	if err := ioutil.WriteFile(tmp, data, 0644); err != nil {
		return
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
	}
}

func (j *updateJournal) markSuccess() {
	j.Count = 0
	j.LastTS = 0
	j.LastError = ""
	j.Pending = false
}

func (j *updateJournal) markAttempt() { j.Pending = true }

func (j *updateJournal) markFailure(msg string) {
	now := time.Now().Unix()
	if j.LastTS > 0 && now-j.LastTS >= failureWindowForReset {
		j.Count = 0
	}
	j.Count++
	j.LastTS = now
	j.LastError = msg
	j.Pending = false
}

func (j updateJournal) shouldSkip(maxAttempts int, window time.Duration) bool {
	if j.Count < maxAttempts || j.LastTS <= 0 {
		return false
	}
	return time.Now().Unix()-j.LastTS < int64(window/time.Second)
}
